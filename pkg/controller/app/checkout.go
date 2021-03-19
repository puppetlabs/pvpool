package app

import (
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	pvpoolv1alpha1obj "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1/obj"
	corev1 "k8s.io/api/core/v1"
)

func ConfigureCheckout(cs *CheckoutState) *pvpoolv1alpha1obj.Checkout {
	if cs.PersistentVolume == nil {
		// Something didn't go right when loading probably. Clear our state.
		cs.Checkout.Object.Status.VolumeName = ""
		cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
	} else {
		cs.Checkout.Object.Status.VolumeName = cs.PersistentVolume.Name

		if cs.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound {
			cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{
				Name: cs.PersistentVolumeClaim.Key.Name,
			}
		} else {
			cs.Checkout.Object.Status.VolumeClaimRef = corev1.LocalObjectReference{}
		}
	}

	var conds []pvpoolv1alpha1.CheckoutCondition
	for _, typ := range []pvpoolv1alpha1.CheckoutConditionType{pvpoolv1alpha1.CheckoutAcquired} {
		prev, _ := cs.Checkout.Condition(typ)
		next := cs.Conds[typ]
		conds = append(conds, pvpoolv1alpha1.CheckoutCondition{
			Condition: UpdateCondition(prev.Condition, next),
			Type:      typ,
		})
	}
	cs.Checkout.Object.Status.Conditions = conds

	return cs.Checkout
}
