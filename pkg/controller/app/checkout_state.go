package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/puppetlabs/leg/errmap/pkg/errmark"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/k8sutil/pkg/norm"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	pvpoolv1alpha1obj "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1/obj"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CheckoutReclaimPolicyAnnotationKey = "pvpool.puppet.com/checkout.reclaim-policy"
)

type CheckoutState struct {
	Checkout *pvpoolv1alpha1obj.Checkout

	// PersistentVolumeClaim is the final, settled PVC that should be made
	// available to the requestor.
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim

	// PersistentVolume is the PV corresponding to the PVC owned by this
	// checkout. If this object exists, but not PersistentVolumeClaim, we merely
	// need to set up the PVC to point at this PV.
	PersistentVolume *corev1obj.PersistentVolume

	// LockedPersistentVolumeClaim is a PVC that we use to take a PV from the
	// pool for this checkout. This PVC is necessary if we need to perform
	// modifications to the underlying PV before we can present it to the
	// requestor (for example, setting fields on the volume source).
	//
	// Using this intermediate means the somewhat complicated logic to detect
	// whether a pool replica has become stale can be left alone while we mess
	// with the PV here instead.
	LockedPersistentVolumeClaim *corev1obj.PersistentVolumeClaim

	// LockedPersistentVolume is the PV corresponding to the locked PVC, i.e.,
	// the original PV from the pool.
	LockedPersistentVolume *corev1obj.PersistentVolume

	// Conds represent status updates for given conditions.
	Conds map[pvpoolv1alpha1.CheckoutConditionType]pvpoolv1alpha1.Condition
}

var _ lifecycle.Loader = &CheckoutState{}
var _ lifecycle.Persister = &CheckoutState{}

