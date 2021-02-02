package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckoutKind is the public Kubernetes group-version-kind triple for the
// Checkout type.
var CheckoutKind = SchemeGroupVersion.WithKind("Checkout")

// Checkout requests a PVC from a Pool.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Claim",type="string",JSONPath=".status.volumeClaimRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Checkout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CheckoutSpec `json:"spec"`

	// +optional
	Status CheckoutStatus `json:"status"`
}

// CheckoutSpec is the configuration to request a particular PV from a Pool.
type CheckoutSpec struct {
	// PoolRef is the pool to check out a PVC from.
	PoolRef PoolReference `json:"poolRef"`

	// AccessModes are the access modes to assign to the checked out PVC.
	// Defaults to ReadWriteOnce.
	//
	// +optional
	// +kubebuilder:default={"ReadWriteOnce"}
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// CheckoutConditionType is the type of a Checkout condition.
type CheckoutConditionType string

const (
	// CheckoutAcquired indicates whether a Checkout has successfully taken a
	// PVC from the pool.
	CheckoutAcquired CheckoutConditionType = "Acquired"

	// CheckoutAcquiredReasonPoolDoesNotExist is used to indicate that the
	// poolRef points to a nonexistent pool.
	CheckoutAcquiredReasonPoolDoesNotExist = "PoolDoesNotExist"

	// CheckoutAcquiredReasonNotAvailable is used to indicate that the pool does
	// not have any available PVCs.
	CheckoutAcquiredReasonNotAvailable = "NotAvailable"

	// CheckoutAcquiredReasonInvalid is used to indicate that the PVC template
	// for this checkout is invalid.
	CheckoutAcquiredReasonInvalid = "Invalid"

	// CheckoutAcquiredReasonCheckedOut is used to indicate that a PVC was
	// successfully taken and is now available.
	CheckoutAcquiredReasonCheckedOut = "CheckedOut"
)

// CheckoutCondition is a status condition for a Checkout.
type CheckoutCondition struct {
	Condition `json:",inline"`

	// Type is the identifier for this condition.
	//
	// +kubebuilder:validation:Enum=Acquired
	Type CheckoutConditionType `json:"type"`
}

// CheckoutStatus is the runtime state of a checkout.
type CheckoutStatus struct {
	// VolumeName is the name of the volume being configured for the checkout.
	// It will track a volume from the upstream pool until its configuration is
	// copied to a new volume, at which point it will be permanently set to that
	// new volume.
	//
	// This field will be set as soon as a PVC is available in the pool.
	//
	// +optional
	VolumeName string `json:"volumeName,omitempty"`

	// VolumeClaimRef is a reference to the PVC checked out from the pool.
	//
	// This field will only be set when the checked out PVC is ready to be used.
	//
	// +optional
	VolumeClaimRef corev1.LocalObjectReference `json:"volumeClaimRef,omitempty"`

	// Conditions are the possible observable conditions for the checkout.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []CheckoutCondition `json:"conditions,omitempty"`
}

// CheckoutList enumerates many Checkout resources.
//
// +kubebuilder:object:root=true
type CheckoutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Checkout `json:"items"`
}
