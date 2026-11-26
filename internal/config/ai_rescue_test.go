package config

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

var p9Backoff = []time.Duration{2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}

// TestAIRescueConfig_Defaults: the P9 numbers by default, from the code and from the
// versioned YAML, and each AI_RESCUE_* variable through the real loader.
func TestAIRescueConfig_Defaults(t *testing.T) {
	want := AIRescueSettings{
		Enabled: true, SweepInterval: 5 * time.Minute, MinAge: 15 * time.Minute, MaxRetries: 5,
		RetryBackoff: p9Backoff, JobPendingStaleAfter: 6 * time.Hour,
	}
	got, err := AIRescueConfig{}.Settings()
	if err != nil {
		t.Fatalf("el bloque vacío usa los valores por defecto: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("por defecto: got %+v, want %+v", got, want)
	}

	t.Chdir("../..")
	t.Setenv("APP_ENV", "local")
	t.Setenv("RABBITMQ_URL", "amqp://test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load sin variables: %v", err)
	}
	if got, err = cfg.AIRescue.Settings(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("config.yaml: got %+v (%v), want %+v", got, err, want)
	}

	t.Setenv("AI_RESCUE_ENABLED", "false")
	t.Setenv("AI_RESCUE_SWEEP_INTERVAL", "10s")
	t.Setenv("AI_RESCUE_MIN_AGE", "11m")
	t.Setenv("AI_RESCUE_MAX_RETRIES", "2")
	t.Setenv("AI_RESCUE_RETRY_BACKOFF", "15s, 30s")
	t.Setenv("AI_RESCUE_JOB_PENDING_STALE_AFTER", "1h")
	if cfg, err = Load(); err != nil {
		t.Fatalf("Load con variables: %v", err)
	}
	want = AIRescueSettings{
		Enabled: false, SweepInterval: 10 * time.Second, MinAge: 11 * time.Minute, MaxRetries: 2,
		RetryBackoff: []time.Duration{15 * time.Second, 30 * time.Second}, JobPendingStaleAfter: time.Hour,
	}
	if got, err = cfg.AIRescue.Settings(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("por variable: got %+v (%v), want %+v", got, err, want)
	}

	t.Setenv("AI_RESCUE_MIN_AGE", "10m")
	if _, err = Load(); !errors.Is(err, ErrInvalidAIRescue) {
		t.Fatalf("una edad mínima que no supera el candado no arranca: %v", err)
	}
}

func TestAIRescueConfig_RejectsInvalid(t *testing.T) {
	for name, cfg := range map[string]AIRescueConfig{
		"min_age igual al candado":    {MinAge: 10 * time.Minute},
		"min_age bajo el candado":     {MinAge: 5 * time.Minute},
		"min_age con fracción":        {MinAge: 10*time.Minute + 500*time.Millisecond},
		"intervalo negativo":          {SweepInterval: -time.Second},
		"reintentos negativos":        {MaxRetries: -1},
		"espera ilegible":             {RetryBackoff: "2m,diez"},
		"espera con hueco":            {RetryBackoff: "2m,,10m"},
		"espera menor de un segundo":  {RetryBackoff: "2m,500ms"},
		"job pending negativo":        {JobPendingStaleAfter: -time.Hour},
		"lista de esperas solo comas": {RetryBackoff: ","},
	} {
		if _, err := cfg.Settings(); !errors.Is(err, ErrInvalidAIRescue) {
			t.Fatalf("%s: se esperaba ErrInvalidAIRescue, llegó %v", name, err)
		}
	}
	if _, err := (AIRescueConfig{MinAge: 10*time.Minute + time.Second}).Settings(); err != nil {
		t.Fatalf("10m1s supera el candado: %v", err)
	}
}
