package circuitbreaker

import sharedcb "github.com/EduGoGroup/edugo-shared/resilience/circuitbreaker"

// Re-export types from shared module
type State = sharedcb.State
type Config = sharedcb.Config
type CircuitBreaker = sharedcb.CircuitBreaker
type MetricsHook = sharedcb.MetricsHook

// Re-export constants
const (
	StateClosed   = sharedcb.StateClosed
	StateOpen     = sharedcb.StateOpen
	StateHalfOpen = sharedcb.StateHalfOpen
)

// Re-export errors
var (
	ErrCircuitOpen     = sharedcb.ErrCircuitOpen
	ErrTooManyRequests = sharedcb.ErrTooManyRequests
)

// New creates a new CircuitBreaker with the given configuration.
func New(config Config) *CircuitBreaker {
	return sharedcb.New(config)
}

// DefaultConfig returns a default Config for the given name.
func DefaultConfig(name string) Config {
	return sharedcb.DefaultConfig(name)
}
