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
			// M2M (plan 039 F4): service JWT + base URLs de dominio.
			"service_jwt.secret":    "SERVICE_JWT_SECRET",
			"api_academic.base_url": "API_ACADEMIC_BASE_URL",
			"api_learning.base_url": "API_LEARNING_BASE_URL",
			// LLM (plan 039 D-039.3): credenciales/URL/modelo de EduGo, no por escuela.
			"llm.local.base_url": "LLM_LOCAL_BASE_URL",
			"llm.local.model":    "LLM_LOCAL_MODEL",
			"llm.api.provider":   "LLM_API_PROVIDER",
			"llm.api.api_key":    "LLM_API_KEY",
			"llm.api.model":      "LLM_API_MODEL",
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
