package ratelimiter

import sharedrl "github.com/EduGoGroup/edugo-shared/resilience/ratelimiter"

type RateLimiter = sharedrl.RateLimiter
type Config = sharedrl.Config
type MultiRateLimiter = sharedrl.MultiRateLimiter

// New creates a new RateLimiter with the given requests per second and burst size.
func New(requestsPerSecond float64, burstSize float64) *RateLimiter {
	return sharedrl.New(requestsPerSecond, burstSize)
}

// NewMulti creates a new MultiRateLimiter with per-key configs and an optional default config.
func NewMulti(configs map[string]Config, defaultConfig *Config) *MultiRateLimiter {
	return sharedrl.NewMulti(configs, defaultConfig)
}
