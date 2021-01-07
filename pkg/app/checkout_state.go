package app

import (
	"context"

	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheckoutState struct {
	Checkout *obj.Checkout

	// PersistentVolume is the PV corresponding to the PVC owned by this
	// checkout. If this object exists, but not PersistentVolumeClaim, we merely
	// need to set up the PVC to point at this PV.
	PersistentVolume *corev1obj.PersistentVolume

	// PersistentVolumeClaim is the PVC owned by this checkout.
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim

	// PoolReplica points to the PVC we need to delete from the pool if we
	// haven't taken care of it already.
	PoolReplica *PoolReplica
}

var _ lifecycle.Loader = &CheckoutState{}
var _ lifecycle.Persister = &CheckoutState{}

func (cs *CheckoutState) loadFromVolumeName(ctx context.Context, cl client.Client, pool *obj.Pool) error {
	volumeName := cs.Checkout.Object.Status.VolumeName
	if volumeName == "" {
		return nil
	}

	volume := corev1obj.NewPersistentVolume(volumeName)
	if _, err := cs.PersistentVolume.Load(ctx, cl); err != nil {
		return err
	}

	// Check PV phase to determine which PVC we should load, if any.
	switch volume.Object.Status.Phase {
	case corev1.VolumeReleased:
		// If the PV phase is Released, its ClaimRef is invalid (because we
		// deleted it). Then we just need to hook up our own PVC to it.
		cs.PersistentVolume = volume
	case corev1.VolumeBound:
		// If the PV phase is Bound, it's either attached to our PVC or to
		// the pool's PVC. We'll now figure out which.
		claim := corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
			Namespace: volume.Object.Spec.ClaimRef.Namespace,
			Name:      volume.Object.Spec.ClaimRef.Name,
		})
		if _, err := claim.Load(ctx, cl); err != nil {
			return err
		}

		// It's possible the PVC was deleted from under us, in which case
		// the claim won't load. In this case we assume whatever stole it
		// knows what it's doing with the volume and pick a new one from the
		// pool.
		if ctrl := metav1.GetControllerOf(claim.Object); ctrl != nil {
			switch {
			case ctrl.UID == cs.Checkout.Object.GetUID():
				// This is our PVC.
				cs.PersistentVolume = volume
				cs.PersistentVolumeClaim = claim
			case ctrl.UID == pool.Object.GetUID():
				// This is a pool-owned PVC that we may need to delete.
				cs.PersistentVolume = volume

				pr := NewPoolReplica(pool, client.ObjectKeyFromObject(claim.Object))
				if ok, err := cs.PoolReplica.Load(ctx, cl); err != nil {
					return err
				} else if ok {
					cs.PoolReplica = pr
				}
			default:
				// The claim belongs to someone else, so again, we'll just
				// pick a new volume from the pool.
			}
		}
	default:
		// This generally should never happen and it means the volume
		// probably not valid anymore (maybe deleted from under us). We'll
		// assume we need to get a new volume then.
	}

	return nil
}

func (cs *CheckoutState) loadFromPool(ctx context.Context, cl client.Client, pool *obj.Pool) (bool, error) {
	if cs.PersistentVolume != nil {
		return true, nil
	}

	ps := NewPoolState(pool)
	if ok, err := ps.Load(ctx, cl); err != nil || !ok {
		return ok, err
	}

	rng, err := rand.DefaultFactory.New()
	if err != nil {
		return false, err
	}

	// Pick a new volume from the available pool.
	pr, found, err := ps.Available.Pop(rng)
	if err != nil {
		return false, err
	} else if !found {
		// No available PVCs!
		// XXX: ERROR
		return false, nil
	}

	cs.PoolReplica = pr
	cs.PersistentVolume = corev1obj.NewPersistentVolume(cs.PoolReplica.PersistentVolumeClaim.Object.Spec.VolumeName)
	if ok, err := cs.PersistentVolume.Load(ctx, cl); err != nil || !ok {
		return ok, err
	}

	return true, nil
}

