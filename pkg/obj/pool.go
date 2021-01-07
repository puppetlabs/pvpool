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
var _ lifecycle.Loader = &Pool{}
var _ lifecycle.Owner = &Pool{}

func (p *Pool) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	return helper.DeleteIgnoreNotFound(ctx, cl, p.Object, opts...)
}

func (p *Pool) Own(ctx context.Context, other lifecycle.Ownable) error {
	return other.Owned(ctx, lifecycle.TypedObject{GVK: PoolKind, Object: p.Object})
}

func (p *Pool) Load(ctx context.Context, cl client.Client) (bool, error) {
	return helper.GetIgnoreNotFound(ctx, cl, p.Key, p.Object)
}

func (p *Pool) PersistStatus(ctx context.Context, cl client.Client) error {
	return cl.Status().Update(ctx, p.Object)
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
