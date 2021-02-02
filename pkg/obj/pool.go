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
	Key    client.ObjectKey
	Object *pvpoolv1alpha1.Pool
}

var _ lifecycle.Deleter = &Pool{}
var _ lifecycle.Finalizable = &Pool{}
var _ lifecycle.Loader = &Pool{}
var _ lifecycle.Owner = &Pool{}
var _ lifecycle.Persister = &Pool{}

func (p *Pool) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	return helper.DeleteIgnoreNotFound(ctx, cl, p.Object, opts...)
}

func (p *Pool) Finalizing() bool {
	return !p.Object.GetDeletionTimestamp().IsZero()
}

func (p *Pool) AddFinalizer(ctx context.Context, name string) bool {
	return helper.AddFinalizer(p.Object, name)
}

func (p *Pool) RemoveFinalizer(ctx context.Context, name string) bool {
	return helper.RemoveFinalizer(p.Object, name)
}

func (p *Pool) Load(ctx context.Context, cl client.Client) (bool, error) {
	return helper.GetIgnoreNotFound(ctx, cl, p.Key, p.Object)
}

func (p *Pool) Own(ctx context.Context, other lifecycle.Ownable) error {
	return other.Owned(ctx, lifecycle.TypedObject{GVK: PoolKind, Object: p.Object})
}

func (p *Pool) Persist(ctx context.Context, cl client.Client) error {
	return helper.CreateOrUpdate(ctx, cl, p.Object, helper.WithObjectKey(p.Key))
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
	return &Pool{
		Key:    key,
		Object: &pvpoolv1alpha1.Pool{},
	}
}

func NewPoolFromObject(obj *pvpoolv1alpha1.Pool) *Pool {
	return &Pool{
		Key:    client.ObjectKeyFromObject(obj),
		Object: obj,
	}
}
