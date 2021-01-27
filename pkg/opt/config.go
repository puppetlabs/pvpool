package opt

import "github.com/spf13/viper"

type Config struct {
	// Debug determines whether this server starts with debugging enabled.
	Debug bool
}

func NewConfig() *Config {
	viper.SetEnvPrefix("pvpool")
	viper.AutomaticEnv()

	return &Config{
		Debug: viper.GetBool("debug"),
	}
}
