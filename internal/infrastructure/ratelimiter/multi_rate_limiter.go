package ratelimiter

import (
	"context"
	"sync"
)

// Config configuración para un rate limiter individual
type Config struct {
	RequestsPerSecond float64 // Requests por segundo permitidos
	BurstSize         float64 // Tamaño máximo de ráfaga
}

// MultiRateLimiter gestiona múltiples rate limiters por tipo de evento
// Permite configurar límites diferentes para cada tipo de evento
type MultiRateLimiter struct {
	limiters      map[string]*RateLimiter
	defaultConfig *Config
	mu            sync.RWMutex
}

// NewMulti crea un nuevo multi-rate-limiter con configuraciones por tipo de evento
//
// Parámetros:
//   - configs: mapa de tipo de evento -> configuración de rate limiting
//   - defaultConfig: configuración por defecto para eventos no especificados (opcional)
//
// Ejemplo:
//
//	configs := map[string]Config{
//	    "material.uploaded": {RequestsPerSecond: 5, BurstSize: 10},
//	    "assessment.attempt": {RequestsPerSecond: 15, BurstSize: 30},
//	}
//	defaultCfg := &Config{RequestsPerSecond: 10, BurstSize: 20}
//	limiter := NewMulti(configs, defaultCfg)
func NewMulti(configs map[string]Config, defaultConfig *Config) *MultiRateLimiter {
	limiters := make(map[string]*RateLimiter)

	for eventType, cfg := range configs {
		limiters[eventType] = New(cfg.RequestsPerSecond, cfg.BurstSize)
	}

	return &MultiRateLimiter{
		limiters:      limiters,
		defaultConfig: defaultConfig,
	}
}

// Allow verifica si hay tokens disponibles para el tipo de evento
// Retorna true si se puede proceder, false si se debe esperar
//
// Si no existe un rate limiter específico para el tipo de evento:
//   - Si hay configuración por defecto, la usa
//   - Si no hay configuración por defecto, permite la petición
func (m *MultiRateLimiter) Allow(eventType string) bool {
	limiter := m.getLimiter(eventType)
	if limiter == nil {
		// No hay rate limiter para este tipo, permitir
		return true
	}

	return limiter.Allow()
}

// Wait espera hasta que haya un token disponible para el tipo de evento
// Retorna nil cuando puede proceder, o error si el contexto se cancela
//
// Si no existe un rate limiter específico para el tipo de evento:
//   - Si hay configuración por defecto, la usa
//   - Si no hay configuración por defecto, retorna inmediatamente (sin límite)
func (m *MultiRateLimiter) Wait(ctx context.Context, eventType string) error {
	limiter := m.getLimiter(eventType)
	if limiter == nil {
		// No hay rate limiter para este tipo, permitir sin esperar
		return nil
	}

	return limiter.Wait(ctx)
}

// getLimiter obtiene el rate limiter para un tipo de evento
// Si no existe, crea uno usando la configuración por defecto (si está disponible)
func (m *MultiRateLimiter) getLimiter(eventType string) *RateLimiter {
	// Primero intentar lectura sin lock
	m.mu.RLock()
	limiter, exists := m.limiters[eventType]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	// No existe, verificar si debemos crear uno con config por defecto
	if m.defaultConfig == nil {
		return nil
	}

	// Adquirir write lock para crear el limiter
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check: otro goroutine pudo haberlo creado
	if limiter, exists := m.limiters[eventType]; exists {
		return limiter
	}

	// Crear nuevo limiter con configuración por defecto
	limiter = New(m.defaultConfig.RequestsPerSecond, m.defaultConfig.BurstSize)
	m.limiters[eventType] = limiter

	return limiter
}

// Tokens retorna el número de tokens disponibles para un tipo de evento
// Útil para debugging y métricas
func (m *MultiRateLimiter) Tokens(eventType string) float64 {
	limiter := m.getLimiter(eventType)
	if limiter == nil {
		return -1 // Indicar que no hay rate limiter
	}

	return limiter.Tokens()
}

// Reset reinicia el rate limiter de un tipo de evento específico
// Útil para testing o resetear límites
func (m *MultiRateLimiter) Reset(eventType string) {
	m.mu.RLock()
	limiter, exists := m.limiters[eventType]
	m.mu.RUnlock()

	if exists {
		limiter.Reset()
	}
}

// ResetAll reinicia todos los rate limiters
func (m *MultiRateLimiter) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, limiter := range m.limiters {
		limiter.Reset()
	}
}

// EventTypes retorna la lista de tipos de eventos configurados
func (m *MultiRateLimiter) EventTypes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	types := make([]string, 0, len(m.limiters))
	for eventType := range m.limiters {
		types = append(types, eventType)
	}

	return types
}

// HasLimiter verifica si existe un rate limiter para el tipo de evento
func (m *MultiRateLimiter) HasLimiter(eventType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.limiters[eventType]
	return exists
}
