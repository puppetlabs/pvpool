package app

import (
	"context"

	batchv1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/batchv1"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/helper"
	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	"github.com/puppetlabs/leg/mathutil/pkg/rand"
	"github.com/puppetlabs/pvpool/pkg/obj"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PoolReplicaPhaseAnnotationKey = "pvpool.puppet.com/replica.phase"

	PoolReplicaPhaseAnnotationValueInitializing = "Initializing"
	PoolReplicaPhaseAnnotationValueAvailable    = "Available"
)

type PoolReplica struct {
	Pool                  *obj.Pool
	PersistentVolumeClaim *corev1obj.PersistentVolumeClaim
	InitJob               *batchv1obj.Job
}

var _ lifecycle.Deleter = &PoolReplica{}
var _ lifecycle.Loader = &PoolReplica{}
var _ lifecycle.Persister = &PoolReplica{}

func (pr *PoolReplica) Delete(ctx context.Context, cl client.Client, opts ...lifecycle.DeleteOption) (bool, error) {
	return pr.PersistentVolumeClaim.Delete(ctx, cl, opts...)
}

func (pr *PoolReplica) Load(ctx context.Context, cl client.Client) (bool, error) {
	return lifecycle.Loaders{
		pr.PersistentVolumeClaim,
		lifecycle.IgnoreNilLoader{Loader: pr.InitJob},
	}.Load(ctx, cl)
}

func (pr *PoolReplica) Persist(ctx context.Context, cl client.Client) error {
	if err := pr.Pool.Own(ctx, pr.PersistentVolumeClaim); err != nil {
		return err
	}

	if pr.InitJob != nil {
		// Copy labels and annotations from template.
		pr.InitJob.LabelAnnotateFrom(ctx, &pr.Pool.Object.Spec.InitJob.Template.ObjectMeta)
	}

	// We set ownership on the init job indirectly so we can receive updates.
	if err := helper.OwnUncontrolled(pr.InitJob.Object, lifecycle.TypedObject{
		GVK:    obj.PoolKind,
		Object: pr.Pool.Object,
	}); err != nil {
		return err
	}

	return lifecycle.OwnershipPersister{
		Owner:     pr.PersistentVolumeClaim,
		Dependent: lifecycle.IgnoreNilOwnablePersister{OwnablePersister: pr.InitJob},
	}.Persist(ctx, cl)
}

func (pr *PoolReplica) Stale() bool {
	return pr.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimLost ||
		(pr.InitJob != nil && pr.InitJob.Failed())
}

func (pr *PoolReplica) Available() bool {
	return pr.PersistentVolumeClaim.Object.Status.Phase == corev1.ClaimBound &&
		pr.PersistentVolumeClaim.Object.GetAnnotations()[PoolReplicaPhaseAnnotationKey] == PoolReplicaPhaseAnnotationValueAvailable
}

func NewPoolReplica(p *obj.Pool, key client.ObjectKey) *PoolReplica {
	pr := &PoolReplica{
		Pool:                  p,
		PersistentVolumeClaim: corev1obj.NewPersistentVolumeClaim(key),
	}

	if pr.Pool.Object.Spec.InitJob != nil {
		pr.InitJob = batchv1obj.NewJob(key)
	}

	return pr
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
	}

	// Configure init job if it hasn't already started to run.
	if pr.InitJob != nil && !pr.InitJob.Succeeded() {
		// Copy spec from template.
		pr.InitJob.Object.Spec = pr.Pool.Object.Spec.InitJob.Template.Spec

		// Set up volume.
		vols := &pr.InitJob.Object.Spec.Template.Spec.Volumes
		volIdx := indexVolumeByName(*vols, pr.Pool.Object.Spec.InitJob.VolumeName)
		if volIdx < 0 {
			volIdx = len(*vols)
			*vols = append(*vols, corev1.Volume{Name: pr.Pool.Object.Spec.InitJob.VolumeName})
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
