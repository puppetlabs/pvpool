package webhook

import (
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:name=checkout.validate.webhook.pvpool.puppet.com,groups=pvpool.puppet.com,versions=v1alpha1,resources=checkouts,verbs=create;update,path=/validate-pvpool-puppet-com-v1alpha1-checkout,failurePolicy=fail,mutating=false,sideEffects=None,admissionReviewVersions=v1

// CheckoutValidator extends the Checkout type to provide validation.
//
// +kubebuilder:object:root=true
type CheckoutValidator struct {
	*pvpoolv1alpha1.Checkout `json:",inline"`
}

var _ webhook.Validator = &CheckoutValidator{}

func (cv *CheckoutValidator) ValidateCreate() error {
	return nil
}

func (cv *CheckoutValidator) ValidateUpdate(old runtime.Object) error {
	return nil
}

func (cv *CheckoutValidator) ValidateDelete() error {
	return nil
}

func AddCheckoutValidatorToManager(mgr manager.Manager) error {
	mgr.GetWebhookServer().Register("/validate-pvpool-puppet-com-v1alpha1-checkout", admission.ValidatingWebhookFor(&CheckoutValidator{}))
	return nil
}