func (cs *CheckoutState) loadFromPool(ctx context.Context, cl client.Client) (bool, error) {
	namespace := cs.Checkout.Object.Spec.PoolRef.Namespace
	if namespace == "" {
		namespace = cs.Checkout.Key.Namespace
	}

	pool := pvpoolv1alpha1obj.NewPool(client.ObjectKey{
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
		cs.LockedPersistentVolume = pr.PersistentVolume
	}

	return true, nil
}

func (cs *CheckoutState) Load(ctx context.Context, cl client.Client) (bool, error) {
	pairs := []struct {
		PersistentVolumeClaim *corev1obj.PersistentVolumeClaim
		PersistentVolume      **corev1obj.PersistentVolume
	}{
		{
			PersistentVolumeClaim: cs.LockedPersistentVolumeClaim,
			PersistentVolume:      &cs.LockedPersistentVolume,
		},
		{
			PersistentVolumeClaim: cs.PersistentVolumeClaim,
			PersistentVolume:      &cs.PersistentVolume,
		},
	}
	for _, pair := range pairs {
		if ok, err := pair.PersistentVolumeClaim.Load(ctx, cl); err != nil {
			return false, err
		} else if !ok {
			continue
		}

		if ctrl := metav1.GetControllerOf(pair.PersistentVolumeClaim.Object); ctrl == nil || ctrl.UID != cs.Checkout.Object.GetUID() {
			cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
				Status:  corev1.ConditionUnknown,
				Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonConflict,
				Message: fmt.Sprintf("A non-controlled PVC with the name %s already exists.", pair.PersistentVolumeClaim.Key.Name),
			}
			return false, errmark.MarkTransient(fmt.Errorf("a PVC with this checkout's desired name already exists"))
		}

		volume := corev1obj.NewPersistentVolume(pair.PersistentVolumeClaim.Object.Spec.VolumeName)
		if ok, err := volume.Load(ctx, cl); err != nil {
			return false, err
		} else if !ok {
			continue
		}

		// Sanity check: volume must cross-reference the PVC for successful
		// pre-bind.
		if volume.Object.Spec.ClaimRef == nil || volume.Object.Spec.ClaimRef.UID != pair.PersistentVolumeClaim.Object.GetUID() {
			klog.InfoS("checkout state: load: persistent volume does not match claim (not using)", "checkout", cs.Checkout.Key, "pvc", pair.PersistentVolumeClaim.Key, "pv", volume.Name)
			continue
		}

		*pair.PersistentVolume = volume
	}

	if cs.PersistentVolumeClaim.Object.Status.Phase != corev1.ClaimBound && cs.LockedPersistentVolume == nil {
		// We need to get one from the pool.
		if ok, err := cs.loadFromPool(ctx, cl); err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

func (cs *CheckoutState) Persist(ctx context.Context, cl client.Client) error {
	// We can either be in a state where we're still trying to allocate the PV
	// and PVC, or we can have the PVC settled and bound. If it's not bound,
	// we'll also set up the locked PV/PVC.
	switch cs.PersistentVolumeClaim.Object.Status.Phase {
	case corev1.ClaimBound:
		cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
			Status:  corev1.ConditionTrue,
			Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonCheckedOut,
			Message: "The PVC is ready to use.",
		}

		// Now that we've allocated everything, we no longer need the locked PV
		// (must be manually deleted) or PVC.
		if cs.LockedPersistentVolume != nil {
			if _, err := cs.LockedPersistentVolume.Delete(ctx, cl, lifecycle.DeleteWithPropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
				return err
			}
		}

		if _, err := cs.LockedPersistentVolumeClaim.Delete(ctx, cl); err != nil {
			return err
		}
	default:
		// Note on the logic here: if we successfully wrote both the locked PVC
		// and PV, we have completed the pre-bind and we're guaranteed to have
		// it settle at some point.
		//
		// We don't actually need to wait for the controller to make that
		// happen, but if it does, we'll just go through the reconcile loop
		// again.
		pairs := []struct {
			PersistentVolumeClaim *corev1obj.PersistentVolumeClaim
			PersistentVolume      *corev1obj.PersistentVolume
		}{
			{
				PersistentVolumeClaim: cs.LockedPersistentVolumeClaim,
				PersistentVolume:      cs.LockedPersistentVolume,
			},
			{
				PersistentVolumeClaim: cs.PersistentVolumeClaim,
				PersistentVolume:      cs.PersistentVolume,
			},
		}
		for _, pair := range pairs {
			if err := cs.Checkout.Own(ctx, pair.PersistentVolumeClaim); err != nil {
				return err
			}

			if err := pair.PersistentVolumeClaim.Persist(ctx, cl); errors.IsInvalid(err) {
				cs.Conds[pvpoolv1alpha1.CheckoutAcquired] = pvpoolv1alpha1.Condition{
					Status:  corev1.ConditionFalse,
					Reason:  pvpoolv1alpha1.CheckoutAcquiredReasonInvalid,
					Message: fmt.Sprintf("The PVC %q could not be created because of configuration problems: %v", pair.PersistentVolumeClaim.Key.Name, err),
				}
				return errmark.MarkUser(err)
			} else if err != nil {
				return err
			}

			if pair.PersistentVolume == nil {
				eventctx.EventRecorder(ctx).Event(cs.Checkout.Object, "Warning", "VolumeAttachment", fmt.Sprintf("Volume for PVC %q could not be found", pair.PersistentVolumeClaim.Key.Name))

				return errmark.MarkTransient(fmt.Errorf("missing persistent volume"))
			}

			// Sync UID to ClaimRef.
			pair.PersistentVolume.Object.Spec.ClaimRef.UID = pair.PersistentVolumeClaim.Object.GetUID()

			if err := pair.PersistentVolume.Persist(ctx, cl); err != nil {
				return err
			}
		}
	}

	return nil
}

func NewCheckoutState(c *pvpoolv1alpha1obj.Checkout) *CheckoutState {
	claimName := c.Object.Spec.ClaimName
	if claimName == "" {
		claimName = c.Key.Name
	}

	return &CheckoutState{
		Checkout: c,
		PersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
			Namespace: c.Key.Namespace,
			Name:      claimName,
		}),
		LockedPersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(client.ObjectKey{
			Namespace: c.Key.Namespace,
			Name:      norm.MetaNameSuffixed(c.Key.Name, "-locked"),
		}),
		Conds: make(map[pvpoolv1alpha1.CheckoutConditionType]pvpoolv1alpha1.Condition),
	}
}

