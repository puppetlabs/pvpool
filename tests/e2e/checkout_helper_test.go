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

func (ch *CheckoutHelpers) WaitCheckedOut(ctx context.Context, co *obj.Checkout) (*obj.Checkout, error) {
	err := Wait(ctx, func(ctx context.Context) (bool, error) {
		if _, err := (lifecycle.RequiredLoader{Loader: co}).Load(ctx, ch.eit.ControllerClient); err != nil {
			return true, err
		}

		if co.Object.Status.VolumeClaimRef.Name == "" {
			return false, fmt.Errorf("no volume claim associated with checkout")
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return co, nil
}

func (ch *CheckoutHelpers) RequireWaitCheckedOut(ctx context.Context, co *obj.Checkout) *obj.Checkout {
	co, err := ch.WaitCheckedOut(ctx, co)
	require.NoError(ch.eit.t, err)
	return co
}

func (ch *CheckoutHelpers) CreateCheckout(ctx context.Context, key, poolKey client.ObjectKey) (*obj.Checkout, error) {
	co := obj.NewCheckout(key)
	co.Object.Spec = pvpoolv1alpha1.CheckoutSpec{
		PoolRef: pvpoolv1alpha1.PoolReference{
			Namespace: poolKey.Namespace,
			Name:      poolKey.Name,
		},
	}
	if err := co.Persist(ctx, ch.eit.ControllerClient); err != nil {
		return nil, err
	}

	return co, nil
}

func (ch *CheckoutHelpers) RequireCreateCheckout(ctx context.Context, key, poolKey client.ObjectKey) *obj.Checkout {
	co, err := ch.CreateCheckout(ctx, key, poolKey)
	require.NoError(ch.eit.t, err)
	return co
}

func (ch *CheckoutHelpers) CreateCheckoutThenWaitCheckedOut(ctx context.Context, key, poolKey client.ObjectKey) (*obj.Checkout, error) {
	co, err := ch.CreateCheckout(ctx, key, poolKey)
	if err != nil {
		return nil, err
	}

	return ch.WaitCheckedOut(ctx, co)
}

func (ch *CheckoutHelpers) RequireCreateCheckoutThenWaitCheckedOut(ctx context.Context, key, poolKey client.ObjectKey) *obj.Checkout {
	return ch.RequireWaitCheckedOut(ctx, ch.RequireCreateCheckout(ctx, key, poolKey))
}
