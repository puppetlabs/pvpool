package e2e_test

import (
	"context"
	"testing"

	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPoolScaleUpDown(t *testing.T) {
	ctx := context.Background()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			p := obj.NewPool(client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test",
			})
			require.NoError(t, p.Persist(ctx, eit.ControllerClient))
		})
	})
}
