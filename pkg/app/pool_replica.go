package app

import (
	"context"
	"fmt"

	batchv1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/batchv1"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/k8sutil/pkg/norm"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PoolReplicaPhaseAnnotationKey = "pvpool.puppet.com/replica.phase"

	PoolReplicaPhaseAnnotationValueInitializing = "Initializing"
	PoolReplicaPhaseAnnotationValueAvailable    = "Available"
)

var (
	DefaultPoolReplicaInitJobSpec = batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "init",
						// https://hub.docker.com/layers/busybox/library/busybox/stable-musl/images/sha256-8d0c42425011ea3fb5b4ec5a121dde4ce986c2efea46be9d981a478fe1d206ec?context=explore
						Image: "busybox@sha256:8d0c42425011ea3fb5b4ec5a121dde4ce986c2efea46be9d981a478fe1d206ec",
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		},
		ActiveDeadlineSeconds: func(i int64) *int64 { return &i }(300),
		BackoffLimit:          func(i int32) *int32 { return &i }(10),
	}
)

type PoolReplica struct {
	Pool                  *obj.Pool
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim
	PersistentVolume      *corev1obj.PersistentVolume
	InitJob               *batchv1obj.Job
}

var _ lifecycle.Deleter = &PoolReplica{}
var _ lifecycle.Loader = &PoolReplica{}
var _ lifecycle.Persister = &PoolReplica{}

func (pr *PoolReplica) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	// We can get into a state where a Checkout has failed but managed to update
	// the PV to Retain. In this case, we need to reset the policy.
	if pr.PersistentVolume != nil && pr.PersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		pr.PersistentVolume.Object.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete

		if err := pr.PersistentVolume.Persist(ctx, cl); err != nil {
			return false, err
		}
	}

	// We have to delete the job first because its existence will block the PVC
	// from being deleted (unless it's failed).
	if _, err := pr.InitJob.Delete(ctx, cl, opts...); err != nil {
		return false, err
	}

	return pr.PersistentVolumeClaim.Delete(ctx, cl, opts...)
}

func (pr *PoolReplica) Load(ctx context.Context, cl client.Client) (bool, error) {
	// The init job may not exist. This is desired behavior.
	if _, err := pr.InitJob.Load(ctx, cl); err != nil {
		return false, err
	}

	ok, err := pr.PersistentVolumeClaim.Load(ctx, cl)
	if err != nil || !ok {
		return ok, err
	}

	if pr.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound {
		volume := corev1obj.NewPersistentVolume(pr.PersistentVolumeClaim.Object.Spec.VolumeName)
		if ok, err := volume.Load(ctx, cl); err != nil || !ok {
			return ok, err
		} else if volume.Object.Spec.ClaimRef.UID != pr.PersistentVolumeClaim.Object.GetUID() {
			return false, nil
		}

		pr.PersistentVolume = volume
	}

	return true, nil
}

func (pr *PoolReplica) Persist(ctx context.Context, cl client.Client) error {
	pr.PersistentVolumeClaim.LabelAnnotateFrom(ctx, &pr.Pool.Object.Spec.Template.ObjectMeta)

	if err := pr.Pool.Own(ctx, pr.PersistentVolumeClaim); err != nil {
		return err
	}

	switch {
	case pr.Available():
		// If we're in the Available phase, we can go ahead and delete the init
		// job and not worry about it again.
		if _, err := pr.InitJob.Delete(ctx, cl, lifecycle.DeleteWithPropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			return err
		}

		fallthrough
	case helper.Exists(pr.InitJob.Object):
		// If the init job exists, we can't reconfigure it. Jobs are effectively
		// immutable once they start.
		return pr.PersistentVolumeClaim.Persist(ctx, cl)
	default:
		// Otherwise, we're likely creating it for the very first time.

		// Copy labels and annotations from template.
		if pr.Pool.Object.Spec.InitJob != nil {
			pr.InitJob.LabelAnnotateFrom(ctx, &pr.Pool.Object.Spec.InitJob.Template.ObjectMeta)
		}

		// We set ownership on the init job indirectly so we can receive updates.
		// Don't use OwnUncontrolled here because it will block deletion of the job.
		// Just use SetDependencyOf, our external tracking mechanism.
		if err := DependencyManager.SetDependencyOf(pr.InitJob.Object, lifecycle.TypedObject{
			GVK:    obj.PoolKind,
			Object: pr.Pool.Object,
		}); err != nil {
			return err
		}

		return lifecycle.OwnershipPersister{
			Owner:     pr.PersistentVolumeClaim,
			Dependent: pr.InitJob,
		}.Persist(ctx, cl)
	}
}

