package app

import (
	"context"
	"fmt"
	"sort"

	batchv1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/batchv1"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	storagev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/storagev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/k8sutil/pkg/norm"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	pvpoolv1alpha1obj "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1/obj"
	pvpoolv1alpha1validation "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1/validation"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
			},
		},
		BackoffLimit: pointer.Int32Ptr(pvpoolv1alpha1validation.MountJobMaxBackoffLimit),
	}
)

type PoolReplica struct {
	Pool                  *pvpoolv1alpha1obj.Pool
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim
	PersistentVolume      *corev1obj.PersistentVolume
	VolumeAttachments     []*storagev1obj.VolumeAttachment
	InitJob               *batchv1obj.Job
}

var _ lifecycle.Deleter = &PoolReplica{}
var _ lifecycle.Loader = &PoolReplica{}
var _ lifecycle.Persister = &PoolReplica{}

func (pr *PoolReplica) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	// We have to delete the job first because its existence will block the PVC
	// from being deleted (unless it's failed).
	if _, err := pr.InitJob.Delete(ctx, cl, opts...); err != nil {
		return false, err
	}

	return pr.PersistentVolumeClaim.Delete(ctx, cl, opts...)
}

func (pr *PoolReplica) loadVolumeAttachments(ctx context.Context, cl client.Reader) (bool, error) {
	pr.VolumeAttachments = nil

	l := &storagev1.VolumeAttachmentList{}
	if err := cl.List(ctx, l); err != nil {
		return false, err
	}
	for i := range l.Items {
		va := storagev1obj.NewVolumeAttachmentFromObject(&l.Items[i])
		if name := va.Object.Spec.Source.PersistentVolumeName; name == nil || *name != pr.PersistentVolume.Name {
			continue
		}

		pr.VolumeAttachments = append(pr.VolumeAttachments, va)
	}

	return len(pr.VolumeAttachments) > 0, nil
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

	if !pr.Stale() && !pr.Available() && pr.PersistentVolume != nil {
		// List relevant VolumeAttachments and determine if our PV is free to be
		// used by pool consumers.
		//
		// This operation is relatively heavy, as it requires listing and
		// iterating all VolumeAttachments (there's no way to map a PV to its
		// set of attachments, e.g., by a label selector), so it's important to
		// usually use a cache here (the default for controller-runtime).
		//
		// However, from a consistency perspective, the VolumeAttachment cache
		// and the PV cache are maintained separately and I don't think we can
		// depend on the API server to send watch events in any particular order
		// between them. That means we have to contend with the init-job
		// finishing while we've never seen the first instance of a
		// VolumeAttachment appearing. This is particularly confounded by the
		// fact that only CSI and CSI migration use VolumeAttachments, so we
		// basically have no idea if the object should exist at all.
		//
		// So, if we have no VolumeAttachments and we have a complete InitJob,
		// we'll actually bypass the cache and use the real API client to verify
		// everything looks OK. This will happen infrequently on clusters with
		// actual VolumeAttachment objects (otherwise the list will likely be
		// empty), so the performance impact should be negligible.
		//
		// We make this behavior opt-in, so if you don't pass a client that is
		// capable of bypassing the cache, we just make this a best-effort
		// operation.
		if ok, err := pr.loadVolumeAttachments(ctx, cl); err != nil {
			return false, err
		} else if !ok && pr.InitJob.Complete() {
			if bcl, ok := cl.(lifecycle.CacheBypasserClient); ok {
				if _, err := pr.loadVolumeAttachments(ctx, bcl.BypassCache()); err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

func (pr *PoolReplica) Persist(ctx context.Context, cl client.Client) error {
	pr.PersistentVolumeClaim.LabelAnnotateFrom(ctx, &pr.Pool.Object.Spec.Template.ObjectMeta)

	if err := pr.Pool.Own(ctx, pr.PersistentVolumeClaim); err != nil {
		return err
	}

	// Always update the PVC configuration once we have it.
	if err := pr.PersistentVolumeClaim.Persist(ctx, cl); err != nil {
		return err
	}

	switch {
	case pr.Available():
		// If we're in the Available phase, we can go ahead and delete the init
		// job and not worry about it again.
		if _, err := pr.InitJob.Delete(ctx, cl, lifecycle.DeleteWithPropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
			return err
		}

		fallthrough
	case helper.Exists(pr.InitJob.Object):
		// If the init job exists, we can't reconfigure it. Jobs are effectively
		// immutable once they start.

		// We need to track VolumeAttachments to make sure we re-enter the
		// reconcile loop for the pool when they change state.
		for _, va := range pr.VolumeAttachments {
			if err := DependencyManager.SetDependencyOf(va.Object, lifecycle.TypedObject{
				GVK:    pvpoolv1alpha1obj.PoolKind,
				Object: pr.Pool.Object,
			}); err != nil {
				return err
			}

			if err := va.Persist(ctx, cl); err != nil {
				return err
			}
		}
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
			GVK:    pvpoolv1alpha1obj.PoolKind,
			Object: pr.Pool.Object,
		}); err != nil {
			return err
		}

		if err := (lifecycle.OwnershipPersister{
			Owner:     pr.PersistentVolumeClaim,
			Dependent: pr.InitJob,
		}.Persist(ctx, cl)); err != nil {
			return err
		}
	}

	return nil
}

func (pr *PoolReplica) Stale() bool {
	return !pr.PersistentVolumeClaim.Object.GetDeletionTimestamp().IsZero() ||
		pr.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimLost ||
		pr.InitJob.Failed()
}

func (pr *PoolReplica) Available() bool {
	return pr.PersistentVolume != nil &&
		pr.PersistentVolumeClaim.Object.GetAnnotations()[PoolReplicaPhaseAnnotationKey] == PoolReplicaPhaseAnnotationValueAvailable
}

func NewPoolReplica(p *pvpoolv1alpha1obj.Pool, key client.ObjectKey) *PoolReplica {
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
	if pvc := pr.PersistentVolumeClaim.Object; pvc.Status.Phase != corev1.ClaimPending && pvc.Status.Phase != corev1.ClaimBound {
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

		// We default to RWO, but pools may request other modes if they want.
		if len(pvc.Spec.AccessModes) == 0 {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			}
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

		// Configure some of the required fields.
		if pr.InitJob.Object.Spec.Template.Spec.RestartPolicy == "" {
			pr.InitJob.Object.Spec.Template.Spec.RestartPolicy = pvpoolv1alpha1validation.MountJobSpecBackoffPolicy
		}

		if pr.InitJob.Object.Spec.ActiveDeadlineSeconds == nil || *pr.InitJob.Object.Spec.ActiveDeadlineSeconds > pvpoolv1alpha1validation.MountJobMaxActiveDeadlineSeconds {
			pr.InitJob.Object.Spec.ActiveDeadlineSeconds = pointer.Int64Ptr(pvpoolv1alpha1validation.MountJobMaxActiveDeadlineSeconds)
		}

		if pr.InitJob.Object.Spec.BackoffLimit != nil && *pr.InitJob.Object.Spec.BackoffLimit > pvpoolv1alpha1validation.MountJobMaxBackoffLimit {
			pr.InitJob.Object.Spec.BackoffLimit = pointer.Int32Ptr(pvpoolv1alpha1validation.MountJobMaxBackoffLimit)
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
	} else if len(pr.VolumeAttachments) == 0 {
		helper.Annotate(pr.PersistentVolumeClaim.Object, PoolReplicaPhaseAnnotationKey, PoolReplicaPhaseAnnotationValueAvailable)
	}

	return pr
}

func ApplyPoolReplica(ctx context.Context, cl client.Client, p *pvpoolv1alpha1obj.Pool, id string) (*PoolReplica, error) {
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

type PoolReplicasSortByCreationTimestamp []*PoolReplica

var _ sort.Interface = PoolReplicasSortByCreationTimestamp(nil)

func (prs PoolReplicasSortByCreationTimestamp) Len() int      { return len(prs) }
func (prs PoolReplicasSortByCreationTimestamp) Swap(i, j int) { prs[i], prs[j] = prs[j], prs[i] }
func (prs PoolReplicasSortByCreationTimestamp) Less(i, j int) bool {
	return prs[i].PersistentVolumeClaim.Object.CreationTimestamp.Before(&prs[j].PersistentVolumeClaim.Object.CreationTimestamp)
}
