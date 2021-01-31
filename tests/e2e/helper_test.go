package e2e_test

import (
	"context"
	"fmt"
	"time"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/timeutil/pkg/backoff"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var backoffFactory = backoff.Build(
	backoff.Exponential(250*time.Millisecond, 2.0),
	backoff.MaxBound(5*time.Second),
	backoff.FullJitter(),
	backoff.NonSliding,
)

func Wait(ctx context.Context, work retry.WorkFunc) error {
	return retry.Wait(ctx, work, retry.WithBackoffFactory(backoffFactory))
}

type PoolHelpers struct {
	eit *EnvironmentInTest
}

func (ph *PoolHelpers) WaitSettled(ctx context.Context, p *obj.Pool) *obj.Pool {
	require.NoError(ph.eit.t, Wait(ctx, func(ctx context.Context) (bool, error) {
		if _, err := (lifecycle.RequiredLoader{Loader: p}).Load(ctx, ph.eit.ControllerClient); err != nil {
			return true, err
		}

		var request int32 = 1
		if p.Object.Spec.Replicas != nil {
			request = *p.Object.Spec.Replicas
		}

		avail := p.Object.Status.AvailableReplicas

		switch {
		case request > avail:
			return false, fmt.Errorf("scaling up: pool has %d replicas, but needs %d", avail, request)
		case request < avail:
			return false, fmt.Errorf("scaling down: pool has %d replicas, but needs %d", avail, request)
		default:
			return true, nil
		}
	}))
	return p
}

type CreatePoolOptions struct {
	Replicas *int32
}

type CreatePoolOption interface {
	ApplyToCreatePoolOptions(target *CreatePoolOptions)
}

func (o *CreatePoolOptions) ApplyOptions(opts []CreatePoolOption) {
	for _, opt := range opts {
		opt.ApplyToCreatePoolOptions(o)
	}
}

type CreatePoolWithReplicas int32

var _ CreatePoolOption = CreatePoolWithReplicas(0)

func (wr CreatePoolWithReplicas) ApplyToCreatePoolOptions(target *CreatePoolOptions) {
	target.Replicas = (*int32)(&wr)
}

func (ph *PoolHelpers) CreatePool(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) *obj.Pool {
	o := &CreatePoolOptions{}
	o.ApplyOptions(opts)

	p := obj.NewPool(key)
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
				StorageClassName: func(s string) *string { return &s }("local-path"),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Mi"),
					},
				},
			},
		},
	}
	require.NoError(ph.eit.t, p.Persist(ctx, ph.eit.ControllerClient))

	return p
}

func (ph *PoolHelpers) CreatePoolThenWaitSettled(ctx context.Context, key client.ObjectKey, opts ...CreatePoolOption) *obj.Pool {
	return ph.WaitSettled(ctx, ph.CreatePool(ctx, key, opts...))
}

func (ph *PoolHelpers) ScalePool(ctx context.Context, p *obj.Pool, replicas int32) *obj.Pool {
	p.Object.Spec.Replicas = &replicas
	require.NoError(ph.eit.t, p.Persist(ctx, ph.eit.ControllerClient))
	return p
}

func (ph *PoolHelpers) ScalePoolThenWaitSettled(ctx context.Context, p *obj.Pool, replicas int32) *obj.Pool {
	return ph.WaitSettled(ctx, ph.ScalePool(ctx, p, replicas))
}
