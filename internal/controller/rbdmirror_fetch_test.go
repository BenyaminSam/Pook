package controller

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
)

func newFetchTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := storagev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("register RBDMirror API: %v", err)
	}

	return scheme
}

func TestReconcileMissingRBDMirror(t *testing.T) {
	scheme := newFetchTestScheme(t)
	testClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &RBDMirrorReconciler{
		Client: testClient,
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "missing",
		},
	})
	if err != nil {
		t.Fatalf("expected missing resource to be ignored, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected no explicit requeue, got: %+v", result)
	}
}

type fetchErrorClient struct {
	client.Client
	err error
}

func (c *fetchErrorClient) Get(
	_ context.Context,
	_ client.ObjectKey,
	_ client.Object,
	_ ...client.GetOption,
) error {
	return c.err
}

func TestReconcileReturnsFetchError(t *testing.T) {
	scheme := newFetchTestScheme(t)
	testClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	injectedErr := apierrors.NewForbidden(
		schema.GroupResource{
			Group:    "storage.pook.io",
			Resource: "rbdmirrors",
		},
		"example",
		errors.New("access denied"),
	)

	reconciler := &RBDMirrorReconciler{
		Client: &fetchErrorClient{
			Client: testClient,
			err:    injectedErr,
		},
		Scheme: scheme,
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "example",
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected wrapped fetch error, got: %v", err)
	}
}

func TestReconcileDeletingRBDMirror(t *testing.T) {
	scheme := newFetchTestScheme(t)
	now := metav1.Now()

	mirror := &storagev1alpha1.RBDMirror{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "default",
			Name:              "deleting",
			DeletionTimestamp: &now,
			// Keeps the deleting object present in the fake client.
			// This is not a finalizer implemented by pook.
			Finalizers: []string{"tests.pook.io/hold"},
		},
	}

	testClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(mirror).
		Build()

	reconciler := &RBDMirrorReconciler{
		Client: testClient,
		Scheme: scheme,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(mirror),
	})
	if err != nil {
		t.Fatalf("expected deleting resource to be skipped, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected no explicit requeue, got: %+v", result)
	}
}
