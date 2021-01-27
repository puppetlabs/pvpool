package app

import (
	"context"
	"fmt"

	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
}

var _ lifecycle.Loader = &CheckoutState{}
var _ lifecycle.Persister = &CheckoutState{}

func (cs *CheckoutState) loadFromVolumeName(ctx context.Context, cl client.Client, pool *obj.Pool) error {
	volumeName := cs.Checkout.Object.Status.VolumeName
	if volumeName == "" {
		return nil
	}

	klog.V(4).InfoS("checkout state: load: loading from persistent volume name", "checkout", cs.Checkout.Key, "pv", volumeName)

	volume := corev1obj.NewPersistentVolume(volumeName)
	if _, err := volume.Load(ctx, cl); err != nil {
		return err
	}

	// Let's see which PVC this PV points at.
	claim := corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
		Namespace: volume.Object.Spec.ClaimRef.Namespace,
		Name:      volume.Object.Spec.ClaimRef.Name,
	})
	if _, err := claim.Load(ctx, cl); err != nil {
		return err
	}

	// It's possible the PVC was deleted from under us, in which case the claim
	// won't load. In this case we assume whatever stole it knows what it's
	// doing with the volume and pick a new one from the pool.
	if ctrl := metav1.GetControllerOf(claim.Object); ctrl != nil {
		switch {
		case ctrl.UID == cs.Checkout.Object.GetUID():
			// This is our PVC.
			klog.V(4).InfoS("checkout state: load: persistent volume is used by this checkout", "checkout", cs.Checkout.Key, "pv", volume.Name)
			cs.PersistentVolume = volume
			cs.PersistentVolumeClaim = claim
		case ctrl.UID == pool.Object.GetUID():
			klog.V(4).InfoS("checkout state: load: persistent volume is still in pool", "checkout", cs.Checkout.Key, "pv", volume.Name)
			pr := NewPoolReplica(pool, client.ObjectKeyFromObject(claim.Object))
			if ok, err := pr.Load(ctx, cl); err != nil {
				return err
			} else if ok {
				cs.PersistentVolume = pr.PersistentVolume
			}
		default:
			// The claim belongs to someone else, so again, we'll just pick a
			// new volume from the pool.
			klog.InfoS("checkout state: load: persistent volume stolen (not using)", "checkout", cs.Checkout.Key, "pv", volume.Name)
		}
	}

	return nil
}

func (cs *CheckoutState) loadFromPool(ctx context.Context, cl client.Client, pool *obj.Pool) (bool, error) {
	if cs.PersistentVolume != nil {
		return true, nil
	}

	klog.V(4).InfoS("checkout state: load: loading from pool", "checkout", cs.Checkout.Key, "pool", pool.Key)

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
		klog.InfoS("checkout state: load: pool has no available PVCs", "checkout", cs.Checkout.Key, "pool", pool.Key)
		// No available PVCs!
		// XXX: ERROR
		return false, nil
	}

	klog.V(4).InfoS("checkout state: load: using PVC from pool", "checkout", cs.Checkout.Key, "pool", pool.Key, "pvc", pr.PersistentVolumeClaim.Key, "pv", pr.PersistentVolume.Name)

	cs.PersistentVolume = pr.PersistentVolume

	return true, nil
}

func (cs *CheckoutState) Load(ctx context.Context, cl client.Client) (bool, error) {
	namespace := cs.Checkout.Object.Spec.PoolRef.Namespace
	if namespace == "" {
		namespace = cs.Checkout.Key.Namespace
	}

	pool := obj.NewPool(client.ObjectKey{
		Namespace: namespace,
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
		klog.V(4).InfoS("checkout state: load: no owned PVC found", "checkout", cs.Checkout.Key)
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

	if err := cs.Checkout.Own(ctx, cs.PersistentVolumeClaim); err != nil {
		return err
	}

	if err := cs.PersistentVolumeClaim.Persist(ctx, cl); err != nil {
		return err
	}

	if cs.PersistentVolume == nil {
		return fmt.Errorf("XXX REQUEUE")
	} else {
		// Sync UID to ClaimRef.
		cs.PersistentVolume.Object.Spec.ClaimRef.UID = cs.PersistentVolumeClaim.Object.GetUID()

		if err := cs.PersistentVolume.Persist(ctx, cl); err != nil {
			return err
		}
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
		klog.V(4).InfoS("checkout state: configure: no volume, clearing status", "checkout", cs.Checkout.Key)

		// Something didn't go right when loading probably. Clear our state.
		cs.Checkout.Object.Status.VolumeName = ""
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
		return cs, nil
	}

	cs.Checkout.Object.Status.VolumeName = cs.PersistentVolume.Name

	// Set up the PV to point at our claim.
	cs.PersistentVolume.Object.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: corev1obj.PersistentVolumeClaimKind.GroupVersion().String(),
		Kind:       corev1obj.PersistentVolumeClaimKind.Kind,
		Namespace:  cs.PersistentVolumeClaim.Key.Namespace,
		Name:       cs.PersistentVolumeClaim.Key.Name,
		UID:        cs.PersistentVolumeClaim.Object.GetUID(),
	}
	cs.PersistentVolumeClaim.Object.Spec.VolumeName = cs.PersistentVolume.Name
	cs.PersistentVolumeClaim.Object.Spec.AccessModes = cs.Checkout.Object.Spec.AccessModes
	cs.PersistentVolumeClaim.Object.Spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: cs.PersistentVolume.Object.Spec.Capacity.Storage().DeepCopy(),
		},
	}

	if cs.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound {
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{
			Name: cs.PersistentVolumeClaim.Key.Name,
		}
	} else {
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
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
