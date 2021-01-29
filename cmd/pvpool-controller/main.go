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
		manager.Options{},
		reconciler.AddCheckoutReconcilerToManager,
		reconciler.AddPoolReconcilerToManager,
	))
}
