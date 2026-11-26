package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Defaults of the rescue of failed AI work (HU-IA.6, P9 of the MVP-IA plan).
const (
	DefaultAIRescueSweepInterval        = 5 * time.Minute
	DefaultAIRescueMinAge               = 15 * time.Minute
	DefaultAIRescueMaxRetries           = 5
	DefaultAIRescueRetryBackoff         = "2m,10m,30m,2h,6h"
	DefaultAIRescueJobPendingStaleAfter = 6 * time.Hour
)

// aiRescueClaimTTL is the AI review lock of learning: the minimum age must exceed it, or
// the sweep would republish an attempt that is still being reviewed (learning answers 400).
const aiRescueClaimTTL = 10 * time.Minute

// ErrInvalidAIRescue marks an ai_rescue block the worker cannot run with: it does not start.
var ErrInvalidAIRescue = errors.New("invalid ai_rescue config")

// AIRescueConfig is the ai_rescue block as it is read (config.yaml and AI_RESCUE_*).
// Zero or absent means the default. RetryBackoff is a comma separated list of durations.
type AIRescueConfig struct {
	Enabled              *bool         `mapstructure:"enabled"`
	SweepInterval        time.Duration `mapstructure:"sweep_interval"`
	MinAge               time.Duration `mapstructure:"min_age"`
	MaxRetries           int           `mapstructure:"max_retries"`
	RetryBackoff         string        `mapstructure:"retry_backoff"`
	JobPendingStaleAfter time.Duration `mapstructure:"job_pending_stale_after"`
}

// AIRescueSettings is the resolved and validated ai_rescue block.
type AIRescueSettings struct {
	Enabled              bool
	SweepInterval        time.Duration
	MinAge               time.Duration
	MaxRetries           int
	RetryBackoff         []time.Duration
	JobPendingStaleAfter time.Duration
}

// Settings resolves the defaults and validates the block with a typed error.
func (c AIRescueConfig) Settings() (AIRescueSettings, error) {
	s := AIRescueSettings{
		Enabled:              c.Enabled == nil || *c.Enabled,
		SweepInterval:        orDuration(c.SweepInterval, DefaultAIRescueSweepInterval),
		MinAge:               orDuration(c.MinAge, DefaultAIRescueMinAge),
		MaxRetries:           c.MaxRetries,
		JobPendingStaleAfter: orDuration(c.JobPendingStaleAfter, DefaultAIRescueJobPendingStaleAfter),
	}
	if s.MaxRetries == 0 {
		s.MaxRetries = DefaultAIRescueMaxRetries
	}
	raw := c.RetryBackoff
	if strings.TrimSpace(raw) == "" {
		raw = DefaultAIRescueRetryBackoff
	}
	backoff, err := parseBackoff(raw)
	if err != nil {
		return AIRescueSettings{}, err
	}
	s.RetryBackoff = backoff
	if err := s.validate(); err != nil {
		return AIRescueSettings{}, err
	}
	return s, nil
}

func (s AIRescueSettings) validate() error {
	switch {
	case s.SweepInterval < time.Second:
		return fmt.Errorf("%w: ai_rescue.sweep_interval=%s (AI_RESCUE_SWEEP_INTERVAL) debe ser de al menos 1s",
			ErrInvalidAIRescue, s.SweepInterval)
	case s.MinAge.Truncate(time.Second) <= aiRescueClaimTTL:
		return fmt.Errorf("%w: ai_rescue.min_age=%s (AI_RESCUE_MIN_AGE) debe superar los %s del candado",
			ErrInvalidAIRescue, s.MinAge, aiRescueClaimTTL)
	case s.MaxRetries < 1:
		return fmt.Errorf("%w: ai_rescue.max_retries=%d (AI_RESCUE_MAX_RETRIES) debe ser positivo",
			ErrInvalidAIRescue, s.MaxRetries)
	case s.JobPendingStaleAfter < time.Second:
		return fmt.Errorf("%w: ai_rescue.job_pending_stale_after=%s (AI_RESCUE_JOB_PENDING_STALE_AFTER) debe ser de al menos 1s",
			ErrInvalidAIRescue, s.JobPendingStaleAfter)
	}
	return nil
}

// parseBackoff reads "2m,10m,30m" into durations of at least one second each.
func parseBackoff(raw string) ([]time.Duration, error) {
	var out []time.Duration
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("%w: ai_rescue.retry_backoff=%q (AI_RESCUE_RETRY_BACKOFF) tiene un hueco", ErrInvalidAIRescue, raw)
		}
		d, err := time.ParseDuration(part)
		if err != nil {
			return nil, fmt.Errorf("%w: ai_rescue.retry_backoff=%q (AI_RESCUE_RETRY_BACKOFF): %v", ErrInvalidAIRescue, raw, err)
		}
		if d < time.Second {
			return nil, fmt.Errorf("%w: ai_rescue.retry_backoff=%q (AI_RESCUE_RETRY_BACKOFF): %s es menor que 1s", ErrInvalidAIRescue, raw, d)
		}
		out = append(out, d)
	}
	return out, nil
}

func orDuration(v, def time.Duration) time.Duration {
	if v == 0 {
		return def
	}
	return v
}
