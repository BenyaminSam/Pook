package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
	"github.com/BenyaminSam/pook/internal/ceph"
)

type SecretErrorKind string

const (
	SecretErrorInvalidReference SecretErrorKind = "InvalidReference"
	SecretErrorNotFound         SecretErrorKind = "NotFound"
	SecretErrorKeyMissing       SecretErrorKind = "KeyMissing"
	SecretErrorReadFailed       SecretErrorKind = "ReadFailed"
	SecretErrorEmpty            SecretErrorKind = "Empty"
)

type SecretError struct {
	Kind      SecretErrorKind
	Namespace string
	Name      string
	Key       string
	Cause     error
}

func (e *SecretError) Error() string {
	switch e.Kind {
	case SecretErrorInvalidReference:
		return "Ceph secret reference is invalid"
	case SecretErrorNotFound:
		return fmt.Sprintf("Ceph secret %s/%s was not found", e.Namespace, e.Name)
	case SecretErrorKeyMissing:
		return fmt.Sprintf("Required key %q is missing from ceph Secret %s/%s", e.Key, e.Namespace, e.Name)
	case SecretErrorEmpty:
		return fmt.Sprintf("Failed to read Ceph Secret %s/%s", e.Namespace, e.Name)
	case SecretErrorReadFailed:
		return fmt.Sprintf("Failed to read ceph secret %s/%s", e.Namespace, e.Name)
	default:
		return "Failed to load Ceph Secret data"
	}
}

func (e *SecretError) Unwrap() error {
	return e.Cause
}

type SecretAdapter struct {
	reader client.Reader
}

// var _ client.Reader = (*SecretAdapter)(nil)

func NewSecretAdapter(reader client.Reader) *SecretAdapter {
	return &SecretAdapter{reader: reader}
}

func (a *SecretAdapter) LoadConnectionInput(ctx context.Context, namespace string, spec storagev1alpha1.CephClusterConnectionSpec) (ceph.ConnectionInput, error) {
	if a == nil || a.reader == nil {
		return ceph.ConnectionInput{}, fmt.Errorf("Ceph Secret adapter has no Kuberentes reader")
	}

	if strings.TrimSpace(namespace) == "" {
		return ceph.ConnectionInput{}, &SecretError{
			Kind:  SecretErrorInvalidReference,
			Cause: errors.New("namespace is empty"),
		}
	}

	if strings.TrimSpace(spec.ClusterID) == "" {
		return ceph.ConnectionInput{}, &SecretError{
			Kind:  SecretErrorInvalidReference,
			Cause: errors.New("Cluster ID is empty"),
		}
	}

	if strings.TrimSpace(spec.ClientName) == "" {
		return ceph.ConnectionInput{}, &SecretError{
			Kind:  SecretErrorInvalidReference,
			Cause: errors.New("client name is empty"),
		}
	}

	configSecretName := strings.TrimSpace(spec.ConfigSecretRef.Name)
	if configSecretName == "" {
		return ceph.ConnectionInput{}, &SecretError{
			Kind:  SecretErrorInvalidReference,
			Cause: errors.New("config Secret name is empty"),
		}
	}

	credentialsSecretName := strings.TrimSpace(spec.CredentialsSecretRef.Name)
	if credentialsSecretName == "" {
		return ceph.ConnectionInput{}, &SecretError{
			Kind:  SecretErrorInvalidReference,
			Cause: errors.New("credentials Secret name is empty"),
		}
	}

	configSecret, err := a.getSecret(ctx, namespace, configSecretName)
	if err != nil {
		return ceph.ConnectionInput{}, err
	}

	credentialsSecret, err := a.getSecret(ctx, namespace, credentialsSecretName)
	if err != nil {
		return ceph.ConnectionInput{}, err
	}

	configData, err := requiredSecretData(configSecret, storagev1alpha1.RBDMirrorConfigSecretKey)
	if err != nil {
		return ceph.ConnectionInput{}, err
	}

	keyringData, err := requiredSecretData(credentialsSecret, storagev1alpha1.RBDMirrorCredentialsSecretKey)
	if err != nil {
		return ceph.ConnectionInput{}, err
	}

	return ceph.ConnectionInput{
		ClusterID:   spec.ClusterID,
		ClientName:  spec.ClientName,
		ConfigData:  configData,
		KeyringData: keyringData,
	}, nil
}

func (a *SecretAdapter) getSecret(ctx context.Context, namespace string, name string) (*corev1.Secret, error) {
	secert := &corev1.Secret{}
	err := a.reader.Get(
		ctx,
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		secert,
	)

	if err == nil {
		return secert, nil
	}

	if apierrors.IsNotFound(err) {
		return nil, &SecretError{
			Kind:      SecretErrorNotFound,
			Namespace: namespace,
			Name:      name,
			Cause:     err,
		}
	}

	return nil, &SecretError{
		Kind:      SecretErrorReadFailed,
		Namespace: namespace,
		Name:      name,
		Cause:     err,
	}
}

func requiredSecretData(secret *corev1.Secret, key string) ([]byte, error) {
	if secret == nil {
		return nil, &SecretError{
			Kind:  SecretErrorReadFailed,
			Key:   key,
			Cause: errors.New("Secret object is nil"),
		}
	}

	value, exists := secret.Data[key]
	if !exists {
		return nil, &SecretError{
			Kind:      SecretErrorKeyMissing,
			Namespace: secret.Namespace,
			Name:      secret.Name,
			Key:       key,
		}
	}

	if len(value) == 0 || len(strings.TrimSpace(string(value))) == 0 {
		return nil, &SecretError{
			Kind:      SecretErrorEmpty,
			Namespace: secret.Namespace,
			Name:      secret.Name,
			Key:       key,
		}
	}

	return append([]byte(nil), value...), nil
}
