package ceph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type PoolNotFoundError struct {
	PoolName string
}

func (e *PoolNotFoundError) Error() string {
	return fmt.Sprintf("Ceph pool %q was not found", e.PoolName)
}

type PoolObservationError struct {
	Cause error
}

func (e *PoolObservationError) Error() string {
	return "Ceph pool observation failed"
}

func (e *PoolObservationError) Unwrap() error {
	return e.Cause
}

type PoolMirroringInfo struct {
	PoolName   string              `json:"-"`
	Mode       string              `json:"mode"`
	SiteName   string              `json:"site_name,omitempty"`
	MirrorUUID string              `json:"mirror_uuid,omitempty"`
	Peers      []PoolMirroringPeer `json:"peers,omitempty"`
}

type PoolMirroringPeer struct {
	UUID       string `json:"uuid"`
	Direction  string `json:"direction"`
	SiteName   string `json:"site_name"`
	MirrorUUID string `json:"mirror_uuid,omitempty"`
	ClientName string `json:"client_name"`
}

type PoolMirroringObservationError struct {
	PoolName string
	Cause    error
}

func (e *PoolMirroringObservationError) Error() string {
	return fmt.Sprintf("Ceph pool mirroring observation failed for pool %q", e.PoolName)
}

func (e *PoolMirroringObservationError) Unwrap() error {
	return e.Cause
}

func clearBytes(value []byte) {
	clear(value)
}

func VerifyPoolExist(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolNames []string) error {
	if executor == nil {
		return &PoolObservationError{
			Cause: errors.New("Ceph CLI executor is nil"),
		}
	}

	if connection == nil {
		return &PoolObservationError{
			Cause: errors.New("Ceph connection is nil"),
		}
	}

	if len(poolNames) == 0 {
		return &PoolObservationError{
			Cause: errors.New("No Ceph pools were requested"),
		}
	}

	for _, poolName := range poolNames {
		if err := validatePoolName(poolName); err != nil {
			return &PoolObservationError{Cause: err}
		}
	}

	command := connection.Command(
		"ceph",
		"osd",
		"pool",
		"ls",
		"--format=json",
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return &PoolObservationError{
			Cause: err,
		}
	}

	var observedPoolNames []string

	if err := json.Unmarshal(result.Stdout, &observedPoolNames); err != nil {
		return &PoolObservationError{
			Cause: fmt.Errorf("Decode ceph pool list: %w", err),
		}
	}

	observedPools := make(map[string]struct{}, len(observedPoolNames))
	for _, poolName := range observedPoolNames {
		observedPools[poolName] = struct{}{}
	}

	for _, requestedPoolName := range poolNames {
		if _, found := observedPools[requestedPoolName]; !found {
			return &PoolNotFoundError{
				PoolName: requestedPoolName,
			}
		}
	}
	return nil
}

func ObservePoolMirroring(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolName string) (PoolMirroringInfo, error) {
	observationError := func(cause error) error {
		return &PoolMirroringObservationError{
			PoolName: poolName,
			Cause:    cause,
		}
	}

	if executor == nil {
		return PoolMirroringInfo{}, observationError(errors.New("Ceph CLI executor is nil"))
	}

	if connection == nil {
		return PoolMirroringInfo{}, observationError(errors.New("Ceph connection is nil"))
	}

	if err := validatePoolName(poolName); err != nil {
		return PoolMirroringInfo{}, observationError(err)
	}

	command := connection.Command(
		"rbd",
		"mirror",
		"pool",
		"info",
		poolName,
		"--format=json",
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return PoolMirroringInfo{}, observationError(err)
	}

	var info PoolMirroringInfo

	if err := json.Unmarshal(result.Stdout, &info); err != nil {
		return PoolMirroringInfo{}, observationError(
			fmt.Errorf("Decode pool mirroring information: %w", err),
		)
	}

	if strings.TrimSpace(info.Mode) == "" {
		return PoolMirroringInfo{}, observationError(
			errors.New("Pool mirroring response does not contain a mode"),
		)
	}
	info.PoolName = poolName

	return info, nil
}

