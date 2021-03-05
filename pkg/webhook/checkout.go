package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:name=checkout.validate.webhook.pvpool.puppet.com,groups=pvpool.puppet.com,versions=v1alpha1,resources=checkouts,verbs=create;update,path=/validate-pvpool-puppet-com-v1alpha1-checkout,failurePolicy=fail,mutating=false,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

// CheckoutValidatorHandler performs validation for the Checkout type.
type CheckoutValidatorHandler struct {
	cl      client.Client
	decoder *admission.Decoder
	mapper  meta.RESTMapper
}

func (cvh *CheckoutValidatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
	default:
		return admission.Allowed("")
	}

	checkout := &pvpoolv1alpha1.Checkout{}
	if err := cvh.decoder.Decode(req, checkout); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	gvr, err := cvh.mapper.RESTMapping(pvpoolv1alpha1.PoolKind.GroupKind())
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	namespace := checkout.Spec.PoolRef.Namespace
	if namespace == "" {
		namespace = checkout.GetNamespace()
	}

	extra := make(map[string]authorizationv1.ExtraValue, len(req.UserInfo.Extra))
	for k, v := range req.UserInfo.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}

	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "use",
				Group:     gvr.Resource.Group,
				Resource:  gvr.Resource.Resource,
				Namespace: namespace,
				Name:      checkout.Spec.PoolRef.Name,
			},
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			Extra:  extra,
			UID:    req.UserInfo.UID,
		},
	}
	if err := cvh.cl.Create(ctx, review); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !review.Status.Allowed {
		var err error
		if review.Status.Reason != "" {
			err = errors.New(review.Status.Reason)
		} else {
			err = fmt.Errorf("User %q cannot use resource %q in API group %q in the namespace %q", req.UserInfo.Username, gvr.Resource.Resource, gvr.Resource.Group, namespace)
		}

		status := k8serrors.NewForbidden(
			gvr.Resource.GroupResource(),
			checkout.Spec.PoolRef.Name,
			err,
		).Status()

		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result:  &status,
			},
		}
	}

	return admission.Allowed("")
}

var _ admission.DecoderInjector = &CheckoutValidatorHandler{}
var _ inject.Mapper = &CheckoutValidatorHandler{}

func (cvh *CheckoutValidatorHandler) InjectDecoder(d *admission.Decoder) error {
	cvh.decoder = d
	return nil
}

func (cvh *CheckoutValidatorHandler) InjectMapper(m meta.RESTMapper) error {
	cvh.mapper = m
	return nil
}

func AddCheckoutValidatorToManager(mgr manager.Manager) error {
	hnd := &CheckoutValidatorHandler{
		cl: mgr.GetClient(),
	}

	mgr.GetWebhookServer().Register(
		"/validate-pvpool-puppet-com-v1alpha1-checkout",
		&admission.Webhook{Handler: hnd},
	)
	if err := mgr.AddHealthzCheck("checkout", func(_ *http.Request) error {
		return nil
	}); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("checkout", func(_ *http.Request) error {
		return nil
	}); err != nil {
		return err
	}
	return nil
}
