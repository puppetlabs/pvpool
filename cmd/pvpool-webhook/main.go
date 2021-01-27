package main

import (
	"os"

	"github.com/puppetlabs/pvpool/pkg/runtime"
	"github.com/puppetlabs/pvpool/pkg/webhook"
)

func main() {
	os.Exit(runtime.Main(
		"pvpool-webhook",
		webhook.AddCheckoutValidatorToManager,
		webhook.AddPoolValidatorToManager,
	))
}
