package main

import (
	"os"

	"github.com/puppetlabs/pvpool/pkg/controller/reconciler"
	"github.com/puppetlabs/pvpool/pkg/opt"
	"github.com/puppetlabs/pvpool/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	cfg := opt.NewConfig("pvpool-controller")

	os.Exit(runtime.Main(
		cfg,
		manager.Options{
			LeaderElection: true,
		},
		func(mgr manager.Manager) error {
			return reconciler.AddCheckoutReconcilerToManager(mgr, cfg)
		},
		func(mgr manager.Manager) error {
			return reconciler.AddPoolReconcilerToManager(mgr, cfg)
		},
	))
}
