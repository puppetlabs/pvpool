package e2e_test

import (
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type WithReplicas int32

var _ CreatePoolOption = WithReplicas(0)

func (wr WithReplicas) ApplyToCreatePoolOptions(target *CreatePoolOptions) {
	target.Replicas = (*int32)(&wr)
}

type WithAccessModes []corev1.PersistentVolumeAccessMode

var _ CreateCheckoutOption = WithAccessModes(nil)
var _ CreatePoolOption = WithAccessModes(nil)

func (wam WithAccessModes) ApplyToCreateCheckoutOptions(target *CreateCheckoutOptions) {
	target.AccessModes = wam
}

func (wam WithAccessModes) ApplyToCreatePoolOptions(target *CreatePoolOptions) {
	target.AccessModes = wam
}

type WithInitJob pvpoolv1alpha1.MountJob

var _ CreatePoolOption = WithInitJob{}

// nolint:gocritic // This is the most expressive way to represent this test
//                 // option.
func (wij WithInitJob) ApplyToCreatePoolOptions(target *CreatePoolOptions) {
	target.InitJob = (*pvpoolv1alpha1.MountJob)(&wij)
}
