package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func Load() (*Config, error) {
	v := viper.New()

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath("../config")

	v.SetConfigName("config")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	v.SetConfigName(fmt.Sprintf("config-%s", env))
	if err := v.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("EDUGO_WORKER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.BindEnv("database.postgres.password", "POSTGRES_PASSWORD")
	v.BindEnv("database.mongodb.uri", "MONGODB_URI")
	v.BindEnv("messaging.rabbitmq.url", "RABBITMQ_URL")
	v.BindEnv("nlp.api_key", "OPENAI_API_KEY")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
