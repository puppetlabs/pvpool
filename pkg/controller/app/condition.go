package app

import (
	"time"

	"github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateCondition(prev, next v1alpha1.Condition) v1alpha1.Condition {
	if next.Status == "" {
		next.Status = corev1.ConditionUnknown
	}

	if next.Status == prev.Status && next.Reason == prev.Reason && next.Message == prev.Message {
		return prev
	}

	if next.LastTransitionTime.IsZero() {
		next.LastTransitionTime = metav1.Time{Time: time.Now()}
	}

	return next
}