func (cs *CheckoutState) Load(ctx context.Context, cl client.Client) (bool, error) {
	pool := obj.NewPool(client.ObjectKey{
		Namespace: cs.Checkout.Object.Spec.PoolRef.Namespace,
		Name:      cs.Checkout.Object.Spec.PoolRef.Name,
	})
	if _, err := (lifecycle.RequiredLoader{Loader: pool}).Load(ctx, cl); err != nil {
		return false, err
	}

	if err := cs.loadFromVolumeName(ctx, cl, pool); err != nil {
		return false, err
	}

	// If we did not end up setting the PV above, we need to request a new one
	// from the pool.
	if ok, err := cs.loadFromPool(ctx, cl, pool); err != nil || !ok {
		return ok, err
	}

	// In any case, if we didn't resolve a PVC, we'll make sure we're pointing
	// at one for persistence later.
	if cs.PersistentVolumeClaim == nil {
		cs.PersistentVolumeClaim = corev1obj.NewPersistentVolumeClaim(cs.Checkout.Key)

		// It could be that the pool changed part way through this process, so
		// we'll go ahead and try to load this so we don't get conflicts when we
		// try to save.
		if _, err := cs.PersistentVolumeClaim.Load(ctx, cl); err != nil {
			return false, nil
		}
	}

	return true, nil
}

func (cs *CheckoutState) Persist(ctx context.Context, cl client.Client) error {
	if err := cs.Checkout.PersistStatus(ctx, cl); err != nil {
		return err
	}

	if err := cs.PersistentVolume.Persist(ctx, cl); err != nil {
		return err
	}

	if cs.PoolReplica != nil {
		if _, err := cs.PoolReplica.Delete(ctx, cl); err != nil {
			return err
		}
	} else if err := cs.PersistentVolumeClaim.Persist(ctx, cl); err != nil {
		return err
	}

	return nil
}

func NewCheckoutState(c *obj.Checkout) *CheckoutState {
	return &CheckoutState{
		Checkout: c,
	}
}

func ConfigureCheckoutState(cs *CheckoutState) (*CheckoutState, error) {
	if cs.PersistentVolume == nil {
		// Something didn't go right when loading probably. Clear our state.
		cs.Checkout.Object.Status.VolumeName = ""
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
		return cs, nil
	}

	// Subscribe to our chosen volume so we get status updates as we attempt to
	// move through the binding process.
	if err := DependencyManager.SetDependencyOf(cs.PersistentVolume.Object, lifecycle.TypedObject{
		GVK:    obj.CheckoutKind,
		Object: cs.Checkout.Object,
	}); err != nil {
		return nil, err
	}

	cs.Checkout.Object.Status.VolumeName = cs.PersistentVolume.Name

	if cs.PoolReplica != nil {
		// We want to set up the volume to transition states to our PVC now. We
		// change the binding mode to Retain so we can later re-bind it.
		cs.PersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

		// The PVC is currently not available so make sure we zero it out.
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
	} else {
		// Now we can set up the PVC to point at our volume.
		cs.PersistentVolume.Object.Spec.ClaimRef = &corev1.ObjectReference{
			APIVersion: corev1obj.PersistentVolumeClaimKind.GroupVersion().String(),
			Kind:       corev1obj.PersistentVolumeClaimKind.Kind,
			Namespace:  cs.PersistentVolumeClaim.Key.Namespace,
			Name:       cs.PersistentVolumeClaim.Key.Name,
			UID:        cs.PersistentVolumeClaim.Object.GetUID(),
		}
		cs.PersistentVolumeClaim.Object.Spec = corev1.PersistentVolumeClaimSpec{
			VolumeName:  cs.PersistentVolume.Name,
			AccessModes: cs.Checkout.Object.Spec.AccessModes,
		}

		if cs.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound {
			cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{
				Name: cs.PersistentVolumeClaim.Key.Name,
			}
		}
	}

	return cs, nil
}

func ApplyCheckoutState(ctx context.Context, cl client.Client, c *obj.Checkout) (*CheckoutState, error) {
	cs := NewCheckoutState(c)

	if _, err := cs.Load(ctx, cl); err != nil {
		return nil, err
	}

	cs, err := ConfigureCheckoutState(cs)
	if err != nil {
		return nil, err
	}

	if err := cs.Persist(ctx, cl); err != nil {
		return nil, err
	}

	return cs, nil
}