func CreatePoolMirroringBootstrapToken(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolName string) ([]byte, error) {
	if executor == nil {
		return nil, errors.New("Ceph CLI executor is nil")
	}

	if connection == nil {
		return nil, errors.New("Ceph connection is nil")
	}

	if err := validatePoolName(poolName); err != nil {
		return nil, err
	}

	// switch direction {
	// case "rx-only", "tx-only", "rx-tx":
	// default:
	// 	return nil, fmt.Errorf("Unsupported bootstrap create direction %q", direction)
	// }

	command := connection.Command(
		"rbd",
		"mirror",
		"pool",
		"peer",
		"bootstrap",
		"create",
		poolName,
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		clearBytes(result.Stdout)
		return nil, fmt.Errorf("create pool mirror bootstrap token: %w", err)
	}

	if result.Truncated {
		clearBytes(result.Stdout)
		return nil, errors.New("create pool mirror bootstrap token output was truncated")
	}

	token := append([]byte(nil), bytes.TrimSpace(result.Stdout)...)
	clearBytes(result.Stdout)

	if len(token) == 0 {
		return nil, errors.New("create pool mirror bootstrap token returned empty output")
	}
	return token, nil
}

func ImportPoolMirrorBootstrapToken(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolName string, direction string, tokenPath string) error {
	if executor == nil {
		return errors.New("Ceph CLI executor is nil")
	}

	if connection == nil {
		return errors.New("Ceph connection is nil")
	}

	if err := validatePoolName(poolName); err != nil {
		return err
	}

	switch direction {
	case "rx-only", "rx-tx":
	default:
		return fmt.Errorf("Unsupported bootstrap import direction %q", direction)
	}

	if strings.TrimSpace(tokenPath) == "" {
		return errors.New("bootstrap token path is empty")
	}

	if strings.TrimSpace(tokenPath) != tokenPath {
		return errors.New("Bootstrap token path has surrounding whitespace")
	}

	command := connection.Command(
		"rbd",
		"mirror",
		"pool",
		"peer",
		"bootstrap",
		"import",
		"--direction",
		direction,
		poolName,
		tokenPath,
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return fmt.Errorf("import pool mirror bootstrap token: %w", err)
	}

	if result.Truncated {
		return errors.New("import pool mirror bootstrap command output was truncated")
	}
	return nil
}

func SetPoolMirrorPeerDirection(ctx context.Context, executor CommandExecutor, connection *PreparedConnection, poolName string, peerUUID string, direction string) error {
	if executor == nil {
		return errors.New("Ceph CLI executor is nil")
	}

	if connection == nil {
		return errors.New("Ceph connection is nil")
	}

	if err := validatePoolName(poolName); err != nil {
		return err
	}

	if strings.TrimSpace(peerUUID) == "" {
		return errors.New("pool mirror peer UUID is empty")
	}

	if strings.TrimSpace(peerUUID) != peerUUID {
		return errors.New("pool mirror peer UUID has surrounding whitespace")
	}

	switch direction {
	case "rx-only", "rx-tx", "tx-only":
	default:
		return fmt.Errorf("Unsupported pool mirror peer direction %q", direction)
	}

	command := connection.Command(
		"rbd",
		"mirror",
		"pool",
		"peer",
		"set",
		poolName,
		peerUUID,
		"direction",
		direction,
	)

	result, err := executor.Execute(ctx, command)
	if err != nil {
		return fmt.Errorf("set pool mirror peer direction: %w", err)
	}

	if result.Truncated {
		return errors.New("set pool mirror peer direction output was truncated")
	}
	return nil
}

func validatePoolName(poolName string) error {

	trimmed := strings.TrimSpace(poolName)
	if trimmed == "" {
		return errors.New("Requested Ceph pool name is empty")
	}

	if trimmed != poolName {
		return fmt.Errorf("Requested Ceph pool name %q has surrounding whitespace", poolName)
	}
	return nil
}
