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
	*helper.NamespaceScopedAPIObject

	Key    client.ObjectKey
	Object *pvpoolv1alpha1.Checkout
}

func makeCheckout(key client.ObjectKey, obj *pvpoolv1alpha1.Checkout) *Checkout {
	c := &Checkout{Key: key, Object: obj}
	c.NamespaceScopedAPIObject = helper.ForNamespaceScopedAPIObject(&c.Key, lifecycle.TypedObject{GVK: CheckoutKind, Object: c.Object})
	return c
}

func (c *Checkout) Copy() *Checkout {
	return makeCheckout(c.Key, c.Object.DeepCopy())
}

func (c *Checkout) PersistStatus(ctx context.Context, cl client.Client) error {
	return cl.Status().Update(ctx, c.Object)
}

func (c *Checkout) Condition(typ pvpoolv1alpha1.CheckoutConditionType) (pvpoolv1alpha1.CheckoutCondition, bool) {
	for _, cond := range c.Object.Status.Conditions {
		if cond.Type == typ {
			return cond, true
		}
	}
	return pvpoolv1alpha1.CheckoutCondition{Type: typ}, false
}

func NewCheckout(key client.ObjectKey) *Checkout {
	return makeCheckout(key, &pvpoolv1alpha1.Checkout{})
}

func NewCheckoutFromObject(obj *pvpoolv1alpha1.Checkout) *Checkout {
	return makeCheckout(client.ObjectKeyFromObject(obj), obj)
}

func NewCheckoutPatcher(upd, orig *Pool) lifecycle.Persister {
	return helper.NewPatcher(upd.Object, orig.Object, helper.WithObjectKey(upd.Key))
}
