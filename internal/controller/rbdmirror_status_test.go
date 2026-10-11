package controller

import (
	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RBDMirror initial status", func() {
	It("reports a pending assessment without rewriting unchanged status", func() {
		mirror := &storagev1alpha1.RBDMirror{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "status-test-",
				Namespace:    "default",
			},
			Spec: storagev1alpha1.RBDMirrorSpec{
				LocalCluster: storagev1alpha1.CephClusterConnectionSpec{
					ClusterID: "local",
					ConfigSecretRef: storagev1alpha1.CephSecretReference{
						Name: "local-config",
					},
					CredentialsSecretRef: storagev1alpha1.CephSecretReference{
						Name: "local-credentials",
					},
				},
				PeerCluster: storagev1alpha1.CephClusterConnectionSpec{
					ClusterID: "peer",
					ConfigSecretRef: storagev1alpha1.CephSecretReference{
						Name: "peer-config",
					},
					CredentialsSecretRef: storagev1alpha1.CephSecretReference{
						Name: "peer-credentials",
					},
				},
				Pools: []storagev1alpha1.RBDMirrorPoolSpec{
					{
						Name:      "replicated-pool",
						Mode:      "journal",
						Direction: "rx-tx",
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, mirror)).To(Succeed())

		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(
				k8sClient.Delete(ctx, mirror),
			)).To(Succeed())
		})

		reconciler := &RBDMirrorReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		key := client.ObjectKeyFromObject(mirror)
		request := ctrl.Request{NamespacedName: key}

		By("recording the initial assessment")
		result, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		first := &storagev1alpha1.RBDMirror{}
		Expect(k8sClient.Get(ctx, key, first)).To(Succeed())

		Expect(first.Status.Phase).To(Equal(
			storagev1alpha1.RBDMirrorPhasePending,
		))
		Expect(first.Status.ObservedGeneration).To(Equal(first.Generation))
		Expect(first.Status.Conditions).To(HaveLen(3))

		expectedConditions := []struct {
			conditionType storagev1alpha1.RBDMirrorConditionType
			status        metav1.ConditionStatus
			reason        string
		}{
			{
				conditionType: storagev1alpha1.RBDMirrorConditionReady,
				status:        metav1.ConditionFalse,
				reason:        storagev1alpha1.RBDMirrorReasonDependenciesNotChecked,
			},
			{
				conditionType: storagev1alpha1.RBDMirrorConditionProgressing,
				status:        metav1.ConditionFalse,
				reason:        storagev1alpha1.RBDMirrorReasonReconciliationNotStarted,
			},
			{
				conditionType: storagev1alpha1.RBDMirrorConditionDegraded,
				status:        metav1.ConditionUnknown,
				reason:        storagev1alpha1.RBDMirrorReasonDependenciesNotChecked,
			},
		}

		for _, expected := range expectedConditions {
			condition := meta.FindStatusCondition(
				first.Status.Conditions,
				string(expected.conditionType),
			)

			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(expected.status))
			Expect(condition.Reason).To(Equal(expected.reason))
			Expect(condition.ObservedGeneration).To(Equal(first.Generation))
			Expect(condition.LastTransitionTime.IsZero()).To(BeFalse())
		}

		By("reconciling again without changing the persisted object")
		result, err = reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		second := &storagev1alpha1.RBDMirror{}
		Expect(k8sClient.Get(ctx, key, second)).To(Succeed())

		Expect(second.Status).To(Equal(first.Status))
		Expect(second.ResourceVersion).To(Equal(first.ResourceVersion))
	})
})
