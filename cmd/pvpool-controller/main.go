package main

import (
	"context"
	"os"

	"github.com/puppetlabs/leg/mainutil"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	reconcilers = []func(mgr manager.Manager) error{
		controller.AddCheckoutReconcilerToManager,
		controller.AddPoolReconcilerToManager,
	}
)

func main() {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	// XXX: ERR
	_ = pvpoolv1alpha1.AddToScheme(scheme)

	mgr, _ := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
	})

	for _, reconciler := range reconcilers {
		// XXX: ERR
		_ = reconciler(mgr)
	}

	os.Exit(mainutil.TrapAndWait(ctx, mgr.Start))
}
