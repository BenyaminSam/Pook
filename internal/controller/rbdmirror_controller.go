package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logger "sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	"github.com/BenyaminSam/pook/internal/ceph"
)

const (
	poolMirroringStableRequeueAfter       = 30 * time.Second
	poolMirroringVerificationRequeueAfter = 10 * time.Second
)

type RBDMirrorReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CephExecutor ceph.CommandExecutor
}

// +kubebuilder:rbac:groups=storage.pook.io,resources=rbdmirrors,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.pook.io,resources=rbdmirrors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
func (r *RBDMirrorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logger.FromContext(ctx).WithValues("rbdmirror", req.NamespacedName)

	mirror := &storagev1alpha1.RBDMirror{}

	if err := r.Get(ctx, req.NamespacedName, mirror); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("RBDMirror no longer exists")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get RBDMirror %s: %w", req.NamespacedName, err)
	}

	if !mirror.DeletionTimestamp.IsZero() {
		logger.V(1).Info("Skipping RBDMirror marked for deletion")
		return ctrl.Result{}, nil
	}

	logger.V(1).Info("Fetched RBDMirror", "generation", mirror.Generation)

	desiredPools, err := normalizePoolMirroring(mirror.Spec.Pools)
	if err != nil {
		before := mirror.DeepCopy()
		applyPoolPlanningStatus(mirror, true, "InvalidMirroringSpec", err.Error())
		if err := r.patchStatus(ctx, before, mirror); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	validation := r.validateDependencies(ctx, mirror)

	if validation.err != nil || !validation.valid {
		before := mirror.DeepCopy()

		applyDependencyStatus(mirror, validation)

		if err := r.patchStatus(ctx, before, mirror); err != nil {
			return ctrl.Result{}, err
		}

		if validation.err != nil {
			return ctrl.Result{}, validation.err
		}

		return ctrl.Result{
			RequeueAfter: validation.requeueAfter,
		}, nil
	}

	observations, observation := r.observeCephConnections(ctx, validation, mirror.Spec.Pools)

	if observation.err != nil || !observation.valid {
		before := mirror.DeepCopy()

		applyCephObservationStatus(mirror, observation)

		if err := r.patchStatus(ctx, before, mirror); err != nil {
			return ctrl.Result{}, err
		}

		if observation.err != nil {
			return ctrl.Result{}, observation.err
		}
		return ctrl.Result{RequeueAfter: observation.requeueAfter}, nil
	}

	plan := buildPoolMirroringPlan(desiredPools, observations)

	logger.V(1).Info(
		"Planned pool mirroring configuration",
		"localPoolCount", len(observations.localPools),
		"peerPoolCount", len(observations.peerPools),
		"actionCount", len(plan.actions),
		"conflictCount", len(plan.conflicts),
	)

	before := mirror.DeepCopy()
	requeueAfter := poolMirroringStableRequeueAfter

	switch {
	case len(plan.conflicts) > 0:
		applyPoolPlanningStatus(
			mirror,
			true,
			"PoolMirroringConflict",
			plan.conflictMessage(),
		)

	case len(plan.actions) > 0:
		if executionErr := r.executePoolMirroringPlan(ctx, validation, plan); executionErr != nil {
			applyPoolPlanningStatus(
				mirror,
				true,
				"PoolMirroringExecutionFailed",
				"Pool mirroring execution failed; completed changes are preserved and Ceph will be observed again before retrying",
			)

			if patchErr := r.patchStatus(ctx, before, mirror); patchErr != nil {
				return ctrl.Result{}, errors.Join(executionErr, patchErr)
			}

			return ctrl.Result{}, executionErr
		}

		requeueAfter = poolMirroringVerificationRequeueAfter

		applyPoolPlanningStatus(
			mirror,
			false,
			"PoolMirroringVerificationPending",
			"Pool mirroring enable operations completed; waiting for a fresh Ceph observation",
		)

	default:
		peerPlan := buildPoolPeerPlan(desiredPools, observations)

		logger.V(1).Info(
			"Planned pool peer configuration",
			"actionCount", len(peerPlan.actions),
			"conflictCount", len(peerPlan.conflicts),
		)

		switch {
		case len(peerPlan.conflicts) > 0:
			applyPoolPlanningStatus(
				mirror,
				true,
				"PoolPeerConflict",
				peerPlan.conflictMessage(),
			)

		case len(peerPlan.actions) > 0:
			if executionErr := r.executePoolPeerPlan(ctx, validation, peerPlan); executionErr != nil {
				applyPoolPlanningStatus(
					mirror,
					true,
					"PoolPeerExecutionFailed",
					"Pool peer execution failed; Ceph will be observed again before retrying",
				)

				if patchErr := r.patchStatus(ctx, before, mirror); patchErr != nil {
					return ctrl.Result{}, errors.Join(executionErr, patchErr)
				}

				return ctrl.Result{}, executionErr
			}

			requeueAfter = poolMirroringVerificationRequeueAfter

			applyPoolPlanningStatus(
				mirror,
				false,
				"PoolPeerVerificationPending",
				"Peer bootstrap operations completed; waiting for a fresh Ceph observation",
			)

		default:
			applyPoolPlanningStatus(
				mirror,
				false,
				"PoolConfigurationObserved",
				"Peer identities and directions match on both clusters; peer credentials, daemon availability, and replication health have not been verified",
			)
		}
	}

	if err := r.patchStatus(ctx, before, mirror); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *RBDMirrorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &storagev1alpha1.RBDMirror{}, rbdMirrorSecretRefIndex, indexRBDMirrorSecretRef); err != nil {
		return fmt.Errorf("Index RBDMirror secret reference: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.RBDMirror{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.mapSecretToRBDMirrors)).
		Named("rbdmirror").
		Complete(r)
}
