package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/BenyaminSam/pook/internal/ceph"
)

func clearSecretBytes(value []byte) {
	clear(value)
}

func (r *RBDMirrorReconciler) executePoolMirroringPlan(ctx context.Context, validation dependencyValidation, plan poolMirroringPlan) error {
	if len(plan.conflicts) > 0 {
		return errors.New("Cannot execute a pool mirroring plan with conflicts")
	}

	for _, action := range plan.actions {
		if err := ctx.Err(); err != nil {
			return err
		}

		var input ceph.ConnectionInput

		switch action.cluster {
		case mirroringClusterLocal:
			input = validation.localConnection
		case mirroringClusterPeer:
			input = validation.peerConnection
		default:
			return fmt.Errorf("Unknown pool mirroring cluster role %q", action.cluster)
		}

		if err := executePoolMirroringAction(ctx, r.CephExecutor, input, action); err != nil {
			return err
		}
	}
	return nil
}

func executePoolMirroringAction(ctx context.Context, executor ceph.CommandExecutor, input ceph.ConnectionInput, action poolMirroringAction) (err error) {
	connection, err := ceph.PrepareConnection(input)
	if err != nil {
		return fmt.Errorf("Prepare %s connection for pool %q: %w", action.cluster, action.poolName, err)
	}

	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("Clean up %s connection for pool %q: %w", action.cluster, action.poolName, closeErr))
		}
	}()

	if enableErr := ceph.EnablePoolMirroring(ctx, executor, connection, action.poolName, action.desiredMode); enableErr != nil {
		return fmt.Errorf("Enable mirroring for %s pool %q: %w", action.cluster, action.poolName, enableErr)
	}

	return nil
}

func (r *RBDMirrorReconciler) executePoolPeerPlan(ctx context.Context, validation dependencyValidation, plan poolPeerPlan) error {
	if len(plan.conflicts) > 0 {
		return errors.New("Cannot execute a pool peer plan with conflicts")
	}

	for _, action := range plan.actions {
		if err := ctx.Err(); err != nil {
			return err
		}

		switch action.kind {
		case poolPeerActionBootstrap:
			if err := executePoolPeerBootstrap(ctx, r.CephExecutor, validation.localConnection, validation.peerConnection, action); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unknown pool peer action kind %q", action.kind)
		}
	}
	return nil
}

func executePoolPeerBootstrap(ctx context.Context, executor ceph.CommandExecutor, localInput ceph.ConnectionInput, peerInput ceph.ConnectionInput, action poolPeerAction) (err error) {
	directions, err := expectedPoolPeerDirections(action.direction)

	if err != nil {
		return err
	}

	source, err := ceph.PrepareConnection(localInput)
	if err != nil {
		return fmt.Errorf("Prepare local connection for pool %q: %w", action.poolName, err)
	}

	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("Clean up local connection for pool %q: %w", action.poolName, closeErr))
		}
	}()

	token, err := ceph.CreatePoolMirroringBootstrapToken(ctx, executor, source, action.poolName)
	if err != nil {
		return fmt.Errorf("Create bootstrap token for pool %q: %w", action.poolName, err)
	}

	defer clearSecretBytes(token)

	destination, err := ceph.PrepareConnection(peerInput)
	if err != nil {
		return fmt.Errorf("Prepare peer connection for pool %q: %w", action.poolName, err)
	}

	defer func() {
		if closeErr := destination.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("Clean up peer connection for pool %q: %w", action.poolName, closeErr))
		}
	}()

	tokenPath, err := destination.WriteBootstrapToken(token)
	clearSecretBytes(token)

	if err != nil {
		return fmt.Errorf("Stage bootstrap token for pool %q: %w", action.poolName, err)
	}

	if err := ceph.ImportPoolMirrorBootstrapToken(ctx, executor, destination, action.poolName, directions.peer, tokenPath); err != nil {
		return fmt.Errorf("Import bootstrap token for pool %q: %w", action.poolName, err)
	}
	return nil
}
