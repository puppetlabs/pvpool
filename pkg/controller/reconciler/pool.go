package reconciler

import (
	"context"
	"time"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/lifecycle"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/controller/app"
	"github.com/puppetlabs/pvpool/pkg/obj"
	"github.com/puppetlabs/pvpool/pkg/opt"
	"golang.org/x/time/rate"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// +kubebuilder:rbac:groups=pvpool.puppet.com,resources=pools,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=pvpool.puppet.com,resources=pools/status,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete

const (
	PoolReconcilerFinalizerName = "pvpool.puppet.com/pool-reconciler"
)

type PoolReconciler struct {
	cl client.Client
}

var _ reconcile.Reconciler = &PoolReconciler{}

func (pr *PoolReconciler) Reconcile(ctx context.Context, req reconcile.Request) (r reconcile.Result, err error) {
	klog.InfoS("pool reconciler: starting reconcile for pool", "pool", req.NamespacedName)
	defer klog.InfoS("pool reconciler: ending reconcile for pool", "pool", req.NamespacedName)
	defer func() {
		if err != nil {
			klog.ErrorS(err, "pool reconciler: failed to reconcile pool", "pool", req.NamespacedName)
		}
	}()

	pool := obj.NewPool(req.NamespacedName)
	if ok, err := pool.Load(ctx, pr.cl); err != nil || !ok {
		return reconcile.Result{}, err
	}

	ps := app.NewPoolState(pool)
	defer func() {
		pool = app.ConfigurePool(ps)
		if pool.Finalizing() {
			return
		}

		if serr := pool.PersistStatus(ctx, pr.cl); serr != nil {
			if err == nil {
				err = serr
			} else {
				klog.ErrorS(serr, "pool reconciler: failed to update pool status", "pool", req.NamespacedName)
			}
		}
	}()

	if ok, err := ps.Load(ctx, pr.cl); err != nil || !ok {
		return reconcile.Result{Requeue: true}, err
	}

	finalized, err := lifecycle.Finalize(ctx, pr.cl, PoolReconcilerFinalizerName, pool, func() error {
		_, err := ps.Delete(ctx, pr.cl)
		return err
	})
	if err != nil || finalized {
		return reconcile.Result{}, err
	}

	ps = app.ConfigurePoolState(ps)

	err = ps.Persist(ctx, pr.cl)
	return
}

func NewPoolReconciler(cl client.Client) *PoolReconciler {
	return &PoolReconciler{
		cl: cl,
	}
}

func AddPoolReconcilerToManager(mgr manager.Manager, cfg *opt.Config) error {
	rl := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, cfg.ControllerMaxReconcileBackoffDuration),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)

	r := NewPoolReconciler(mgr.GetClient())

	return builder.ControllerManagedBy(mgr).
		For(&pvpoolv1alpha1.Pool{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(
			&source.Kind{Type: &batchv1.Job{}},
			app.DependencyManager.NewEnqueueRequestForAnnotatedDependencyOf(&pvpoolv1alpha1.Pool{}),
		).
		WithOptions(controller.Options{RateLimiter: rl}).
		Complete(r)
}
