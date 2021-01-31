package obj

import (
	"context"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var CheckoutKind = pvpoolv1alpha1.CheckoutKind

type Checkout struct {
	Key    client.ObjectKey
	Object *pvpoolv1alpha1.Checkout
}

var _ lifecycle.Deleter = &Checkout{}
var _ lifecycle.Loader = &Checkout{}
var _ lifecycle.Owner = &Checkout{}
var _ lifecycle.Persister = &Checkout{}

func (c *Checkout) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	return helper.DeleteIgnoreNotFound(ctx, cl, c.Object, opts...)
}

func (c *Checkout) Own(ctx context.Context, other lifecycle.Ownable) error {
	return other.Owned(ctx, lifecycle.TypedObject{GVK: CheckoutKind, Object: c.Object})
}

func (c *Checkout) Load(ctx context.Context, cl client.Client) (bool, error) {
	return helper.GetIgnoreNotFound(ctx, cl, c.Key, c.Object)
}

func (c *Checkout) Persist(ctx context.Context, cl client.Client) error {
	return helper.CreateOrUpdate(ctx, cl, c.Object, helper.WithObjectKey(c.Key))
}

func (c *Checkout) PersistStatus(ctx context.Context, cl client.Client) error {
	return cl.Status().Update(ctx, c.Object)
}

func NewCheckout(key client.ObjectKey) *Checkout {
	return &Checkout{
		Key:    key,
		Object: &pvpoolv1alpha1.Checkout{},
	}
}

func NewCheckoutFromObject(obj *pvpoolv1alpha1.Checkout) *Checkout {
	return &Checkout{
		Key:    client.ObjectKeyFromObject(obj),
		Object: obj,
	}
}
