package controller

import (
	"context"

	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/app"
	"github.com/puppetlabs/pvpool/pkg/obj"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// +kubebuilder:rbac:groups=pvpool.puppet.com,resources=pools,verbs=get;list;watch
// +kubebuilder:rbac:groups=pvpool.puppet.com,resources=pools/status,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=batch,resources=job,verbs=get;list;watch;create;update;delete

type PoolReconciler struct {
	cl client.Client
}

var _ reconcile.Reconciler = &PoolReconciler{}

func (pr *PoolReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pool := obj.NewPool(req.NamespacedName)
	if ok, err := pool.Load(ctx, pr.cl); err != nil || !ok {
		return reconcile.Result{}, err
	}

	_, err := app.ApplyPoolState(ctx, pr.cl, pool)
	return reconcile.Result{}, err
}

func NewPoolReconciler(cl client.Client) *PoolReconciler {
	return &PoolReconciler{
		cl: cl,
	}
}

func AddPoolReconcilerToManager(mgr manager.Manager) error {
	r := NewPoolReconciler(mgr.GetClient())

	return builder.ControllerManagedBy(mgr).
		For(&pvpoolv1alpha1.Pool{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(
			&source.Kind{Type: &batchv1.Job{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &pvpoolv1alpha1.Pool{},
				IsController: false,
			},
		).
		Complete(r)
}
