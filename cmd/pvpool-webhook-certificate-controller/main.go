package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/puppetlabs/leg/k8sutil/pkg/app/selfsignedsecret"
	"github.com/puppetlabs/leg/k8sutil/pkg/app/webhookcert"
	corev1obj "github.com/puppetlabs/leg/k8sutil/pkg/controller/obj/api/corev1"
	"github.com/puppetlabs/pvpool/pkg/opt"
	"github.com/puppetlabs/pvpool/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	cfg := opt.NewConfig("pvpool-webhook-certificate-controller")

	secretKey := client.ObjectKey{
		Namespace: cfg.Namespace,
		Name:      cfg.WebhookCertificateSecretName,
	}

	os.Exit(runtime.Main(
		cfg,
		manager.Options{
			LeaderElection: true,
			Namespace:      cfg.Namespace,
		},
		func(mgr manager.Manager) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			secret := corev1obj.NewTLSSecret(secretKey)
			if err := secret.Persist(ctx, mgr.GetClient()); errors.IsAlreadyExists(err) {
				return nil
			} else if err != nil {
				return err
			}

			return nil
		},
		func(mgr manager.Manager) error {
			return webhookcert.AddReconcilerToManager(
				mgr,
				secretKey,
				webhookcert.WithValidatingWebhookConfiguration(cfg.ValidatingWebhookConfigurationName),
			)
		},
		func(mgr manager.Manager) error {
			return selfsignedsecret.AddReconcilerToManager(
				mgr,
				secretKey,
				"Puppet, Inc.",
				fmt.Sprintf("%s.%s.svc", cfg.WebhookServiceName, cfg.Namespace),
			)
		},
	))
}
