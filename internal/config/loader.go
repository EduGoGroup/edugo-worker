package config

import (
	"fmt"
	"os"

	sharedconfig "github.com/EduGoGroup/edugo-shared/config"
)

func Load() (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	var cfg Config

	loader := sharedconfig.NewLoader(
		sharedconfig.WithEnvPrefix("EDUGO_WORKER"),
		sharedconfig.WithConfigPath("./config"),
		sharedconfig.WithConfigPath("../config"),
		sharedconfig.WithEnvironmentOverride(env),
		sharedconfig.WithDefaults(map[string]interface{}{
			"logging.version": "dev",
		}),
		sharedconfig.WithExplicitBindings(map[string]string{
			"messaging.rabbitmq.url": "RABBITMQ_URL",
			"nlp.api_key":            "OPENAI_API_KEY",
		}),
	)

	if err := loader.Load(&cfg); err != nil {
		return nil, fmt.Errorf("error loading worker config: %w", err)
	}

	// Derive logging.env from APP_ENV so logs are labeled with the correct environment
	if cfg.Logging.Env == "" {
		cfg.Logging.Env = env
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
