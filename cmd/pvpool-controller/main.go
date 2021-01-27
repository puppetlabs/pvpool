package main

import (
	"context"
	"flag"
	"os"

	"github.com/puppetlabs/leg/mainutil"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	schemes = runtime.NewSchemeBuilder(
		scheme.AddToScheme,
		pvpoolv1alpha1.AddToScheme,
	)

	reconcilers = []func(mgr manager.Manager) error{
		controller.AddCheckoutReconcilerToManager,
		controller.AddPoolReconcilerToManager,
	}
)

func main() {
	os.Exit(mainutil.TrapAndWait(context.Background(), func(ctx context.Context) error {
		defer klog.Flush()

		flag.Parse()

		kfs := flag.NewFlagSet("klog", flag.ExitOnError)
		klog.InitFlags(kfs)

		s := runtime.NewScheme()
		if err := schemes.AddToScheme(s); err != nil {
			klog.Fatalf("failed to create scheme: %+v", err)
		}

		mgr, _ := manager.New(config.GetConfigOrDie(), manager.Options{
			Scheme: s,
		})

		for i, reconciler := range reconcilers {
			if err := reconciler(mgr); err != nil {
				klog.Fatalf("failed to add reconciler #%d: %+v", i, err)
			}
		}

		return mgr.Start(ctx)
	}))
}
