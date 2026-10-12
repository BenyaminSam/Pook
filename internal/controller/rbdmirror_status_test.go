package controller

import (
	"reflect"
	"testing"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyPoolPlanningStatus(t *testing.T) {
	tests := []struct {
		name        string
		blocked     bool
		reason      string
		phase       storagev1alpha1.RBDMirrorPhase
		progressing metav1.ConditionStatus
		degraded    metav1.ConditionStatus
	}{
		{
			name:        "verification pending",
			reason:      "PoolPeerVerificationPending",
			phase:       storagev1alpha1.RBDMirrorPhaseConfiguring,
			progressing: metav1.ConditionTrue,
			degraded:    metav1.ConditionFalse,
		},
		{
			name:        "configuration conflict",
			blocked:     true,
			reason:      "PoolPeerConflict",
			phase:       storagev1alpha1.RBDMirrorPhaseFailed,
			progressing: metav1.ConditionFalse,
			degraded:    metav1.ConditionTrue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mirror := &storagev1alpha1.RBDMirror{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
			}

			const message = "Test reconciliation result"

			applyPoolPlanningStatus(
				mirror,
				tt.blocked,
				tt.reason,
				message,
			)

			if mirror.Status.Phase != tt.phase {
				t.Fatalf(
					"phase = %q, want %q",
					mirror.Status.Phase,
					tt.phase,
				)
			}

			if mirror.Status.ObservedGeneration != mirror.Generation {
				t.Fatalf(
					"observed generation = %d, want %d",
					mirror.Status.ObservedGeneration,
					mirror.Generation,
				)
			}

			expected := map[string]metav1.ConditionStatus{
				string(storagev1alpha1.RBDMirrorConditionReady):       metav1.ConditionFalse,
				string(storagev1alpha1.RBDMirrorConditionProgressing): tt.progressing,
				string(storagev1alpha1.RBDMirrorConditionDegraded):    tt.degraded,
			}

			if len(mirror.Status.Conditions) != len(expected) {
				t.Fatalf(
					"condition count = %d, want %d",
					len(mirror.Status.Conditions),
					len(expected),
				)
			}

			for conditionType, wantStatus := range expected {
				condition := apimeta.FindStatusCondition(
					mirror.Status.Conditions,
					conditionType,
				)

				if condition == nil {
					t.Fatalf("missing condition %q", conditionType)
				}

				if condition.Status != wantStatus {
					t.Errorf(
						"%s status = %q, want %q",
						conditionType,
						condition.Status,
						wantStatus,
					)
				}

				if condition.Reason != tt.reason {
					t.Errorf(
						"%s reason = %q, want %q",
						conditionType,
						condition.Reason,
						tt.reason,
					)
				}

				if condition.Message != message {
					t.Errorf(
						"%s message = %q, want %q",
						conditionType,
						condition.Message,
						message,
					)
				}

				if condition.ObservedGeneration != mirror.Generation {
					t.Errorf(
						"%s observed generation = %d, want %d",
						conditionType,
						condition.ObservedGeneration,
						mirror.Generation,
					)
				}
			}
		})
	}
}

func TestApplyPoolPlanningStatusIsStable(t *testing.T) {
	mirror := &storagev1alpha1.RBDMirror{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
		},
	}

	applyPoolPlanningStatus(
		mirror,
		false,
		"PoolPeerVerificationPending",
		"Waiting for observation",
	)

	before := mirror.DeepCopy()

	applyPoolPlanningStatus(
		mirror,
		false,
		"PoolPeerVerificationPending",
		"Waiting for observation",
	)

	if !reflect.DeepEqual(before.Status, mirror.Status) {
		t.Fatal("identical planning results changed the resource status")
	}
}
