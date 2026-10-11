package controller

import (
	"context"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const rbdMirrorSecretRefIndex = "storage.pook.io/rbdmirror-secret-ref"

func indexRBDMirrorSecretRef(obj client.Object) []string {
	mirror, ok := obj.(*storagev1alpha1.RBDMirror)
	if !ok {
		return nil
	}

	dependencies := buildSecretDependencies(mirror)
	names := make([]string, 0, len(dependencies))
	seen := make(map[string]struct{}, len(dependencies))

	for _, dependency := range dependencies {
		if dependency.name == "" {
			continue
		}

		if _, exists := seen[dependency.name]; exists {
			continue
		}

		seen[dependency.name] = struct{}{}
		names = append(names, dependency.name)
	}
	return names
}

func (r *RBDMirrorReconciler) mapSecretToRBDMirrors(ctx context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	logger := log.FromContext(ctx).WithValues("secret", client.ObjectKeyFromObject(secret))

	var mirrors storagev1alpha1.RBDMirrorList
	if err := r.List(ctx, &mirrors, client.InNamespace(secret.Namespace), client.MatchingFields{rbdMirrorSecretRefIndex: secret.Name}); err != nil {
		logger.Error(err, "unable to list RBDMirrors referencing secret")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(mirrors.Items))
	for i := range mirrors.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&mirrors.Items[i])})
	}

	logger.V(1).Info("Mapped Secret event to RBDMirrors", "requestCount", len(requests))

	return requests
}
