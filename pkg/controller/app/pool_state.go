package app

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/puppetlabs/leg/errmap/pkg/errmark"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PoolState struct {
	Pool         *obj.Pool
	Initializing PoolReplicas
	Available    PoolReplicas
	Stale        PoolReplicas
}

var _ lifecycle.Deleter = &PoolState{}
var _ lifecycle.Loader = &PoolState{}
var _ lifecycle.Persister = &PoolState{}

func (ps *PoolState) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	rng, err := rand.DefaultFactory.New()
	if err != nil {
		return false, err
	}

	for _, prs := range []*PoolReplicas{&ps.Initializing, &ps.Available, &ps.Stale} {
		for {
			pr, found, err := prs.Pop(rng)
			if err != nil {
				return false, err
			} else if !found {
				break
			}

			if _, err := pr.Delete(ctx, cl, opts...); err != nil {
				// Add back into the list since we couldn't delete it.
				*prs = append(*prs, pr)
				return false, err
			}
		}
	}

	return true, nil
}

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

func (ps *PoolState) persistInitializing(ctx context.Context, cl client.Client) error {
	for i := 0; i < len(ps.Initializing); {
		if err := ps.Initializing[i].Persist(ctx, cl); err != nil {
			return err
		}

		// Move initializing PVCs to available if possible.
		if ps.Initializing[i].Available() {
			ps.Available = append(ps.Available, ps.Initializing[i])
			ps.Initializing[i] = ps.Initializing[len(ps.Initializing)-1]
			ps.Initializing = ps.Initializing[:len(ps.Initializing)-1]
		} else {
			i++
		}
	}

	return nil
}

func (ps *PoolState) persistAvailable(ctx context.Context, cl client.Client) error {
	for _, pr := range ps.Available {
		if err := pr.Persist(ctx, cl); err != nil {
			return err
		}
	}

	return nil
}

func (ps *PoolState) persistStale(ctx context.Context, cl client.Client) error {
	rng, err := rand.DefaultFactory.New()
	if err != nil {
		return err
	}

	for {
		pr, found, err := ps.Stale.Pop(rng)
		if err != nil {
			return err
		} else if !found {
			break
		}

		klog.InfoS("pool state: removing stale replica", "pool", ps.Pool.Key, "key", pr.PersistentVolumeClaim.Key)

		if fc, ok := pr.InitJob.FailedCondition(); ok && fc.Status == corev1.ConditionTrue {
			eventctx.EventRecorder(ctx).Eventf(ps.Pool.Object, "Warning", "StaleReplica", "Deleting stale replica with failed init job: %s: %s", fc.Reason, fc.Message)
		}

		if _, err := pr.Delete(ctx, cl); err != nil {
			// Add back into the list since we couldn't delete it.
			ps.Stale = append(ps.Stale, pr)
			return err
		}
	}

	return nil
}

func (ps *PoolState) persistScaleUp(ctx context.Context, cl client.Client) error {
	klog.InfoS("pool state: adding a PVC to meet replica request", "pool", ps.Pool.Key)

	id := uuid.New()
	pr, err := ApplyPoolReplica(ctx, cl, ps.Pool, hex.EncodeToString(id[:]))
	if errors.IsInvalid(err) {
		return errmark.MarkUser(err)
	} else if err != nil {
		return err
	}

	switch {
	case pr.Stale():
		ps.Stale = append(ps.Stale, pr)
	case pr.Available():
		ps.Available = append(ps.Available, pr)
	default:
		ps.Initializing = append(ps.Initializing, pr)
	}

	return nil
}

func (ps *PoolState) persistScaleDown(ctx context.Context, cl client.Client) error {
	rng, err := rand.DefaultFactory.New()
	if err != nil {
		return err
	}

	klog.InfoS("pool state: removing a PVC to meet replica request", "pool", ps.Pool.Key)

	// First work through initializing replicas and delete them. If none are
	// available from the initializing list, use available instead.
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

	return nil
}

func (ps *PoolState) persistScale(ctx context.Context, cl client.Client) error {
	var request int32 = 1
	if n := ps.Pool.Object.Spec.Replicas; n != nil {
		request = *n
	}

	actual := int32(len(ps.Available) + len(ps.Initializing))
	klog.V(4).InfoS("pool state: scale assessed", "pool", ps.Pool.Key, "request", request, "actual", actual)

	switch {
	case actual < request:
		eventctx.EventRecorder(ctx).Eventf(ps.Pool.Object, "Normal", "PoolScaling", "Scaling pool up to %d replicas", request)
		return ps.persistScaleUp(ctx, cl)
	case actual > request:
		eventctx.EventRecorder(ctx).Eventf(ps.Pool.Object, "Normal", "PoolScaling", "Scaling pool down to %d replicas", request)
		return ps.persistScaleDown(ctx, cl)
	default:
		return nil
	}
}

func (ps *PoolState) Persist(ctx context.Context, cl client.Client) error {
	if err := ps.Pool.PersistStatus(ctx, cl); err != nil {
		return err
	}

	if err := ps.persistInitializing(ctx, cl); err != nil {
		return err
	}

	if err := ps.persistAvailable(ctx, cl); err != nil {
		return err
	}

	if err := ps.persistScale(ctx, cl); err != nil {
		return err
	}

	if err := ps.persistStale(ctx, cl); err != nil {
		return err
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

	// See if any initializing PVCs need to be moved.
	for i := range ps.Initializing {
		ps.Initializing[i] = ConfigurePoolReplica(ps.Initializing[i])
	}

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
