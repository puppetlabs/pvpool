package webhook

import (
	"fmt"

	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/validation"
	"k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:name=pool.validate.webhook.pvpool.puppet.com,groups=pvpool.puppet.com,versions=v1alpha1,resources=pools,verbs=create;update,path=/validate-pvpool-puppet-com-v1alpha1-pool,failurePolicy=fail,mutating=false,sideEffects=None,admissionReviewVersions=v1

// PoolValidator extends the Pool type to provide validation.
//
// +kubebuilder:object:root=true
type PoolValidator struct {
	*pvpoolv1alpha1.Pool `json:",inline"`
}

var _ webhook.Validator = &PoolValidator{}

func (pv *PoolValidator) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validation.ValidatePoolSpec(&pv.Spec, field.NewPath("spec"))...)

	if len(errs) != 0 {
		return errors.NewInvalid(pvpoolv1alpha1.PoolKind.GroupKind(), pv.GetName(), errs)
	}

	return nil
}

func (pv *PoolValidator) ValidateUpdate(old runtime.Object) error {
	oldPV, ok := old.(*PoolValidator)
	if !ok {
		return fmt.Errorf("unexpected type %T for old object in update", old)
	}

	var errs field.ErrorList
	errs = append(errs, validation.ValidatePoolSpecUpdate(&pv.Spec, &oldPV.Spec, field.NewPath("spec"))...)

	if len(errs) != 0 {
		return errors.NewInvalid(pvpoolv1alpha1.PoolKind.GroupKind(), pv.GetName(), errs)
	}

	return nil
}

func (pv *PoolValidator) ValidateDelete() error {
	return nil
}

func AddPoolValidatorToManager(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register("/validate-pvpool-puppet-com-v1alpha1-pool", admission.ValidatingWebhookFor(&PoolValidator{}))
	return nil
}
