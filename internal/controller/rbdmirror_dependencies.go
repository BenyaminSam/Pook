package controller

import (
	"context"
	"fmt"
	"time"
	"errors"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	"github.com/BenyaminSam/pook/internal/ceph"
	pookkubernetes "github.com/BenyaminSam/pook/internal/kubernetes"
)

const dependencyRequeueAfter = 30 * time.Second

type secretDependency struct {
	role        string
	name        string
	namespace   string
	requiredKey string
}

type dependencyValidation struct {
	valid        bool
	reason       string
	message      string
	requeueAfter time.Duration
	err          error

	localConnection ceph.ConnectionInput
	peerConnection  ceph.ConnectionInput
}

func buildSecretDependencies(mirror *storagev1alpha1.RBDMirror) []secretDependency {
	return []secretDependency{
		{
			role:        "LocalCluster config",
			name:        mirror.Spec.LocalCluster.ConfigSecretRef.Name,
			namespace:   mirror.Namespace,
			requiredKey: storagev1alpha1.RBDMirrorConfigSecretKey,
		},
		{
			role:        "LocalCluster credentials",
			name:        mirror.Spec.LocalCluster.CredentialsSecretRef.Name,
			namespace:   mirror.Namespace,
			requiredKey: storagev1alpha1.RBDMirrorCredentialsSecretKey,
		},
		{
			role:        "PeerCluster config",
			name:        mirror.Spec.PeerCluster.ConfigSecretRef.Name,
			namespace:   mirror.Namespace,
			requiredKey: storagev1alpha1.RBDMirrorConfigSecretKey,
		},
		{
			role:        "PeerCluster credentials",
			name:        mirror.Spec.PeerCluster.CredentialsSecretRef.Name,
			namespace:   mirror.Namespace,
			requiredKey: storagev1alpha1.RBDMirrorCredentialsSecretKey,
		},
	}
}

func (r *RBDMirrorReconciler) validateDependencies(ctx context.Context, mirror *storagev1alpha1.RBDMirror) dependencyValidation {

	SecretAdapter := pookkubernetes.NewSecretAdapter(r.Client)

	localConnection, err := SecretAdapter.LoadConnectionInput(ctx, mirror.Namespace, mirror.Spec.LocalCluster)
	if err != nil {
		return dependencyValidationForError("LocalCluster", err)
	}

	peerConnection, err := SecretAdapter.LoadConnectionInput(ctx, mirror.Namespace, mirror.Spec.PeerCluster)
	if err != nil {
		return dependencyValidationForError("PeerCluster", err)
	}

	return dependencyValidation{
		valid:           true,
		reason:          storagev1alpha1.RBDMirrorReasonDependenciesValidated,
		message:         "Both Ceph cluster connections are valid",
		localConnection: localConnection,
		peerConnection:  peerConnection,
	}
}

func dependencyValidationForError(role string, err error) dependencyValidation {

	var secretErr *pookkubernetes.SecretError

	if !errors.As(err, &secretErr) {
		return dependencyValidation{
			valid:        false,
			reason:       storagev1alpha1.RBDMirrorReasonDependencyReadFailed,
			message:      fmt.Sprintf("Failed to load %s Ceph connection", role),
			requeueAfter: dependencyRequeueAfter,
			err:          fmt.Errorf("Load %s Ceph connection: %w", role, err),
		}
	}

	validation := dependencyValidation{
		valid:        false,
		requeueAfter: dependencyRequeueAfter,
	}

	switch secretErr.Kind {
	case pookkubernetes.SecretErrorInvalidReference:
		validation.reason = storagev1alpha1.RBDMirrorReasonSecretReferenceInvalid
		validation.message = fmt.Sprintf("%s Ceph connection reference is invalid", role)
		validation.requeueAfter = 0

	case pookkubernetes.SecretErrorNotFound:
		validation.reason = storagev1alpha1.RBDMirrorReasonSecretNotFound
		validation.message = fmt.Sprintf("%s: %s", role, secretErr.Error())

	case pookkubernetes.SecretErrorKeyMissing,
		pookkubernetes.SecretErrorEmpty:
		validation.reason = storagev1alpha1.RBDMirrorReasonSecretKeyMissing
		validation.message = fmt.Sprintf("%s: %s", role, secretErr.Error())

	case pookkubernetes.SecretErrorReadFailed:
		validation.reason = storagev1alpha1.RBDMirrorReasonDependencyReadFailed
		validation.message = fmt.Sprintf("failed to read %s Ceph Secret dependencies", role)
		validation.err = fmt.Errorf("load %s Ceph connection: %w", role, err)

	default:
		validation.reason = storagev1alpha1.RBDMirrorReasonDependencyReadFailed
		validation.message = fmt.Sprintf("Failed to load %s Ceph connection", role)
		validation.err = fmt.Errorf("Load %s Ceph connection: %w", role, err)
	}

	return validation
}
