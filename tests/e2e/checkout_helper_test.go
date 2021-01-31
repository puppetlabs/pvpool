package e2e_test

import (
	"context"
	"fmt"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheckoutHelpers struct {
	eit *EnvironmentInTest
}

func (ch *CheckoutHelpers) WaitCheckedOut(ctx context.Context, co *obj.Checkout) *obj.Checkout {
	require.NoError(ch.eit.t, Wait(ctx, func(ctx context.Context) (bool, error) {
		if _, err := (lifecycle.RequiredLoader{Loader: co}).Load(ctx, ch.eit.ControllerClient); err != nil {
			return true, err
		}

		if co.Object.Status.VolumeClaimRef.Name == "" {
			return false, fmt.Errorf("no volume claim associated with checkout")
		}

		return true, nil
	}))
	return co
}

func (ch *CheckoutHelpers) CreateCheckout(ctx context.Context, key, poolKey client.ObjectKey) *obj.Checkout {
	co := obj.NewCheckout(key)
	co.Object.Spec = pvpoolv1alpha1.CheckoutSpec{
		PoolRef: pvpoolv1alpha1.PoolReference{
			Namespace: poolKey.Namespace,
			Name:      poolKey.Name,
		},
	}
	require.NoError(ch.eit.t, co.Persist(ctx, ch.eit.ControllerClient))

	return co
}

func (ch *CheckoutHelpers) CreateCheckoutThenWaitCheckedOut(ctx context.Context, key, poolKey client.ObjectKey) *obj.Checkout {
	return ch.WaitCheckedOut(ctx, ch.CreateCheckout(ctx, key, poolKey))
}
