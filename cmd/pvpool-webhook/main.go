package main

import (
	"os"

	"github.com/puppetlabs/pvpool/pkg/opt"
	"github.com/puppetlabs/pvpool/pkg/runtime"
	"github.com/puppetlabs/pvpool/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	cfg := opt.NewConfig("pvpool-webhook")

	os.Exit(runtime.Main(
		cfg,
		manager.Options{},
		webhook.AddCheckoutValidatorToManager,
		webhook.AddPoolValidatorToManager,
	))
}
