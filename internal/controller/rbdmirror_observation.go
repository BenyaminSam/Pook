package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	"github.com/BenyaminSam/pook/internal/ceph"
)

const cephObservationRequeueAfter = 30 * time.Second

type cephObservationValidation struct {
	valid        bool
	reason       string
	message      string
	requeueAfter time.Duration
	err          error
}

type cephConnectionObservations struct {
	localPools map[string]ceph.PoolMirroringInfo
	peerPools  map[string]ceph.PoolMirroringInfo
}

func (r *RBDMirrorReconciler) observeCephConnections(ctx context.Context, validation dependencyValidation, pools []storagev1alpha1.RBDMirrorPoolSpec) (cephConnectionObservations, cephObservationValidation) {
	if r.CephExecutor == nil {
		return cephConnectionObservations{}, cephObservationValidation{
			valid:        false,
			reason:       storagev1alpha1.RBDMirrorReasonCephExecutorUnavailable,
			message:      "Ceph CLI Executor is not configured",
			requeueAfter: cephObservationRequeueAfter,
			err:          errors.New("Ceph CLI executor is not configured"),
		}
	}

	localPools, localResult := observeSingleCephConnection(ctx, r.CephExecutor, "localCluster", validation.localConnection, pools)

	if localResult.err != nil || !localResult.valid {
		return cephConnectionObservations{}, localResult
	}

	peerPools, peerResult := observeSingleCephConnection(ctx, r.CephExecutor, "peerCluster", validation.peerConnection, pools)

	if peerResult.err != nil || !peerResult.valid {
		return cephConnectionObservations{}, peerResult
	}

	return cephConnectionObservations{
			localPools: localPools,
			peerPools:  peerPools,
		}, cephObservationValidation{
			valid:   true,
			reason:  storagev1alpha1.RBDMirrorReasonCephObservationSucceeded,
			message: "Read-only Ceph identity, pool existence, and mirroring observations succeeded for both clusters",
		}
}

func observeSingleCephConnection(ctx context.Context, executor ceph.CommandExecutor, role string, input ceph.ConnectionInput, pools []storagev1alpha1.RBDMirrorPoolSpec) (observations map[string]ceph.PoolMirroringInfo, result cephObservationValidation) {

	connection, err := ceph.PrepareConnection(input)
	if err != nil {
		return nil, cephObservationValidation{
			valid:        false,
			reason:       storagev1alpha1.RBDMirrorReasonCephConnectionPreparationFailed,
			message:      fmt.Sprintf("Failed to prepare the %s Ceph connection", role),
			requeueAfter: cephObservationRequeueAfter,
			err:          fmt.Errorf("Prepare %s Ceph connection: %w", role, err),
		}
	}

	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			observations = nil
			result = cephObservationValidation{
				valid:        false,
				reason:       storagev1alpha1.RBDMirrorReasonCephConnectionCleanupFailed,
				message:      fmt.Sprintf("Failed to clean up the %s connection", role),
				requeueAfter: cephObservationRequeueAfter,
				err:          errors.Join(result.err, fmt.Errorf("Close %s Ceph connection: %w", role, closeErr)),
			}
		}
	}()

	if err := ceph.VerifyFSID(ctx, executor, connection, input.ClusterID); err != nil {
		var mismatchError *ceph.FSIDMismatchError
		if errors.As(err, &mismatchError) {
			return nil, cephObservationValidation{
				valid:        false,
				reason:       storagev1alpha1.RBDMirrorReasonCephClusterIDMismatch,
				message:      fmt.Sprintf("%s Ceph cluster identity does not match the configured cluster ID", role),
				requeueAfter: 0,
			}
		}

		return nil, cephObservationValidation{
			valid:        false,
			reason:       storagev1alpha1.RBDMirrorReasonCephObservationFailed,
			message:      fmt.Sprintf("Read-only Ceph FSID observation failed for %s", role),
			requeueAfter: cephObservationRequeueAfter,
			err:          fmt.Errorf("Observe %s Ceph FSID: %w", role, err),
		}
	}

	poolNames := make([]string, 0, len(pools))
	for _, pool := range pools {
		poolNames = append(poolNames, pool.Name)
	}

	if err := ceph.VerifyPoolExist(ctx, executor, connection, poolNames); err != nil {
		var PoolNotFoundError *ceph.PoolNotFoundError
		if errors.As(err, &PoolNotFoundError) {
			return nil, cephObservationValidation{
				valid:        false,
				reason:       storagev1alpha1.RBDMirrorReasonCephPoolNotFound,
				message:      fmt.Sprintf("Configured pool %q was not found on the %s Ceph cluster", PoolNotFoundError.PoolName, role),
				requeueAfter: cephObservationRequeueAfter,
			}
		}

		return nil, cephObservationValidation{
			valid:  false,
			reason: storagev1alpha1.RBDMirrorReasonCephPoolObservationFailed,
			message: fmt.Sprintf(
				"Read-only Ceph pool observation failed for %s",
				role,
			),
			requeueAfter: cephObservationRequeueAfter,
			err:          fmt.Errorf("observe %s Ceph pools: %w", role, err),
		}
	}

	observations, err = observePoolMirroring(ctx, executor, connection, poolNames)
	if err != nil {
		return nil, cephObservationValidation{
			valid:        false,
			reason:       storagev1alpha1.RBDMirrorReasonCephMirroringObservationFailed,
			message:      fmt.Sprintf("Read-only pool mirroring observation failed for %s", role),
			requeueAfter: cephObservationRequeueAfter,
			err:          fmt.Errorf("Observe %s pool mirroring: %w", role, err),
		}
	}

	return observations, cephObservationValidation{
		valid: true,
	}
}

func observePoolMirroring(ctx context.Context, executor ceph.CommandExecutor, connection *ceph.PreparedConnection, poolNames []string) (map[string]ceph.PoolMirroringInfo, error) {

	observations := make(map[string]ceph.PoolMirroringInfo, len(poolNames))

	for _, poolName := range poolNames {
		if _, observed := observations[poolName]; observed {
			continue
		}

		info, err := ceph.ObservePoolMirroring(ctx, executor, connection, poolName)
		if err != nil {
			return nil, fmt.Errorf("Observe mirroring for pool %q: %w", poolName, err)
		}
		observations[poolName] = info
	}

	return observations, nil
}
