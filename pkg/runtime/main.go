package runtime

import (
	"context"
	"flag"
	"fmt"

	"github.com/puppetlabs/leg/k8sutil/pkg/controller/eventctx"
	"github.com/puppetlabs/leg/mainutil"
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/opt"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var schemes = runtime.NewSchemeBuilder(
	scheme.AddToScheme,
	pvpoolv1alpha1.AddToScheme,
)

func Main(cfg *opt.Config, opts manager.Options, transforms ...func(mgr manager.Manager) error) int {
	return mainutil.TrapAndWait(context.Background(), func(ctx context.Context) error {
		defer klog.Flush()

		flag.Parse()

		kfs := flag.NewFlagSet("klog", flag.ExitOnError)
		klog.InitFlags(kfs)

		if cfg.Debug {
			_ = kfs.Set("v", "5")
		}

		log.SetLogger(klogr.NewWithOptions(klogr.WithFormat(klogr.FormatKlog)))

		if opts.Scheme == nil {
			s := runtime.NewScheme()
			if err := schemes.AddToScheme(s); err != nil {
				return fmt.Errorf("failed to create scheme: %w", err)
			}

			opts.Scheme = s
		}

		mgr, err := manager.New(config.GetConfigOrDie(), opts)
		if err != nil {
			return fmt.Errorf("failed to create manager: %w", err)
		}

		for i, transform := range transforms {
			if err := transform(mgr); err != nil {
				return fmt.Errorf("failed to apply manager transform #%d: %w", i, err)
			}
		}

		return mgr.Start(eventctx.WithEventRecorder(ctx, mgr, cfg.Name))
	})
}