func (pr *PoolReplica) Stale() bool {
	return !pr.PersistentVolumeClaim.Object.GetDeletionTimestamp().IsZero() ||
		pr.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimLost ||
		pr.InitJob.Failed()
}

func (pr *PoolReplica) Available() bool {
	return pr.PersistentVolume != nil && pr.PersistentVolumeClaim.Object.GetAnnotations()[PoolReplicaPhaseAnnotationKey] == PoolReplicaPhaseAnnotationValueAvailable
}

func NewPoolReplica(p *obj.Pool, key client.ObjectKey) *PoolReplica {
	return &PoolReplica{
		Pool:                  p,
		PersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(key),
		InitJob:               batchv1obj.NewJob(key),
	}
}

func ConfigurePoolReplica(pr *PoolReplica) *PoolReplica {
	if pr.Stale() || pr.Available() {
		return pr
	}

	// Configure the PVC if it's not yet bound.
	if pvc := pr.PersistentVolumeClaim.Object; pvc.Status.Phase != corev1.ClaimBound {
		pvc.Spec = pr.Pool.Object.Spec.Template.Spec

		// We always request dynamic provisioning, so we must prevent certain
		// fields from being set.
		//
		// A storageClassName of "" (not nil) disables dynamic provisioning.
		if storageClass := pvc.Spec.StorageClassName; storageClass != nil && *storageClass == "" {
			pr.PersistentVolumeClaim.Object.Spec.StorageClassName = nil
		}

		// Any selector also disables dynamic provisioning.
		pvc.Spec.Selector = nil

		// Specifying an explicit binding, obviously, also disables dynamic
		// provisioning.
		pvc.Spec.VolumeName = ""

		// A PVC in the pool should always be RWO.
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		}
	}

	// Configure init job if it hasn't already started to run. Note that we
	// always configure an init job because some storage classes insist use
	// WaitForFirstConsumer which is not compatible with pooling.
	if !pr.InitJob.Succeeded() {
		var volumeName string

		// Copy spec from template if it exists.
		if pr.Pool.Object.Spec.InitJob != nil {
			pr.InitJob.Object.Spec = pr.Pool.Object.Spec.InitJob.Template.Spec
			volumeName = pr.Pool.Object.Spec.InitJob.VolumeName
		} else {
			pr.InitJob.Object.Spec = DefaultPoolReplicaInitJobSpec
			volumeName = "workspace"
		}

		// Set up volume.
		vols := &pr.InitJob.Object.Spec.Template.Spec.Volumes
		volIdx := indexVolumeByName(*vols, volumeName)
		if volIdx < 0 {
			volIdx = len(*vols)
			*vols = append(*vols, corev1.Volume{Name: volumeName})
		}
		(*vols)[volIdx].PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pr.PersistentVolumeClaim.Key.Name,
		}

		// Mark PVC as initializing.
		helper.Annotate(pr.PersistentVolumeClaim.Object, PoolReplicaPhaseAnnotationKey, PoolReplicaPhaseAnnotationValueInitializing)
	} else {
		helper.Annotate(pr.PersistentVolumeClaim.Object, PoolReplicaPhaseAnnotationKey, PoolReplicaPhaseAnnotationValueAvailable)
	}

	return pr
}

func ApplyPoolReplica(ctx context.Context, cl client.Client, p *obj.Pool, id string) (*PoolReplica, error) {
	key := client.ObjectKey{
		Namespace: p.Key.Namespace,
		Name:      norm.MetaNameSuffixed(p.Key.Name, fmt.Sprintf("-%s", id)),
	}

	pr := NewPoolReplica(p, key)

	if _, err := pr.Load(ctx, cl); err != nil {
		return nil, err
	}

	pr = ConfigurePoolReplica(pr)

	if err := pr.Persist(ctx, cl); err != nil {
		return nil, err
	}

	return pr, nil
}

type PoolReplicas []*PoolReplica

func (prs *PoolReplicas) Pop(rng rand.Rand) (*PoolReplica, bool, error) {
	n := uint64(len(*prs))
	if n == 0 {
		return nil, false, nil
	}

	i, err := rand.Uint64N(rng, n)
	if err != nil {
		return nil, false, err
	}

	pr := (*prs)[i]
	(*prs)[i] = (*prs)[n-1]
	*prs = (*prs)[:n-1]

	return pr, true, nil
}

func indexVolumeByName(vols []corev1.Volume, name string) int {
	for i := range vols {
		if vols[i].Name == name {
			return i
		}
	}

	return -1
}
