package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/metrics"
)

// State representa el estado del circuit breaker
type State int

const (
	StateClosed   State = iota // Permite todas las peticiones
	StateOpen                  // Rechaza todas las peticiones
	StateHalfOpen              // Permite peticiones limitadas para probar
)

// String convierte el estado a string para logging
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen se retorna cuando el circuit breaker está abierto
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrTooManyRequests se retorna cuando hay demasiadas peticiones en half-open
	ErrTooManyRequests = errors.New("too many requests")
)

// Config configuración del circuit breaker
type Config struct {
	Name              string        // Nombre del circuit breaker para métricas
	MaxFailures       uint32        // Número máximo de fallos antes de abrir
	Timeout           time.Duration // Tiempo antes de pasar de open a half-open
	MaxRequests       uint32        // Máximo de peticiones en half-open
	SuccessThreshold  uint32        // Éxitos necesarios en half-open para cerrar
	FailureRateWindow time.Duration // Ventana de tiempo para calcular tasa de fallos
}

// DefaultConfig retorna una configuración por defecto
func DefaultConfig(name string) Config {
	return Config{
		Name:              name,
		MaxFailures:       5,
		Timeout:           60 * time.Second,
		MaxRequests:       1,
		SuccessThreshold:  2,
		FailureRateWindow: 30 * time.Second,
	}
}

// CircuitBreaker implementa el patrón circuit breaker
type CircuitBreaker struct {
	config Config

	mu              sync.RWMutex
	state           State
	failures        uint32
	successes       uint32
	requests        uint32
	lastStateChange time.Time
	lastFailure     time.Time
}

// New crea un nuevo circuit breaker
func New(config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}

	// Registrar estado inicial en métricas
	metrics.SetCircuitBreakerState(config.Name, int(StateClosed))

	return cb
}

// Execute ejecuta la función proporcionada con protección del circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Verificar si podemos ejecutar
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Ejecutar la función
	err := fn(ctx)

	// Registrar el resultado
	cb.afterRequest(err)

	return err
}

// beforeRequest verifica si se puede hacer la petición
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Permitir la petición
		return nil

	case StateOpen:
		// Verificar si ha pasado el timeout
		if now.Sub(cb.lastStateChange) >= cb.config.Timeout {
			cb.setState(StateHalfOpen, now)
			cb.requests++
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// Limitar el número de peticiones de forma atómica
		// Verificar e incrementar en el mismo paso para evitar race conditions
		if cb.requests < cb.config.MaxRequests {
			cb.requests++
			return nil
		}
		return ErrTooManyRequests

	default:
		return ErrCircuitOpen
	}
}

// afterRequest registra el resultado de la petición
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	if err != nil {
		// Petición fallida
		cb.failures++
		cb.lastFailure = now

		switch cb.state {
		case StateClosed:
			// Verificar si debemos abrir el circuit
			if cb.failures >= cb.config.MaxFailures {
				cb.setState(StateOpen, now)
			}

		case StateHalfOpen:
			// Un fallo en half-open vuelve a abrir el circuit
			cb.setState(StateOpen, now)
		}
	} else {
		// Petición exitosa
		cb.successes++

		switch cb.state {
		case StateClosed:
			// Resetear el contador de fallos si llevamos tiempo sin fallos
			if now.Sub(cb.lastFailure) >= cb.config.FailureRateWindow {
				cb.failures = 0
			}

		case StateHalfOpen:
			// Verificar si debemos cerrar el circuit
			if cb.successes >= cb.config.SuccessThreshold {
				cb.setState(StateClosed, now)
			}
		}
	}
}

// setState cambia el estado del circuit breaker
func (cb *CircuitBreaker) setState(newState State, now time.Time) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = now

	// Resetear contadores según el nuevo estado
	switch newState {
	case StateClosed:
		cb.failures = 0
		cb.successes = 0
		cb.requests = 0
	case StateOpen:
		cb.requests = 0
		cb.successes = 0
	case StateHalfOpen:
		cb.requests = 0
		cb.successes = 0
	}

	// Registrar transición en métricas
	metrics.SetCircuitBreakerState(cb.config.Name, int(newState))
	metrics.RecordCircuitBreakerTransition(cb.config.Name, oldState.String(), newState.String())
}

// State retorna el estado actual del circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures retorna el número de fallos actuales
func (cb *CircuitBreaker) Failures() uint32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Successes retorna el número de éxitos actuales
func (cb *CircuitBreaker) Successes() uint32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successes
}
