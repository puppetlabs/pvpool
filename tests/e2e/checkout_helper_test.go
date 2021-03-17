package e2e_test

import (
	"context"
	"fmt"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

		if cond, _ := co.Condition(pvpoolv1alpha1.CheckoutAcquired); cond.Status != corev1.ConditionTrue {
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

type CreateCheckoutOptions struct {
	AccessModes []corev1.PersistentVolumeAccessMode
}

type CreateCheckoutOption interface {
	ApplyToCreateCheckoutOptions(target *CreateCheckoutOptions)
}

func (o *CreateCheckoutOptions) ApplyOptions(opts []CreateCheckoutOption) {
	for _, opt := range opts {
		opt.ApplyToCreateCheckoutOptions(o)
	}
}

func (ch *CheckoutHelpers) CreateCheckout(ctx context.Context, key, poolKey client.ObjectKey, opts ...CreateCheckoutOption) (*obj.Checkout, error) {
	o := &CreateCheckoutOptions{}
	o.ApplyOptions(opts)

	co := obj.NewCheckout(key)
	co.Object.Spec = pvpoolv1alpha1.CheckoutSpec{
		PoolRef: pvpoolv1alpha1.PoolReference{
			Namespace: poolKey.Namespace,
			Name:      poolKey.Name,
		},
		AccessModes: o.AccessModes,
	}
	if err := co.Persist(ctx, ch.eit.ControllerClient); err != nil {
		return nil, err
	}

	return co, nil
}

func (ch *CheckoutHelpers) RequireCreateCheckout(ctx context.Context, key, poolKey client.ObjectKey, opts ...CreateCheckoutOption) *obj.Checkout {
	co, err := ch.CreateCheckout(ctx, key, poolKey, opts...)
	require.NoError(ch.eit.t, err)
	return co
}

func (ch *CheckoutHelpers) CreateCheckoutThenWaitCheckedOut(ctx context.Context, key, poolKey client.ObjectKey, opts ...CreateCheckoutOption) (*obj.Checkout, error) {
	co, err := ch.CreateCheckout(ctx, key, poolKey, opts...)
	if err != nil {
		return nil, err
	}

	return ch.WaitCheckedOut(ctx, co)
}

func (ch *CheckoutHelpers) RequireCreateCheckoutThenWaitCheckedOut(ctx context.Context, key, poolKey client.ObjectKey, opts ...CreateCheckoutOption) *obj.Checkout {
	return ch.RequireWaitCheckedOut(ctx, ch.RequireCreateCheckout(ctx, key, poolKey, opts...))
}
