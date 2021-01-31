package e2e_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPoolScaleUpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			key := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test",
			}
			p := eit.PoolHelpers.CreatePoolThenWaitSettled(ctx, key, CreatePoolWithReplicas(3))
			eit.PoolHelpers.ScalePoolThenWaitSettled(ctx, p, 2)
		})
	})
}

func TestPoolPVCReplacement(t *testing.T) {

}
