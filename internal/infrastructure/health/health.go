package health

import (
	"context"
	"time"
)

// Status representa el estado de salud de un componente
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// CheckResult representa el resultado de un health check
type CheckResult struct {
	Status    Status                 `json:"status"`
	Component string                 `json:"component"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthCheck representa un health check individual
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// Checker gestiona múltiples health checks
type Checker struct {
	checks []HealthCheck
}

// NewChecker crea un nuevo Checker
func NewChecker() *Checker {
	return &Checker{
		checks: make([]HealthCheck, 0),
	}
}

// Register registra un nuevo health check
func (c *Checker) Register(check HealthCheck) {
	c.checks = append(c.checks, check)
}

// CheckAll ejecuta todos los health checks registrados
func (c *Checker) CheckAll(ctx context.Context) map[string]CheckResult {
	results := make(map[string]CheckResult)
	for _, check := range c.checks {
		results[check.Name()] = check.Check(ctx)
	}
	return results
}

// IsHealthy retorna true si todos los health checks están healthy
func (c *Checker) IsHealthy(ctx context.Context) bool {
	results := c.CheckAll(ctx)
	for _, result := range results {
		if result.Status == StatusUnhealthy {
			return false
		}
	}
	return true
}

// IsReady retorna true si todos los health checks están healthy (para readiness)
func (c *Checker) IsReady(ctx context.Context) bool {
	return c.IsHealthy(ctx)
}

// IsLive retorna true si la aplicación está viva (liveness básica)
func (c *Checker) IsLive() bool {
	// Una aplicación está viva si puede responder
	return true
}
