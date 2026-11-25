package config

import (
	"errors"
	"testing"
)

func TestInferenceCap_Defaults(t *testing.T) {
	local, api := LLMConfig{}.InferenceCaps()
	if local != 1 || api != 4 {
		t.Fatalf("por defecto se esperaba local=1 y api=4, llegó local=%d api=%d", local, api)
	}

	cfg := LLMConfig{Local: LLMLocalConfig{MaxInFlight: 3}, API: LLMAPIConfig{MaxInFlight: 9}}
	if local, api = cfg.InferenceCaps(); local != 3 || api != 9 {
		t.Fatalf("valores configurados: local=%d api=%d", local, api)
	}
}

// The two variables reach the config through the real loader and the versioned YAML.
func TestInferenceCap_Env(t *testing.T) {
	t.Chdir("../..")
	t.Setenv("APP_ENV", "local")
	t.Setenv("RABBITMQ_URL", "amqp://test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load sin variables: %v", err)
	}
	if local, api := cfg.LLM.InferenceCaps(); local != 1 || api != 4 {
		t.Fatalf("config.yaml: local=%d api=%d", local, api)
	}

	t.Setenv("LLM_LOCAL_MAX_IN_FLIGHT", "2")
	t.Setenv("LLM_API_MAX_IN_FLIGHT", "7")
	if cfg, err = Load(); err != nil {
		t.Fatalf("Load con variables: %v", err)
	}
	if local, api := cfg.LLM.InferenceCaps(); local != 2 || api != 7 {
		t.Fatalf("por variable: local=%d api=%d", local, api)
	}

	t.Setenv("LLM_LOCAL_MAX_IN_FLIGHT", "-1")
	if _, err = Load(); !errors.Is(err, ErrInvalidInferenceCap) {
		t.Fatalf("un tope negativo no debe arrancar: %v", err)
	}
}

func TestInferenceCap_RejectsNegative(t *testing.T) {
	base := Config{}
	base.Messaging.RabbitMQ.URL = "amqp://test"

	for name, mutate := range map[string]func(*Config){
		"local": func(c *Config) { c.LLM.Local.MaxInFlight = -1 },
		"api":   func(c *Config) { c.LLM.API.MaxInFlight = -3 },
	} {
		cfg := base
		mutate(&cfg)
		if err := cfg.Validate(); !errors.Is(err, ErrInvalidInferenceCap) {
			t.Fatalf("%s negativo: se esperaba ErrInvalidInferenceCap, llegó %v", name, err)
		}
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("cero es el valor por defecto, no un error: %v", err)
	}
}
