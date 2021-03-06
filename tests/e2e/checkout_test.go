package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	rbacv1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/rbacv1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCheckout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			_ = eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, client.ObjectKey{Name: poolKey.Name})
			_ = eit.PoolHelpers.RequireWaitSettled(ctx, p)
		})
	})
}

func TestCheckoutAcrossNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns1 *corev1.Namespace) {
			eit.WithNamespace(ctx, func(ns2 *corev1.Namespace) {
				poolKey := client.ObjectKey{
					Namespace: ns1.GetName(),
					Name:      "test-pool",
				}
				checkoutKey := client.ObjectKey{
					Namespace: ns2.GetName(),
					Name:      "test-checkout",
				}
				p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
				_ = eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey)
				_ = eit.PoolHelpers.RequireWaitSettled(ctx, p)
			})
		})
	})
}

func TestCheckoutsPoolAvailability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			for i := 1; i <= 5; i++ {
				checkoutKey := client.ObjectKey{
					Namespace: ns.GetName(),
					Name:      fmt.Sprintf("test-checkout-%d", i),
				}
				_ = eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey)
			}
			_ = eit.PoolHelpers.RequireWaitSettled(ctx, p)
		})
	})
}

func TestCheckoutWithInitJob(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tpl := pvpoolv1alpha1.MountJob{
		Template: pvpoolv1alpha1.JobTemplate{
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "init",
								Image: "busybox:stable-musl",
								Command: []string{
									"/bin/sh",
									"-c",
									"echo test-value >/workspace/foo",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace",
										MountPath: "/workspace",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithInitJob(tpl))
			co := eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey)

			// Create a pod that uses the PVC.
			pod := corev1obj.NewPod(client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout-ref",
			})
			pod.Object.Spec = corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "read",
						Image: "busybox:stable-musl",
						Command: []string{
							"cat", "/workspace/foo",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test",
								MountPath: "/workspace",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "test",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: co.Object.Status.VolumeClaimRef.Name,
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			}
			require.NoError(t, pod.Persist(ctx, eit.ControllerClient))

			ok, err := corev1obj.NewPodTerminatedPoller(pod).Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, corev1.PodSucceeded, pod.Object.Status.Phase)

			// Make sure logs contain the test value.
			logs, err := eit.StaticClient.CoreV1().Pods(pod.Key.Namespace).
				GetLogs(pod.Key.Name, &corev1.PodLogOptions{Container: "read"}).
				Stream(ctx)
			require.NoError(t, err)
			defer logs.Close()

			var buf bytes.Buffer
			_, err = io.Copy(&buf, logs)
			require.NoError(t, err)
			require.Equal(t, "test-value\n", buf.String())
		})
	})
}

func TestCheckoutBeforePoolCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			co := eit.CheckoutHelpers.RequireCreateCheckout(ctx, checkoutKey, poolKey)
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			_ = eit.CheckoutHelpers.RequireWaitCheckedOut(ctx, co)
		})
	})
}

func TestCheckoutBeforePoolHasReplicas(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			p := eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(0))
			co := eit.CheckoutHelpers.RequireCreateCheckout(ctx, checkoutKey, poolKey)
			p = eit.PoolHelpers.RequireScalePoolThenWaitSettled(ctx, p, 3)
			_ = eit.CheckoutHelpers.RequireWaitCheckedOut(ctx, co)
			_ = eit.PoolHelpers.RequireWaitSettled(ctx, p)
		})
	})
}

func TestCheckoutClaimName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			pvcKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pvc",
			}
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			_ = eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey, WithClaimName(pvcKey.Name))

			// Now this PVC should exist.
			pvc := corev1obj.NewPersistentVolumeClaim(pvcKey)
			ok, err := pvc.Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)
		})
	})
}

func TestCheckoutClaimInUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			pvcKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pvc",
			}

			// Create a PVC with the name we want.
			pvc := corev1obj.NewPersistentVolumeClaim(pvcKey)
			pvc.Object.Spec = corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: pointer.StringPtr(eit.StorageClassName),
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Mi"),
					},
				},
			}
			require.NoError(t, pvc.Persist(ctx, eit.ControllerClient))

			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			co := eit.CheckoutHelpers.RequireCreateCheckout(ctx, checkoutKey, poolKey, WithClaimName(pvcKey.Name))

			// The checkout should move to a conflict state.
			require.NoError(t, Wait(ctx, func(ctx context.Context) (bool, error) {
				if _, err := (lifecycle.RequiredLoader{Loader: co}).Load(ctx, eit.ControllerClient); err != nil {
					return true, err
				}

				cond, _ := co.Condition(pvpoolv1alpha1.CheckoutAcquired)
				switch cond.Status {
				case corev1.ConditionTrue:
					return true, fmt.Errorf("volume checked out over existing PVC")
				case corev1.ConditionUnknown:
					if cond.Reason == pvpoolv1alpha1.CheckoutAcquiredReasonConflict {
						return true, nil
					}
					fallthrough
				default:
					return false, fmt.Errorf("waiting for checkout to reconcile")
				}
			}))

			// Now, if we delete the PVC, the checkout should progress.
			require.NoError(t, Wait(ctx, func(ctx context.Context) (bool, error) {
				if _, err := (lifecycle.RequiredLoader{Loader: pvc}).Load(ctx, eit.ControllerClient); err != nil {
					return true, err
				}

				ok, err := pvc.Delete(ctx, eit.ControllerClient)
				if errors.IsConflict(err) {
					return false, err
				} else if err != nil {
					return true, err
				} else if !ok {
					return true, fmt.Errorf("PVC spuriously deleted")
				}

				return true, nil
			}))
			_ = eit.CheckoutHelpers.RequireWaitCheckedOut(ctx, co)
		})
	})
}

func TestCheckoutAccessModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}

			// Create pool in RWO.
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3), WithAccessModes{corev1.ReadWriteOnce})

			// Create checkout in ROX. The volume should transition to the
			// correct access mode.
			co := eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey, WithAccessModes{corev1.ReadOnlyMany})

			// Get the corresponding PVC and check its access mode.
			pvc := corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
				Namespace: co.Object.GetNamespace(),
				Name:      co.Object.Status.VolumeClaimRef.Name,
			})
			ok, err := pvc.Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, pvc.Object.Spec.AccessModes)
			require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, pvc.Object.Status.AccessModes)
		})
	})
}

func TestCheckoutPVCReplacement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))
			co := eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, poolKey)

			// Delete the underlying PVC.
			pvc := corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
				Namespace: co.Object.GetNamespace(),
				Name:      co.Object.Status.VolumeClaimRef.Name,
			})
			ok, err := pvc.Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)

			prevUID := pvc.Object.GetUID()
			require.NotEmpty(t, prevUID)

			prevVolumeName := pvc.Object.Spec.VolumeName
			require.NotEmpty(t, prevVolumeName)

			ok, err = pvc.Delete(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)

			// Wait for a new PVC to be populated by the checkout controller.
			require.NoError(t, Wait(ctx, func(ctx context.Context) (bool, error) {
				ok, err := pvc.Load(ctx, eit.ControllerClient)
				if err != nil || !ok {
					return ok, err
				}

				if prevUID == pvc.Object.GetUID() {
					return false, fmt.Errorf("waiting for PVC deletion")
				}

				if pvc.Object.Status.Phase != corev1.ClaimBound {
					return false, fmt.Errorf("waiting for PVC to bind, current phase is %s", pvc.Object.Status.Phase)
				}

				return true, nil
			}))

			require.NotEmpty(t, pvc.Object.Spec.VolumeName)
			require.NotEqual(t, prevVolumeName, pvc.Object.Spec.VolumeName)
		})
	})
}

