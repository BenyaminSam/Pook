package ceph

import (
	"context"
	"fmt"
)

type Command struct {
	Binary string
	Args   []string
}

type CommandResult struct {
	Stdout    []byte
	Stderr    []byte
	ExitCode  int
	Truncated bool
}

type CommandExecutor interface {
	Execute(ctx context.Context, command Command) (CommandResult, error)
}

type ExecutionError struct {
	ExitCode int
	Cause    error
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("Ceph CLI execution failed (exit code %d)", e.ExitCode)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}
