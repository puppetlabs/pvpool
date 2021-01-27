package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	"github.com/puppetlabs/leg/mainutil"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/controller"
	"github.com/puppetlabs/pvpool/pkg/opt"
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
		cfg := opt.NewConfig()

		defer klog.Flush()

		flag.Parse()

		kfs := flag.NewFlagSet("klog", flag.ExitOnError)
		klog.InitFlags(kfs)

		if cfg.Debug {
			_ = kfs.Set("v", "5")
		}

		s := runtime.NewScheme()
		if err := schemes.AddToScheme(s); err != nil {
			return fmt.Errorf("failed to create scheme: %w", err)
		}

		mgr, _ := manager.New(config.GetConfigOrDie(), manager.Options{
			Scheme: s,
		})

		for i, reconciler := range reconcilers {
			if err := reconciler(mgr); err != nil {
				return fmt.Errorf("failed to add reconciler #%d: %w", i, err)
			}
		}

		return mgr.Start(eventctx.WithEventRecorder(ctx, mgr, "pvpool-controller"))
	}))
}
