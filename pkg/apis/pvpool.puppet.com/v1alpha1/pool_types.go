package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PoolKind is the public Kubernetes group-version-kind triple for the Pool
// type.
var PoolKind = SchemeGroupVersion.WithKind("Pool")

// Pool is a collection of preconfigured persistent volumes that can be taken
// and recycled as needed.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Pool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PoolSpec `json:"spec"`

	// +optional
	Status PoolStatus `json:"status"`
}

// PoolSpec is the configuration for a pool.
type PoolSpec struct {
	// Replicas are the number of PVs to make available in the pool.
	//
	// Once a PV is checked out from the pool, it no longer counts toward the
	// number replicas. Setting this field to 0 will make the pool unusable.
	//
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Selector is the label selector for PVCs maintained in the pool.
	//
	// The selector must match a subset of the labels in the template.
	Selector metav1.LabelSelector `json:"selector"`

	// Template describes the configuration of the dynamic PVCs that this
	// controller should manage.
	Template PersistentVolumeClaimTemplate `json:"template"`

	// InitJob configures a job to process newly created PVs before they are
	// made available as part of the pool.
	//
	// +optional
	InitJob *MountJob `json:"initJob,omitempty"`
}

// MountJob is a job that has a persistent volume attached to it with a
// configured name.
type MountJob struct {
	// Template is the configuration for the job.
	Template JobTemplate `json:"template"`

	// VolumeName is the name of the volume to be added to the template to
	// access the persistent volume. The volume must either not exist in the
	// template or must have a persistent volume claim source.
	//
	// +optional
	// +kubebuilder:default="workspace"
	VolumeName string `json:"volumeName,omitempty"`
}

// PoolStatus is the runtime state of an existing pool.
type PoolStatus struct {
	// ObservedGeneration is the generation of the resource specification that
	// this status matches.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas are the number of PVCs that currently exist that match this
	// pool's selector.
	//
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// AvailableReplicas are the number of PVs from this pool that are ready to
	// be checked out.
	//
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

// PoolList enumerates many Pool resources.
//
// +kubebuilder:object:root=true
type PoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pool `json:"items"`
}

// PoolReference is a reference to a Pool.
type PoolReference struct {
	// Namespace identifies the Kubernetes namespace of the pool.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name identifies the name of the pool within the namespace.
	Name string `json:"name"`
}
