package controller

import (
	"context"
	"fmt"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func applyDependencyStatus(mirror *storagev1alpha1.RBDMirror, validation dependencyValidation) {
	mirror.Status.Phase = storagev1alpha1.RBDMirrorPhasePending
	mirror.Status.ObservedGeneration = mirror.Generation

	switch {
	case validation.err != nil:
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonDependencyReadFailed,
				Message:            "Referenced Secret dependencies could not be read",
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonDependencyReadFailed,
				Message:            "Dependency validation will be retried",
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonDependencyReadFailed,
				Message:            validation.message,
			},
		)
	case !validation.valid:
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             validation.reason,
				Message:            validation.message,
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonDependencyUnavailable,
				Message:            "Ceph mirroring configuration is waiting for valid dependencies",
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: mirror.Generation,
				Reason:             validation.reason,
				Message:            validation.message,
			},
		)
	default:
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonMirroringNotConfigured,
				Message:            "Referenced Secrets are valid; Ceph mirroring has not been configured",
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonReconciliationNotStarted,
				Message:            "Ceph mirroring configuration has not started",
			},
		)
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonDependenciesValidated,
				Message:            validation.message,
			},
		)
	}
}

func (r *RBDMirrorReconciler) patchStatus(ctx context.Context, before *storagev1alpha1.RBDMirror, after *storagev1alpha1.RBDMirror) error {

	if equality.Semantic.DeepEqual(before.Status, after.Status) {
		return nil
	}

	patch := client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})

	if err := r.Status().Patch(ctx, after, patch); err != nil {
		return fmt.Errorf("patch RBDMirror %s status: %w", client.ObjectKeyFromObject(after), err)
	}

	return nil
}

func applyCephObservationStatus(mirror *storagev1alpha1.RBDMirror, observation cephObservationValidation) {
	mirror.Status.Phase = storagev1alpha1.RBDMirrorPhaseConfiguring
	mirror.Status.ObservedGeneration = mirror.Generation

	switch {
	case observation.err != nil:
		meta.SetStatusCondition(&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            observation.message,
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            "Ceph connectivity observation will be retried",
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            observation.message,
			},
		)

	case !observation.valid:
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            observation.message,
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            "Ceph connectivity observation is waiting for corrected configuration",
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: mirror.Generation,
				Reason:             observation.reason,
				Message:            observation.message,
			},
		)

	default:
		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonMirroringNotConfigured,
				Message:            "Ceph connectivity was verified; Ceph mirroring has not been configured",
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionProgressing),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonCephObservationSucceeded,
				Message:            observation.message,
			},
		)

		meta.SetStatusCondition(
			&mirror.Status.Conditions,
			metav1.Condition{
				Type:               string(storagev1alpha1.RBDMirrorConditionDegraded),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: mirror.Generation,
				Reason:             storagev1alpha1.RBDMirrorReasonCephObservationSucceeded,
				Message:            "Both Ceph clusters are reachable through the configured credentials",
			},
		)
	}
}
