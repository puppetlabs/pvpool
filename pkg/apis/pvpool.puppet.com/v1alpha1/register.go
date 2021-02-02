package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is the public Kubernetes group-version pair for this
// package.
var SchemeGroupVersion = schema.GroupVersion{Group: "pvpool.puppet.com", Version: "v1alpha1"}

// Resource returns the public Kubernetes group-version-resource triple for a
// given resource in this package.
func Resource(resource string) schema.GroupVersionResource {
	return SchemeGroupVersion.WithResource(resource)
}

var (
	// SchemeBuilder allows this package to be used with dynamic Kubernetes
	// clients to manage Kubernetes objects.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types from this package to another scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Checkout{},
		&CheckoutList{},
		&Pool{},
		&PoolList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
