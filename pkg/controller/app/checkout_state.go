package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/puppetlabs/leg/errmap/pkg/errmark"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
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

	// Conds represent status updates for given conditions.
	Conds map[pvpoolv1alpha1.CheckoutConditionType]pvpoolv1alpha1.Condition
}

var _ lifecycle.Loader = &CheckoutState{}
var _ lifecycle.Persister = &CheckoutState{}

func (cs *CheckoutState) loadFromPVC(ctx context.Context, cl client.Client) error {
	klog.V(4).InfoS("checkout state: load: loading from owned persistent volume claim", "checkout", cs.Checkout.Key, "pvc", cs.PersistentVolumeClaim.Key, "pv", cs.PersistentVolumeClaim.Object.Spec.VolumeName)

	volume := corev1obj.NewPersistentVolume(cs.PersistentVolumeClaim.Object.Spec.VolumeName)
	if ok, err := volume.Load(ctx, cl); err != nil {
		return err
	} else if !ok || volume.Object.Spec.ClaimRef == nil { // Not bound?
		klog.InfoS("checkout state: load: persistent volume missing or not bound (not using)", "checkout", cs.Checkout.Key, "pvc", cs.PersistentVolumeClaim.Key, "pv", volume.Name)
		return nil
	}

	cs.PersistentVolume = volume
	return nil
}

func (cs *CheckoutState) loadFromVolumeName(ctx context.Context, cl client.Client) error {
	volumeName := cs.Checkout.Object.Status.VolumeName
	if volumeName == "" {
		return nil
	}

	klog.V(4).InfoS("checkout state: load: loading from persistent volume name", "checkout", cs.Checkout.Key, "pv", volumeName)

	volume := corev1obj.NewPersistentVolume(volumeName)
	if ok, err := volume.Load(ctx, cl); err != nil {
		return err
	} else if !ok || volume.Object.Spec.ClaimRef == nil { // Not bound?
		klog.InfoS("checkout state: load: persistent volume missing or not bound (not using)", "checkout", cs.Checkout.Key, "pv", volume.Name)
		return nil
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
		cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
			Status:  corev1.ConditionUnknown,
			Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonPoolDoesNotExist,
			Message: fmt.Sprintf("The pool %q does not exist.", pool.Key),
		}

		return false, err
	}

	klog.V(4).InfoS("checkout state: load: loading from pool", "checkout", cs.Checkout.Key, "pool", pool.Key)

	ps := NewPoolState(pool)
	if ok, err := ps.Load(ctx, cl); err != nil || !ok {
		return ok, err
	}

	// Pick a new volume from the available pool. We always pick the oldest
	// available PVC.
	if len(ps.Available) == 0 {
		eventctx.EventRecorder(ctx).Event(cs.Checkout.Object, "Warning", "PoolAvailability", "Pool has no available PVCs to check out")
		cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
			Status:  corev1.ConditionUnknown,
			Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonNotAvailable,
			Message: fmt.Sprintf("The pool %q has no available PVCs to check out.", pool.Key),
		}

		klog.InfoS("checkout state: load: pool has no available PVCs", "checkout", cs.Checkout.Key, "pool", pool.Key)
		return false, errmark.MarkTransient(fmt.Errorf("pool %s has no available PVCs", pool.Key))
	} else {
		sort.Sort(PoolReplicasSortByCreationTimestamp(ps.Available))
		pr := ps.Available[0]

		klog.V(4).InfoS("checkout state: load: using PVC from pool", "checkout", cs.Checkout.Key, "pool", pool.Key, "pvc", pr.PersistentVolumeClaim.Key, "pv", pr.PersistentVolume.Name)
		cs.PersistentVolume = pr.PersistentVolume
	}

	return true, nil
}

func (cs *CheckoutState) Load(ctx context.Context, cl client.Client) (bool, error) {
	if _, err := cs.PersistentVolumeClaim.Load(ctx, cl); err != nil {
		return false, nil
	}

	switch cs.PersistentVolumeClaim.Object.Status.Phase {
	case corev1.ClaimPending, corev1.ClaimBound:
		if err := cs.loadFromPVC(ctx, cl); err != nil {
			return false, err
		}
	default:
		// Either this object doesn't yet exist or the claim was lost for some
		// reason.
		if err := cs.loadFromVolumeName(ctx, cl); err != nil {
			return false, err
		}
	}

	// If we did not end up setting the PV above, we need to request a new one
	// from the pool.
	if ok, err := cs.loadFromPool(ctx, cl); err != nil || !ok {
		return ok, err
	}

	return true, nil
}

func (cs *CheckoutState) Persist(ctx context.Context, cl client.Client) error {
	if err := cs.Checkout.Own(ctx, cs.PersistentVolumeClaim); err != nil {
		return err
	}

	if err := cs.PersistentVolumeClaim.Persist(ctx, cl); errors.IsInvalid(err) {
		cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
			Status:  corev1.ConditionFalse,
			Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonInvalid,
			Message: fmt.Sprintf("The PVC could not be created because of configuration problems: %v", err),
		}
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

	if err := cs.PersistentVolume.Persist(ctx, cl); err != nil {
		return err
	}

	if cs.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound {
		cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
			Status:  corev1.ConditionTrue,
			Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonCheckedOut,
			Message: "The PVC is ready to use.",
		}
	}

	return nil
}

func NewCheckoutState(c *obj.Checkout) *CheckoutState {
	return &CheckoutState{
		Checkout:              c,
		PersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(c.Key),
		Conds:                 make(map[pvpoolv1alpha1.CheckoutConditionType]pvpoolv1alpha1.Condition),
	}
}

func ConfigureCheckoutState(cs *CheckoutState) (*CheckoutState, error) {
	if cs.PersistentVolume == nil {
		klog.V(4).InfoS("checkout state: configure: no volume, clearing status", "checkout", cs.Checkout.Key)
		return cs, nil
	}

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

	return cs, nil
}
