package ceph

import (
	"context"
	"errors"
	"strings"
)

type FSIDMismatchError struct {
	Expected string
	Observed string
}

func (e *FSIDMismatchError) Error() string {
	return "Observed Ceph FSID does not match the expected cluster ID"
}

type FSIDObservationError struct {
	Cause error
}

func (e *FSIDObservationError) Error() string {
	return "Ceph FSID observation failed"
}

func (e *FSIDObservationError) Unwrap() error {
	return e.Cause
}

func VerifyFSID(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, expectedClusterID string) error {
	if executor == nil {
		return &FSIDObservationError{
			Cause: errors.New("Ceph CLI executor is nil"),
		}
	}

	if connection == nil {
		return &FSIDObservationError{
			Cause: errors.New("Ceph connection is nil"),
		}
	}

	expected := strings.TrimSpace(expectedClusterID)
	if expected == "" {
		return &FSIDObservationError{
			Cause: errors.New("expected Ceph cluster IS is empty"),
		}
	}

	command := connection.Command("ceph", "fsid")

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return &FSIDObservationError{
			Cause: err,
		}
	}

	observed := strings.TrimSpace(string(result.Stdout))
	if observed == "" {
		return &FSIDObservationError{
			Cause: errors.New("Ceph fsid returned empty output"),
		}
	}

	if observed != expected {
		return &FSIDMismatchError{
			Expected: expected,
			Observed: observed,
		}
	}

	return nil
}
