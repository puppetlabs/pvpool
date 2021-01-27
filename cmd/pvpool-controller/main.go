package main

import (
	"os"

	"github.com/puppetlabs/pvpool/pkg/controller/reconciler"
	"github.com/puppetlabs/pvpool/pkg/runtime"
)

func main() {
	os.Exit(runtime.Main(
		"pvpool-controller",
		reconciler.AddCheckoutReconcilerToManager,
		reconciler.AddPoolReconcilerToManager,
	))
}
