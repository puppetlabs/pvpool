package opt

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// Debug determines whether this server starts with debugging enabled.
	Debug bool

	// Name is the name of this deployment, if known.
	Name string

	// Namespace is the Kubernetes namespace this deployment is running in, if
	// known.
	Namespace string

	// ControllerMaxReconcileBackoffDuration is the amount of time the
	// controller may wait to reprocess an object that has encountered an error.
	ControllerMaxReconcileBackoffDuration time.Duration

	// WebhookServiceName is the name of the service that provides access to the
	// webook.
	WebhookServiceName string

	// WebhookCertificateSecretName is the name of the secret that should
	// contain the certificate data for the webhook.
	WebhookCertificateSecretName string

	// ValidatingWebhookConfigurationName is the name of the admission webhook
	// configuration for the API server to communicate with our webhook.
	ValidatingWebhookConfigurationName string
}

func NewConfig(defaultName string) *Config {
	viper.SetEnvPrefix("pvpool")
	viper.AutomaticEnv()

	viper.SetDefault("name", defaultName)
	viper.SetDefault("controller_max_reconcile_backoff_duration", 1*time.Minute)

	return &Config{
		Debug:                                 viper.GetBool("debug"),
		Name:                                  viper.GetString("name"),
		Namespace:                             viper.GetString("namespace"),
		ControllerMaxReconcileBackoffDuration: viper.GetDuration("controller_max_reconcile_backoff_duration"),
		WebhookServiceName:                    viper.GetString("webhook_service_name"),
		WebhookCertificateSecretName:          viper.GetString("webhook_certificate_secret_name"),
		ValidatingWebhookConfigurationName:    viper.GetString("validating_webhook_configuration_name"),
	}
}
