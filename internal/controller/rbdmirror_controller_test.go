/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
)

var _ = Describe("RBDMirror Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		rbdmirror := &storagev1alpha1.RBDMirror{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind RBDMirror")
			err := k8sClient.Get(ctx, typeNamespacedName, rbdmirror)
			if err != nil && errors.IsNotFound(err) {
				resource := &storagev1alpha1.RBDMirror{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: storagev1alpha1.RBDMirrorSpec{
						LocalCluster: storagev1alpha1.CephClusterConnectionSpec{
							ClusterID: "11111111-1111-4111-8111-111111111111",
							ConfigSecretRef: storagev1alpha1.CephSecretReference{
								Name: "local-ceph-config",
							},
							CredentialsSecretRef: storagev1alpha1.CephSecretReference{
								Name: "local-ceph-keyring",
							},
						},
						PeerCluster: storagev1alpha1.CephClusterConnectionSpec{
							ClusterID: "22222222-2222-4222-8222-222222222222",
							ConfigSecretRef: storagev1alpha1.CephSecretReference{
								Name: "peer-ceph-config",
							},
							CredentialsSecretRef: storagev1alpha1.CephSecretReference{
								Name: "peer-ceph-keyring",
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			By("cleaning up the RBDMirror resource")
			resource := &storagev1alpha1.RBDMirror{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
			}

			err := k8sClient.Delete(ctx, resource)
			if !errors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &RBDMirrorReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