func ConfigureCheckoutState(cs *CheckoutState) (*CheckoutState, error) {
	switch {
	case cs.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound:
		return cs, nil
	case cs.LockedPersistentVolume == nil:
		klog.V(4).InfoS("checkout state: configure: no volume, clearing status", "checkout", cs.Checkout.Key)
		return cs, nil
	case cs.PersistentVolume == nil:
		// Uniqueness follows the underlying volume.
		cs.PersistentVolume = corev1obj.NewPersistentVolume(norm.MetaNameSuffixed("pvpool", "-"+string(cs.LockedPersistentVolume.Object.GetUID())))
	}

	// We need to keep track of the original reclaim policy so we can use it for
	// our new PV. We use an annotation to do so.
	//
	// It's important that we only set this once, because once we modify the
	// target PV, it will not have the right value!
	if _, ok := cs.LockedPersistentVolume.Object.GetAnnotations()[CheckoutReclaimPolicyAnnotationKey]; !ok {
		helper.Annotate(cs.LockedPersistentVolume.Object, CheckoutReclaimPolicyAnnotationKey, string(cs.LockedPersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy))
	}

	// Copy locked PV to new PV. Note that we also copy annotations as they are
	// used to keep track of deallocators in CSI.
	helper.CopyLabelsAndAnnotations(cs.PersistentVolume.Object, cs.LockedPersistentVolume.Object)
	cs.LockedPersistentVolume.Object.Spec.DeepCopyInto(&cs.PersistentVolume.Object.Spec)

	// Set up PV.
	cs.PersistentVolume.Object.Spec.AccessModes = cs.Checkout.Object.Spec.AccessModes
	cs.PersistentVolume.Object.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: corev1obj.PersistentVolumeClaimKind.GroupVersion().String(),
		Kind:       corev1obj.PersistentVolumeClaimKind.Kind,
		Namespace:  cs.PersistentVolumeClaim.Key.Namespace,
		Name:       cs.PersistentVolumeClaim.Key.Name,
		UID:        cs.PersistentVolumeClaim.Object.GetUID(),
	}
	cs.PersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimPolicy(cs.LockedPersistentVolume.Object.GetAnnotations()[CheckoutReclaimPolicyAnnotationKey])

	// In most volume sources, setting the readOnly field just forces mounts to
	// also have the readOnly flag set. However, for CSI, it also changes the
	// behavior of the external attacher so that it requests the correct mode
	// from the CSI driver.
	if spec := &cs.PersistentVolume.Object.Spec; spec.CSI != nil {
		spec.CSI.ReadOnly = len(spec.AccessModes) == 1 && spec.AccessModes[0] == corev1.ReadOnlyMany
	}

	// Set up the PVC to point at the PV.
	cs.PersistentVolumeClaim.Object.Spec.StorageClassName = &cs.PersistentVolume.Object.Spec.StorageClassName
	cs.PersistentVolumeClaim.Object.Spec.VolumeName = cs.PersistentVolume.Name
	cs.PersistentVolumeClaim.Object.Spec.AccessModes = cs.Checkout.Object.Spec.AccessModes
	cs.PersistentVolumeClaim.Object.Spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: cs.PersistentVolume.Object.Spec.Capacity.Storage().DeepCopy(),
		},
	}

	// Set up locked PV to point at our locked PVC.
	cs.LockedPersistentVolume.Object.Spec.ClaimRef = &corev1.ObjectReference{
		APIVersion: corev1obj.PersistentVolumeClaimKind.GroupVersion().String(),
		Kind:       corev1obj.PersistentVolumeClaimKind.Kind,
		Namespace:  cs.LockedPersistentVolumeClaim.Key.Namespace,
		Name:       cs.LockedPersistentVolumeClaim.Key.Name,
		UID:        cs.LockedPersistentVolumeClaim.Object.GetUID(),
	}

	// Set locked PV to retain so we won't release the underlying storage when
	// we delete the locked PVC.
	cs.LockedPersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain

	// Configure the temporary locked PVC.
	cs.LockedPersistentVolumeClaim.Object.Spec.StorageClassName = &cs.LockedPersistentVolume.Object.Spec.StorageClassName
	cs.LockedPersistentVolumeClaim.Object.Spec.VolumeName = cs.LockedPersistentVolume.Name
	cs.LockedPersistentVolumeClaim.Object.Spec.AccessModes = cs.LockedPersistentVolume.Object.Spec.AccessModes
	cs.LockedPersistentVolumeClaim.Object.Spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: cs.LockedPersistentVolume.Object.Spec.Capacity.Storage().DeepCopy(),
		},
	}

	return cs, nil
}
