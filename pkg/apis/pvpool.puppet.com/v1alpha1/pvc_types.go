package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PersistentVolumeClaimTemplate is a subset of a core persistent volume claim
// that can be used as a template in an object spec.
type PersistentVolumeClaimTemplate struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec corev1.PersistentVolumeClaimSpec `json:"spec"`
}
