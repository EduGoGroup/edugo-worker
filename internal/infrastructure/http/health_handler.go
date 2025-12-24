package http

import (
	"encoding/json"
	"net/http"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/health"
)

// HealthHandler maneja los endpoints de health checks
type HealthHandler struct {
	checker *health.Checker
}

// NewHealthHandler crea un nuevo handler de health checks
func NewHealthHandler(checker *health.Checker) *HealthHandler {
	return &HealthHandler{
		checker: checker,
	}
}

// HealthResponse representa la respuesta de health check
type HealthResponse struct {
	Status  string                        `json:"status"`
	Checks  map[string]health.CheckResult `json:"checks,omitempty"`
	Message string                        `json:"message,omitempty"`
}

// Health maneja GET /health - health check completo
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results := h.checker.CheckAll(ctx)
	isHealthy := h.checker.IsHealthy(ctx)

	status := "healthy"
	statusCode := http.StatusOK
	if !isHealthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	response := HealthResponse{
		Status: status,
		Checks: results,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// Liveness maneja GET /health/live - liveness probe (Kubernetes)
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	isLive := h.checker.IsLive()

	statusCode := http.StatusOK
	status := "alive"
	if !isLive {
		statusCode = http.StatusServiceUnavailable
		status = "dead"
	}

	response := HealthResponse{
		Status:  status,
		Message: "Application is running",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// Readiness maneja GET /health/ready - readiness probe (Kubernetes)
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results := h.checker.CheckAll(ctx)
	isReady := h.checker.IsReady(ctx)

	status := "ready"
	statusCode := http.StatusOK
	message := "Application is ready to serve traffic"

	if !isReady {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
		message = "Application is not ready to serve traffic"
	}

	response := HealthResponse{
		Status:  status,
		Checks:  results,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}
