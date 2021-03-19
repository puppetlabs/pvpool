package e2e_test

import (
	"context"
	"fmt"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	pvpoolv1alpha1obj "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1/obj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PoolHelpers struct {
	eit *EnvironmentInTest
}

func (ph *PoolHelpers) WaitSettled(ctx context.Context, p *pvpoolv1alpha1obj.Pool) (*pvpoolv1alpha1obj.Pool, error) {
	err := Wait(ctx, func(ctx context.Context) (bool, error) {
		if _, err := (lifecycle.RequiredLoader{Loader: p}).Load(ctx, ph.eit.ControllerClient); err != nil {
			return true, err
		}

		if p.Object.Status.ObservedGeneration != p.Object.GetGeneration() {
			return false, fmt.Errorf("pool status is for generation %d, but pool has updated to generation %d", p.Object.Status.ObservedGeneration, p.Object.GetGeneration())
		}

		if cond, _ := p.Condition(pvpoolv1alpha1.PoolSettlement); cond.Status != corev1.ConditionTrue {
			return false, fmt.Errorf("pool has %d available replicas", p.Object.Status.AvailableReplicas)
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (ph *PoolHelpers) RequireWaitSettled(ctx context.Context, p *pvpoolv1alpha1obj.Pool) *pvpoolv1alpha1obj.Pool {
	p, err := ph.WaitSettled(ctx, p)
	require.NoError(ph.eit.t, err)
	return p
}

type CreatePoolOptions struct {
	Replicas    *int32
	AccessModes []corev1.PersistentVolumeAccessMode
	InitJob     *pvpoolv1alpha1.MountJob
}

type CreatePoolOption interface {
	ApplyToCreatePoolOptions(target *CreatePoolOptions)
}

func (o *CreatePoolOptions) ApplyOptions(opts []CreatePoolOption) {
	for _, opt := range opts {
		opt.ApplyToCreatePoolOptions(o)
	}
}

func (ph *PoolHelpers) CreatePool(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) (*pvpoolv1alpha1obj.Pool, error) {
	o := &CreatePoolOptions{}
	o.ApplyOptions(opts)

	p := pvpoolv1alpha1obj.NewPool(key)
	p.Object.Spec = pvpoolv1alpha1.PoolSpec{
		Replicas: o.Replicas,
		Selector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
		Template: pvpoolv1alpha1.PersistentVolumeClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "test",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      o.AccessModes,
				StorageClassName: pointer.StringPtr(ph.eit.StorageClassName),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Mi"),
					},
				},
			},
		},
		InitJob: o.InitJob,
	}
	if err := p.Persist(ctx, ph.eit.ControllerClient); err != nil {
		return nil, err
	}

	return p, nil
}

func (ph *PoolHelpers) RequireCreatePool(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) *pvpoolv1alpha1obj.Pool {
	p, err := ph.CreatePool(ctx, key, opts...)
	require.NoError(ph.eit.t, err)
	return p
}

func (ph *PoolHelpers) CreatePoolThenWaitSettled(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) (*pvpoolv1alpha1obj.Pool, error) {
	p, err := ph.CreatePool(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	return ph.WaitSettled(ctx, p)
}

func (ph *PoolHelpers) RequireCreatePoolThenWaitSettled(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) *pvpoolv1alpha1obj.Pool {
	return ph.RequireWaitSettled(ctx, ph.RequireCreatePool(ctx, key, opts...))
}

func (ph *PoolHelpers) ScalePool(ctx context.Context, p *pvpoolv1alpha1obj.Pool, replicas int32) (*pvpoolv1alpha1obj.Pool, error) {
	p.Object.Spec.Replicas = &replicas
	if err := p.Persist(ctx, ph.eit.ControllerClient); err != nil {
		return nil, err
	}

	return p, nil
}

func (ph *PoolHelpers) RequireScalePool(ctx context.Context, p *pvpoolv1alpha1obj.Pool, replicas int32) *pvpoolv1alpha1obj.Pool {
	p, err := ph.ScalePool(ctx, p, replicas)
	require.NoError(ph.eit.t, err)
	return p
}

func (ph *PoolHelpers) ScalePoolThenWaitSettled(ctx context.Context, p *pvpoolv1alpha1obj.Pool, replicas int32) (*pvpoolv1alpha1obj.Pool, error) {
	p, err := ph.ScalePool(ctx, p, replicas)
	if err != nil {
		return nil, err
	}

	return ph.WaitSettled(ctx, p)
}

func (ph *PoolHelpers) RequireScalePoolThenWaitSettled(ctx context.Context, p *pvpoolv1alpha1obj.Pool, replicas int32) *pvpoolv1alpha1obj.Pool {
	return ph.RequireWaitSettled(ctx, ph.RequireScalePool(ctx, p, replicas))
}
