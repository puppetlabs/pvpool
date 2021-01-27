package app

import (
	"context"
	"fmt"

	"github.com/puppetlabs/leg/errmap/pkg/errmark"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CheckoutState struct {
	Checkout              *obj.Checkout
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim

	// PersistentVolume is the PV corresponding to the PVC owned by this
	// checkout. If this object exists, but not PersistentVolumeClaim, we merely
	// need to set up the PVC to point at this PV.
	PersistentVolume *corev1obj.PersistentVolume
}

var _ lifecycle.Loader = &CheckoutState{}
var _ lifecycle.Persister = &CheckoutState{}

func (cs *CheckoutState) loadFromVolumeName(ctx context.Context, cl client.Client) error {
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

	if ctrl := metav1.GetControllerOf(claim.Object); ctrl != nil {
		switch {
		case ctrl.UID == cs.Checkout.Object.GetUID():
			klog.V(4).InfoS("checkout state: load: persistent volume is used by this checkout", "checkout", cs.Checkout.Key, "pv", volume.Name)
		case schema.FromAPIVersionAndKind(ctrl.APIVersion, ctrl.Kind) == obj.CheckoutKind:
			// Race condition where the claim has been reassigned from under us.
			klog.InfoS("checkout state: load: persistent volume stolen (not using)", "checkout", cs.Checkout.Key, "pv", volume.Name)
			return nil
		default:
			klog.V(4).InfoS("checkout state: load: persistent volume is still in pool", "checkout", cs.Checkout.Key, "pv", volume.Name)
		}
	}

	cs.PersistentVolume = volume
	return nil
}

func (cs *CheckoutState) loadFromPool(ctx context.Context, cl client.Client) (bool, error) {
	if cs.PersistentVolume != nil {
		return true, nil
	}

	namespace := cs.Checkout.Object.Spec.PoolRef.Namespace
	if namespace == "" {
		namespace = cs.Checkout.Key.Namespace
	}

	pool := obj.NewPool(client.ObjectKey{
		Namespace: namespace,
		Name:      cs.Checkout.Object.Spec.PoolRef.Name,
	})
	if _, err := (lifecycle.RequiredLoader{Loader: pool}).Load(ctx, cl); err != nil {
		eventctx.EventRecorder(ctx).Eventf(cs.Checkout.Object, "Warning", "PoolAvailability", "Pool %s does not exist", pool.Key)

		return false, err
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
		eventctx.EventRecorder(ctx).Event(cs.Checkout.Object, "Warning", "PoolAvailability", "Pool has no available PVCs to check out")

		klog.InfoS("checkout state: load: pool has no available PVCs", "checkout", cs.Checkout.Key, "pool", pool.Key)
		return false, errmark.MarkTransient(fmt.Errorf("pool %s has no available PVCs", pool.Key))
	}

	klog.V(4).InfoS("checkout state: load: using PVC from pool", "checkout", cs.Checkout.Key, "pool", pool.Key, "pvc", pr.PersistentVolumeClaim.Key, "pv", pr.PersistentVolume.Name)

	cs.PersistentVolume = pr.PersistentVolume

	return true, nil
}

func (cs *CheckoutState) Load(ctx context.Context, cl client.Client) (bool, error) {
	if _, err := cs.PersistentVolumeClaim.Load(ctx, cl); err != nil {
		return false, nil
	}

	if err := cs.loadFromVolumeName(ctx, cl); err != nil {
		return false, err
	}

	// If we did not end up setting the PV above, we need to request a new one
	// from the pool.
	if ok, err := cs.loadFromPool(ctx, cl); err != nil || !ok {
		return ok, err
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

	if err := cs.PersistentVolumeClaim.Persist(ctx, cl); errors.IsInvalid(err) {
		return errmark.MarkUser(err)
	} else if err != nil {
		return err
	}

	if cs.PersistentVolume == nil {
		eventctx.EventRecorder(ctx).Event(cs.Checkout.Object, "Warning", "VolumeAttachment", "No volume found")

		return errmark.MarkTransient(fmt.Errorf("missing persistent volume"))
	}

	// Sync UID to ClaimRef.
	cs.PersistentVolume.Object.Spec.ClaimRef.UID = cs.PersistentVolumeClaim.Object.GetUID()

	return cs.PersistentVolume.Persist(ctx, cl)
}

func NewCheckoutState(c *obj.Checkout) *CheckoutState {
	return &CheckoutState{
		Checkout:              c,
		PersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(c.Key),
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
