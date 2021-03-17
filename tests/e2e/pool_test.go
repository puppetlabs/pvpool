package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/controller/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPoolScaleUpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			key := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test",
			}
			p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, key, WithReplicas(3))
			_ = eit.PoolHelpers.RequireScalePoolThenWaitSettled(ctx, p, 2)
		})
	})
}

func TestPoolPVCReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rng, err := rand.DefaultFactory.New()
	require.NoError(t, err)

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			key := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test",
			}
			p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, key, WithReplicas(3))

			ps := app.NewPoolState(p)
			ok, err := ps.Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Len(t, ps.Available, 3)

			// Pick a random PVC from the pool and delete it.
			pr, ok, err := ps.Available.Pop(rng)
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = pr.PersistentVolumeClaim.Delete(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)

			// Reload pool state until we observe a new replica.
			require.NoError(t, Wait(ctx, func(ctx context.Context) (bool, error) {
				if _, err := (lifecycle.RequiredLoader{Loader: ps}).Load(ctx, eit.ControllerClient); err != nil {
					return true, err
				}

				if len(ps.Available) != 3 {
					return false, fmt.Errorf("new replica was not created")
				}

				return true, nil
			}))
		})
	})
}

func TestPoolsWithSameSelector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			p1 := eit.PoolHelpers.RequireCreatePool(ctx, client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-1",
			}, WithReplicas(3))
			p2 := eit.PoolHelpers.RequireCreatePool(ctx, client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-2",
			}, WithReplicas(3))
			_ = eit.PoolHelpers.RequireWaitSettled(ctx, p1)
			_ = eit.PoolHelpers.RequireWaitSettled(ctx, p2)
		})
	})
}
