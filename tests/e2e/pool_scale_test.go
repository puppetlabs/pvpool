package e2e_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPoolScaleUpDown(t *testing.T) {
	ctx := context.Background()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			p := obj.NewPool(client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test",
			})
			p.Object.Spec = pvpoolv1alpha1.PoolSpec{
				Replicas: func(i int32) *int32 { return &i }(3),
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
			require.NoError(t, p.Persist(ctx, eit.ControllerClient))

			require.NoError(t, retry.Wait(ctx, func(ctx context.Context) (bool, error) {
				if _, err := (lifecycle.RequiredLoader{Loader: p}).Load(ctx, eit.ControllerClient); err != nil {
					return true, err
				}

				if avail := p.Object.Status.AvailableReplicas; avail != 3 {
					return false, fmt.Errorf("scaling up: pool has %d replicas", avail)
				}

				return true, nil
			}))

			p.Object.Spec.Replicas = func(i int32) *int32 { return &i }(2)
			require.NoError(t, p.Persist(ctx, eit.ControllerClient))

			require.NoError(t, retry.Wait(ctx, func(ctx context.Context) (bool, error) {
				if _, err := (lifecycle.RequiredLoader{Loader: p}).Load(ctx, eit.ControllerClient); err != nil {
					return true, err
				}

				if avail := p.Object.Status.AvailableReplicas; avail != 2 {
					return false, fmt.Errorf("scaling down: pool has %d replicas", avail)
				}

				return true, nil
			}))
		})
	})
}
