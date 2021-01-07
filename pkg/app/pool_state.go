package app

import (
	"context"

	"github.com/google/uuid"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/k8sutil/pkg/norm"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PoolState struct {
	Pool         *obj.Pool
	Initializing PoolReplicas
	Available    PoolReplicas
	Stale        PoolReplicas
}

var _ lifecycle.Loader = &PoolState{}
var _ lifecycle.Persister = &PoolState{}

func (ps *PoolState) Load(ctx context.Context, cl client.Client) (bool, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(&ps.Pool.Object.Spec.Selector)
	if err != nil {
		return false, err
	}

	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(
		ctx, pvcs,
		client.InNamespace(ps.Pool.Key.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector},
	); err != nil {
		return false, err
	}

	ps.Initializing = nil
	ps.Available = nil
	ps.Stale = nil
	for i := range pvcs.Items {
		pr := NewPoolReplica(ps.Pool, client.ObjectKeyFromObject(&pvcs.Items[i]))
		ok, err := pr.Load(ctx, cl)
		if err != nil {
			return false, err
		} else if !ok {
			// Lost from under us between list and get.
			continue
		}

		switch {
		case pr.Stale():
			ps.Stale = append(ps.Stale, pr)
		case pr.Available():
			ps.Available = append(ps.Available, pr)
		default:
			ps.Initializing = append(ps.Initializing, pr)
		}
	}

	return true, nil
}

func (ps *PoolState) Persist(ctx context.Context, cl client.Client) error {
	var replicas int32 = 1
	if n := ps.Pool.Object.Spec.Replicas; n != nil {
		replicas = *n
	}

	if replicas < 0 {
		replicas = 0
	}

	rng, err := rand.DefaultFactory.New()
	if err != nil {
		return err
	}

	if err := ps.Pool.PersistStatus(ctx, cl); err != nil {
		return err
	}

	wanted := replicas - int32(len(ps.Available)+len(ps.Initializing))
	if wanted < 0 {
		var i int32
		for i = wanted; i < 0; i++ {
			// First work through initializing replicas and delete them. If none
			// are available from the initializing list, use available instead.
			for _, prs := range []*PoolReplicas{&ps.Initializing, &ps.Available} {
				pr, found, err := prs.Pop(rng)
				if err != nil {
					return err
				} else if !found {
					continue
				}

				if _, err := pr.Delete(ctx, cl); err != nil {
					// Add back into the list since we couldn't delete it.
					*prs = append(*prs, pr)
					return err
				}
				break
			}
		}
	} else if wanted > 0 {
		var i int32
		for i = 0; i < wanted; i++ {
			key := client.ObjectKey{
				Namespace: ps.Pool.Key.Namespace,
				Name:      norm.MetaNameSuffixed(ps.Pool.Key.Name, "-"+uuid.New().String()),
			}

			pr := ConfigurePoolReplica(NewPoolReplica(ps.Pool, key))
			if err := pr.Persist(ctx, cl); err != nil {
				return err
			}

			ps.Initializing = append(ps.Initializing, pr)
		}
	}

	// Finally delete any stale replicas.
	for {
		pr, found, err := ps.Stale.Pop(rng)
		if err != nil {
			return err
		} else if !found {
			break
		}

		if _, err := pr.Delete(ctx, cl); err != nil {
			// Add back into the list since we couldn't delete it.
			ps.Stale = append(ps.Stale, pr)
			return err
		}
	}

	return nil
}

func NewPoolState(p *obj.Pool) *PoolState {
	return &PoolState{
		Pool: p,
	}
}

func ConfigurePoolState(ps *PoolState) *PoolState {
	ps.Pool.Object.Status.ObservedGeneration = ps.Pool.Object.GetGeneration()
	ps.Pool.Object.Status.Replicas = int32(len(ps.Available) + len(ps.Initializing) + len(ps.Stale))
	ps.Pool.Object.Status.AvailableReplicas = int32(len(ps.Available))
	return ps
}

func ApplyPoolState(ctx context.Context, cl client.Client, p *obj.Pool) (*PoolState, error) {
	ps := NewPoolState(p)

	if _, err := ps.Load(ctx, cl); err != nil {
		return nil, err
	}

	ps = ConfigurePoolState(ps)

	if err := ps.Persist(ctx, cl); err != nil {
		return nil, err
	}

	return ps, nil
}
