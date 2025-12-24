package ratelimiter

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implementa el algoritmo Token Bucket para rate limiting
type RateLimiter struct {
	tokens         float64    // Tokens disponibles actualmente
	maxTokens      float64    // Capacidad máxima del bucket (burst size)
	refillRate     float64    // Tokens agregados por segundo
	lastRefillTime time.Time  // Última vez que se rellenaron tokens
	mu             sync.Mutex // Protección contra condiciones de carrera
}

// New crea un nuevo rate limiter con algoritmo Token Bucket
//
// Parámetros:
//   - requestsPerSecond: número de requests permitidos por segundo
//   - burstSize: número máximo de requests en ráfaga (capacidad del bucket)
//
// Ejemplo:
//   - requestsPerSecond=10, burstSize=20
//   - Permite 10 requests/segundo sostenido
//   - Permite ráfagas de hasta 20 requests si hay tokens acumulados
func New(requestsPerSecond float64, burstSize float64) *RateLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	if burstSize <= 0 {
		burstSize = 1
	}

	return &RateLimiter{
		tokens:         burstSize, // Iniciar con bucket lleno
		maxTokens:      burstSize,
		refillRate:     requestsPerSecond,
		lastRefillTime: time.Now(),
	}
}

// Allow verifica si hay tokens disponibles y consume uno si existe
// Retorna true si se puede proceder, false si se debe esperar
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Rellenar tokens basado en tiempo transcurrido
	rl.refill()

	// Verificar si hay al menos 1 token disponible
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// Wait espera hasta que haya un token disponible o el contexto se cancele
// Retorna nil cuando puede proceder, o error si el contexto se cancela
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		// Esperar un poco antes de reintentar
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Continuar el loop
		}
	}
}

// refill agrega tokens al bucket basado en el tiempo transcurrido
// Debe llamarse con el mutex adquirido
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefillTime).Seconds()

	// Calcular tokens a agregar basado en tiempo transcurrido
	tokensToAdd := elapsed * rl.refillRate

	// Actualizar tokens sin exceder el máximo
	rl.tokens += tokensToAdd
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastRefillTime = now
}

// Tokens retorna el número de tokens disponibles actualmente
// Útil para debugging y métricas
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()
	return rl.tokens
}

// Reset reinicia el rate limiter a su estado inicial (bucket lleno)
// Útil para testing o reiniciar límites
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokens = rl.maxTokens
	rl.lastRefillTime = time.Now()
}
