package ceph

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type PoolMirroringEnableError struct {
	PoolName string
	Cause    error
}

func (e *PoolMirroringEnableError) Error() string {
	return fmt.Sprintf("Ceph pool mirroring enable failed for pool %q", e.PoolName)
}

func (e *PoolMirroringEnableError) Unwrap() error {
	return e.Cause
}

func EnablePoolMirroring(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolName string, mode string) error {
	fail := func(cause error) error {
		return &PoolMirroringEnableError{
			PoolName: poolName,
			Cause:    cause,
		}
	}

	if executor == nil {
		return fail(errors.New("Ceph CLI executor is nil"))
	}

	if connection == nil {
		return fail(errors.New("Ceph connection is nil"))
	}

	if poolName == "" || strings.TrimSpace(poolName) != poolName {
		return fail(errors.New("Pool name must be nonempty and have no surrounding whitespace"))
	}

	if mode != "pool" {
		return fail(fmt.Errorf("Unsupported pool mirroring enable mode %q", mode))
	}

	if err := ctx.Err(); err != nil {
		return fail(err)
	}

	if err := VerifyFSID(ctx, executor, connection, connection.ClusterID()); err != nil {
		return fail(err)
	}

	command := connection.Command(
		"rbd",
		"mirror",
		"pool",
		"enable",
		"--pool",
		poolName,
		mode,
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return fail(err)
	}

	if result.ExitCode != 0 {
		return fail(&ExecutionError{
			ExitCode: result.ExitCode,
			Cause:    errors.New("rbd returned a nonzero exit code"),
		})
	}

	if result.Truncated {
		return fail(ErrOutputLimitExceeded)
	}
	return nil
}
