package controller

import (
	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func applyPoolPlanningStatus(mirror *storagev1alpha1.RBDMirror, blocked bool, reason string, message string) {
	mirror.Status.ObservedGeneration = mirror.Generation
	mirror.Status.Phase = storagev1alpha1.RBDMirrorPhaseConfiguring

	progressing := metav1.ConditionTrue
	degraded := metav1.ConditionFalse

	if blocked {
		mirror.Status.Phase = storagev1alpha1.RBDMirrorPhaseFailed
		progressing = metav1.ConditionFalse
		degraded = metav1.ConditionTrue
	}

	conditions := []metav1.Condition{
		{
			Type:   string(storagev1alpha1.RBDMirrorConditionReady),
			Status: metav1.ConditionFalse,
		},
		{
			Type:   string(storagev1alpha1.RBDMirrorConditionProgressing),
			Status: progressing,
		},
		{
			Type:   string(storagev1alpha1.RBDMirrorConditionDegraded),
			Status: degraded,
		},
	}

	for _, condition := range conditions {
		condition.ObservedGeneration = mirror.Generation
		condition.Reason = reason
		condition.Message = message

		apimeta.SetStatusCondition(&mirror.Status.Conditions, condition)
	}
}