func TestCheckoutMutability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey1 := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool-1",
			}
			poolKey2 := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool-2",
			}
			checkoutKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout",
			}
			p1 := eit.PoolHelpers.RequireCreatePool(ctx, poolKey1, WithReplicas(3))
			p2 := eit.PoolHelpers.RequireCreatePool(ctx, poolKey2, WithReplicas(3))
			p1 = eit.PoolHelpers.RequireWaitSettled(ctx, p1)
			co := eit.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, checkoutKey, p1.Key)

			// Attempt to change the pool. This field should be immutable.
			co.Object.Spec.PoolRef = pvpoolv1alpha1.PoolReference{
				Namespace: p2.Key.Namespace,
				Name:      p2.Key.Name,
			}
			err := co.Persist(ctx, eit.ControllerClient)
			require.True(t, errors.IsInvalid(err))

			// Now we'll scale down the first pool and delete the checkout's
			// PVC. This should let us change the pool again.
			_ = eit.PoolHelpers.RequireScalePoolThenWaitSettled(ctx, p1, 0)

			pvc := corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
				Namespace: co.Object.GetNamespace(),
				Name:      co.Object.Status.VolumeClaimRef.Name,
			})
			ok, err := pvc.Load(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = pvc.Delete(ctx, eit.ControllerClient)
			require.NoError(t, err)
			require.True(t, ok)

			// Wait for the checkout to reconcile.
			require.NoError(t, Wait(ctx, func(ctx context.Context) (bool, error) {
				ok, err := co.Load(ctx, eit.ControllerClient)
				if err != nil || !ok {
					return ok, err
				}

				if cond, _ := co.Condition(pvpoolv1alpha1.CheckoutAcquired); cond.Status == corev1.ConditionTrue {
					return false, fmt.Errorf("waiting for checkout to reconcile")
				}

				return true, nil
			}))

			// Update the pool and then it should check out again successfully.
			co.Object.Spec.PoolRef = pvpoolv1alpha1.PoolReference{
				Namespace: p2.Key.Namespace,
				Name:      p2.Key.Name,
			}
			require.NoError(t, co.Persist(ctx, eit.ControllerClient))
			_ = eit.CheckoutHelpers.RequireWaitCheckedOut(ctx, co)
		})
	})
}

func TestCheckoutRBAC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	WithEnvironmentInTest(t, func(eit *EnvironmentInTest) {
		eit.WithNamespace(ctx, func(ns *corev1.Namespace) {
			poolKey := client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-pool",
			}
			_ = eit.PoolHelpers.RequireCreatePoolThenWaitSettled(ctx, poolKey, WithReplicas(3))

			// Create a service account and set up impersonation of it.
			sa := corev1obj.NewServiceAccount(client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-sa",
			})
			require.NoError(t, sa.Persist(ctx, eit.ControllerClient))

			actor := eit.Impersonate(rest.ImpersonationConfig{
				UserName: fmt.Sprintf("system:serviceaccount:%s:%s", sa.Key.Namespace, sa.Key.Name),
			})

			// Set up a role and role binding.
			checkoutGVR, err := eit.RESTMapper.RESTMapping(pvpoolv1alpha1.CheckoutKind.GroupKind())
			require.NoError(t, err)

			role := rbacv1obj.NewRole(sa.Key)
			role.Object.Rules = []rbacv1.PolicyRule{
				{
					APIGroups: []string{checkoutGVR.Resource.Group},
					Resources: []string{checkoutGVR.Resource.Resource},
					Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
				},
			}
			require.NoError(t, role.Persist(ctx, eit.ControllerClient))

			rb := rbacv1obj.NewRoleBinding(sa.Key)
			rb.Object.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     role.Key.Name,
			}
			rb.Object.Subjects = []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: sa.Key.Name,
				},
			}
			require.NoError(t, rb.Persist(ctx, eit.ControllerClient))

			// Creating the checkout should fail because we haven't assigned the
			// "use" permission for the pool.
			_, err = actor.CheckoutHelpers.CreateCheckoutThenWaitCheckedOut(ctx, client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout-forbidden",
			}, poolKey)
			require.True(t, errors.IsForbidden(err))

			// Update the role to also include the relevant "use" permission.
			poolGVR, err := eit.RESTMapper.RESTMapping(pvpoolv1alpha1.PoolKind.GroupKind())
			require.NoError(t, err)

			role.Object.Rules = append(role.Object.Rules, rbacv1.PolicyRule{
				APIGroups:     []string{poolGVR.Resource.Group},
				Resources:     []string{poolGVR.Resource.Resource},
				Verbs:         []string{"use"},
				ResourceNames: []string{poolKey.Name},
			})
			require.NoError(t, role.Persist(ctx, eit.ControllerClient))

			// Creating the checkout should now succeed.
			_ = actor.CheckoutHelpers.RequireCreateCheckoutThenWaitCheckedOut(ctx, client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout-ok",
			}, poolKey)

			// Creating a checkout with a different pool name should still fail.
			_, err = actor.CheckoutHelpers.CreateCheckoutThenWaitCheckedOut(ctx, client.ObjectKey{
				Namespace: ns.GetName(),
				Name:      "test-checkout-forbidden-pool-name",
			}, client.ObjectKey{Name: "forbidden-pool"})
			require.True(t, errors.IsForbidden(err))
		})
	})
}
