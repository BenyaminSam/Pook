package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RBDMirrorPhase string

const (
	RBDMirrorPhasePending     RBDMirrorPhase = "Pending"
	RBDMirrorPhaseConfiguring RBDMirrorPhase = "Configuring"
	RBDMirrorPhaseReady       RBDMirrorPhase = "Ready"
	RBDMirrorPhaseFailed      RBDMirrorPhase = "Failed"
)

type RBDMirrorConditionType string

const (
	RBDMirrorConditionReady       RBDMirrorConditionType = "Ready"
	RBDMirrorConditionProgressing RBDMirrorConditionType = "Progressing"
	RBDMirrorConditionDegraded    RBDMirrorConditionType = "Degraded"
)

const (
	RBDMirrorReasonDependenciesNotChecked   = "DependenciesNotChecked"
	RBDMirrorReasonReconciliationNotStarted = "ReconciliationNotStarted"

	RBDMirrorReasonDependenciesValidated  = "DependenciesValidated"
	RBDMirrorReasonDependencyUnavailable  = "DependencyUnavailable"
	RBDMirrorReasonSecretNotFound         = "SecretNotFound"
	RBDMirrorReasonSecretKeyMissing       = "SecretKeyMissing"
	RBDMirrorReasonDependencyReadFailed   = "DependencyReadFailed"
	RBDMirrorReasonSecretReferenceInvalid = "SecretReferenceInvalid"
	RBDMirrorReasonMirroringNotConfigured = "MirroringNotConfigured"

	RBDMirrorReasonCephExecutorUnavailable         = "CephExecutorUnavailable"
	RBDMirrorReasonCephConnectionPreparationFailed = "CephConnectionPreparationFailed"
	RBDMirrorReasonCephConnectionCleanupFailed     = "CephConnectionCleanupFailed"
	RBDMirrorReasonCephObservationFailed           = "CephObservationFailed"
	RBDMirrorReasonCephClusterIDMismatch           = "CephClusterIDMismatch"
	RBDMirrorReasonCephPoolNotFound                = "CephPoolNotFound"

	RBDMirrorReasonCephPoolObservationFailed      = "CephPoolObservationFailed"
	RBDMirrorReasonCephObservationSucceeded       = "CephObservationSucceeded"
	RBDMirrorReasonCephMirroringObservationFailed = "CephMirroringObservationFailed"
)

const (
	RBDMirrorConfigSecretKey      = "ceph.conf"
	RBDMirrorCredentialsSecretKey = "keyring"
)

type CephSecretReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type CephClusterConnectionSpec struct {
	// +kubebuilder:validation:MinLength=1
	ClusterID string `json:"clusterID"`

	// Example: client.pook
	// +kubebuilder:validation:MinLength=1
	ClientName string `json:"clientName"`

	ConfigSecretRef      CephSecretReference `json:"configSecretRef"`
	CredentialsSecretRef CephSecretReference `json:"credentialsSecretRef"`
}

type RBDMirrorImageSelectionSpec struct {

	// +kubebuilder:validation:Enum=Pool
	// +kubebuilder:default=Pool
	// +optional
	Policy string `json:"policy,omitempty"`
}

type RBDMirrorPoolSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:Enum=journal
	Mode string `json:"mode"`

	// +optional
	ImageSelection RBDMirrorImageSelectionSpec `json:"imageSelection,omitempty"`

	// Direction specifies the mirroring direction relative to LocalCluster
	// rx-only means LocalCluster receives mirrored images from PeerCluster
	// rx-tx means both clusters receive mirrored images from each other
	// +kubebuilder:validation:Enum=rx-only;rx-tx
	// +kubebuilder:default=rx-tx
	// +optional
	Direction string `json:"direction,omitempty"`
}

type RBDMirrorDaemonSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// +optional
	Image string `json:"image,omitempty"`
}

type RBDMirrorSpec struct {
	LocalCluster CephClusterConnectionSpec `json:"localCluster"`
	PeerCluster  CephClusterConnectionSpec `json:"peerCluster"`

	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	Pools []RBDMirrorPoolSpec `json:"pools"`

	// +optional
	Daemon RBDMirrorDaemonSpec `json:"daemon,omitempty"`
}

type RBDMirrorStatus struct {

	// +optional
	Phase RBDMirrorPhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
type RBDMirror struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of RBDMirror
	// +required
	Spec RBDMirrorSpec `json:"spec"`

	// status defines the observed state of RBDMirror
	// +optional
	Status RBDMirrorStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RBDMirrorList contains a list of RBDMirror
type RBDMirrorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []RBDMirror `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &RBDMirror{}, &RBDMirrorList{})
		return nil
	})
}
