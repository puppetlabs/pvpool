package validation

import (
	"fmt"

	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	MountJobSpecBackoffPolicy        = corev1.RestartPolicyNever
	MountJobMaxActiveDeadlineSeconds = 300
	MountJobMaxBackoffLimit          = 10
)

func ValidatePersistentVolumeClaimTemplate(tpl *pvpoolv1alpha1.PersistentVolumeClaimTemplate, selector labels.Selector, p *field.Path) (errs field.ErrorList) {
	errs = append(errs, metav1validation.ValidateLabels(tpl.Labels, p.Child("metadata", "labels"))...)
	errs = append(errs, apimachineryvalidation.ValidateAnnotations(tpl.Annotations, p.Child("metadata", "labels"))...)

	if !selector.Empty() {
		ls := labels.Set(tpl.Labels)
		if !selector.Matches(ls) {
			errs = append(errs, field.Invalid(p.Child("metadata", "labels"), tpl.Labels, "`selector` does not match template `labels`"))
		}
	}

	return
}

func ValidateMountJob(j *pvpoolv1alpha1.MountJob, p *field.Path) (errs field.ErrorList) {
	if j.Template.Spec.Template.Spec.RestartPolicy != "" && j.Template.Spec.Template.Spec.RestartPolicy != MountJobSpecBackoffPolicy {
		errs = append(errs, field.NotSupported(
			p.Child("template", "spec", "template", "spec", "restartPolicy"),
			j.Template.Spec.Template.Spec.RestartPolicy,
			[]string{string(MountJobSpecBackoffPolicy)},
		))
	}

	if j.Template.Spec.ActiveDeadlineSeconds != nil && *j.Template.Spec.ActiveDeadlineSeconds > MountJobMaxActiveDeadlineSeconds {
		errs = append(errs, field.Invalid(
			p.Child("template", "spec", "activeDeadlineSeconds"),
			*j.Template.Spec.ActiveDeadlineSeconds,
			fmt.Sprintf("must be at most %d", MountJobMaxActiveDeadlineSeconds),
		))
	}

	if j.Template.Spec.BackoffLimit != nil && *j.Template.Spec.BackoffLimit > MountJobMaxBackoffLimit {
		errs = append(errs, field.Invalid(
			p.Child("template", "spec", "backoffLimit"),
			*j.Template.Spec.BackoffLimit,
			fmt.Sprintf("must be at most %d", MountJobMaxBackoffLimit),
		))
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

	if spec.InitJob != nil {
		errs = append(errs, ValidateMountJob(spec.InitJob, p.Child("initJob"))...)
	}

	return
}

func ValidatePoolSpecUpdate(newSpec, oldSpec *pvpoolv1alpha1.PoolSpec, p *field.Path) (errs field.ErrorList) {
	errs = append(errs, ValidatePoolSpec(newSpec, p)...)
	errs = append(errs, apimachineryvalidation.ValidateImmutableField(newSpec.Selector, oldSpec.Selector, p.Child("selector"))...)
	return
}
