package validation

import (
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidatePersistentVolumeClaimTemplate(tpl *pvpoolv1alpha1.PersistentVolumeClaimTemplate, selector labels.Selector, p *field.Path) (errs field.ErrorList) {
	errs = append(errs, metav1validation.ValidateLabels(tpl.Labels, p.Child("metadata", "labels"))...)
	errs = append(errs, apimachineryvalidation.ValidateAnnotations(tpl.Annotations, p.Child("metadata", "labels"))...)

	if !selector.Empty() {
		labels := labels.Set(tpl.Labels)
		if !selector.Matches(labels) {
			errs = append(errs, field.Invalid(p.Child("metadata", "labels"), tpl.Labels, "`selector` does not match template `labels`"))
		}
	}

	return
}

func ValidatePoolSpec(spec *pvpoolv1alpha1.PoolSpec, p *field.Path) (errs field.ErrorList) {
	errs = append(errs, metav1validation.ValidateLabelSelector(&spec.Selector, p.Child("selector"))...)
	if len(spec.Selector.MatchLabels)+len(spec.Selector.MatchExpressions) == 0 {
		errs = append(errs, field.Invalid(p.Child("selector"), spec.Selector, "empty selector is invalid for deployment"))
	}

	selector, err := metav1.LabelSelectorAsSelector(&spec.Selector)
	if err != nil {
		errs = append(errs, field.Invalid(p.Child("selector"), spec.Selector, "invalid label selector"))
	} else {
		errs = append(errs, ValidatePersistentVolumeClaimTemplate(&spec.Template, selector, p.Child("template"))...)
	}

	return
}

func ValidatePoolSpecUpdate(newSpec, oldSpec *pvpoolv1alpha1.PoolSpec, p *field.Path) (errs field.ErrorList) {
	errs = append(errs, ValidatePoolSpec(newSpec, p)...)
	errs = append(errs, apimachineryvalidation.ValidateImmutableField(newSpec.Selector, oldSpec.Selector, p.Child("selector"))...)
	return
}
