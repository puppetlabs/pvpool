package e2e_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCheckout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			p := eit.PoolHelpers.CreatePoolThenWaitSettled(ctx, poolKey, CreatePoolWithReplicas(3))
			eit.CheckoutHelpers.CreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, client.ObjectKey{Name: poolKey.Name})
			eit.PoolHelpers.WaitSettled(ctx, p)
		})
	})
}

func TestCheckoutAcrossNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns1 *corev1.Namespace) {
			eit.WithNamespace(ctx, func(ns2 *corev1.Namespace) {
				poolKey := client.ObjectKey{
					Namespace: ns1.GetName(),
					Name:      "test-pool",
				}
				checkoutKey := client.ObjectKey{
					Namespace: ns2.GetName(),
					Name:      "test-checkout",
				}
				p := eit.PoolHelpers.CreatePoolThenWaitSettled(ctx, poolKey, CreatePoolWithReplicas(3))
				eit.CheckoutHelpers.CreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey)
				eit.PoolHelpers.WaitSettled(ctx, p)
			})
		})
	})
}

func TestCheckoutWithInitJob(t *testing.T) {

}

func TestCheckoutBeforePoolCreation(t *testing.T) {

}

func TestCheckoutBeforePoolSettled(t *testing.T) {

}

func TestCheckoutPVCReplacement(t *testing.T) {

}

func TestCheckoutRBAC(t *testing.T) {

}
