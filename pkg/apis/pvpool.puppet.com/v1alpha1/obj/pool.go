package obj

import (
	"context"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var PoolKind = pvpoolv1alpha1.PoolKind

type Pool struct {
	*helper.NamespaceScopedAPIObject

	Key    client.ObjectKey
	Object *pvpoolv1alpha1.Pool
}

func makePool(key client.ObjectKey, obj *pvpoolv1alpha1.Pool) *Pool {
	p := &Pool{Key: key, Object: obj}
	p.NamespaceScopedAPIObject = helper.ForNamespaceScopedAPIObject(&p.Key, lifecycle.TypedObject{GVK: PoolKind, Object: p.Object})
	return p
}

func (p *Pool) Copy() *Pool {
	return makePool(p.Key, p.Object.DeepCopy())
}

func (p *Pool) PersistStatus(ctx context.Context, cl client.Client) error {
	return cl.Status().Update(ctx, p.Object)
}

func (p *Pool) Condition(typ pvpoolv1alpha1.PoolConditionType) (pvpoolv1alpha1.PoolCondition, bool) {
	for _, cond := range p.Object.Status.Conditions {
		if cond.Type == typ {
			return cond, true
		}
	}
	return pvpoolv1alpha1.PoolCondition{Type: typ}, false
}

func NewPool(key client.ObjectKey) *Pool {
	return makePool(key, &pvpoolv1alpha1.Pool{})
}

func NewPoolFromObject(obj *pvpoolv1alpha1.Pool) *Pool {
	return makePool(client.ObjectKeyFromObject(obj), obj)
}

func NewPoolPatcher(upd, orig *Pool) lifecycle.Persister {
	return helper.NewPatcher(upd.Object, orig.Object, helper.WithObjectKey(upd.Key))
}
