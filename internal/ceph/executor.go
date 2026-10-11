package ceph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	maxOutputBytes   = 1024 * 1024
	processWaitDelay = 2 * time.Second
)

var ErrOutputLimitExceeded = errors.New("Ceph CLI output limit exceeded")

type OSExecutor struct {
	timeout time.Duration
}

var _ CommandExecutor = (*OSExecutor)(nil)

func NewOSExecutor(timeout time.Duration) (*OSExecutor, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("Ceph CLI timeout must be greater than zero")
	}
	return &OSExecutor{timeout: timeout}, nil
}

func (e *OSExecutor) Execute(ctx context.Context, command Command) (CommandResult, error) {
	result := CommandResult{ExitCode: -1}

	if e.timeout <= 0 {
		return result, fmt.Errorf("Ceph CLI executor has no valid timeout")
	}

	switch command.Binary {
	case "ceph", "rbd":
	default:
		return result, fmt.Errorf("unsupported Ceph CLI executable")
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	if err := runCtx.Err(); err != nil {
		return result, &ExecutionError{
			ExitCode: -1,
			Cause:    err,
		}
	}

	cmd := exec.CommandContext(runCtx, command.Binary, command.Args...)

	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"LANG=C",
		"LC_ALL=C",
	}

	cmd.WaitDelay = processWaitDelay

	stdout := &boundedBuffer{limit: maxOutputBytes}
	stderr := &boundedBuffer{limit: maxOutputBytes}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()

	result.Stdout = stdout.data
	result.Stderr = stderr.data
	result.Truncated = stdout.truncated || stderr.truncated

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if runErr != nil {
		cause := runErr

		if ctxErr := runCtx.Err(); ctxErr != nil {
			cause = errors.Join(cause, ctxErr)
		}

		if result.Truncated {
			cause = errors.Join(cause, ErrOutputLimitExceeded)
		}

		return result, &ExecutionError{
			ExitCode: result.ExitCode,
			Cause:    cause,
		}
	}

	if result.Truncated {
		return result, &ExecutionError{
			ExitCode: result.ExitCode,
			Cause:    ErrOutputLimitExceeded,
		}
	}

	return result, nil
}

type boundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - len(b.data)

	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}

	b.data = append(b.data, p...)

	return n, nil
}
