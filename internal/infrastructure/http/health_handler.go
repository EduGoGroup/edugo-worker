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
// Implementa lógica granular: solo componentes críticos (MongoDB, PostgreSQL) afectan el estado ready
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results := h.checker.CheckAll(ctx)

	// Componentes críticos que deben estar saludables para que el servicio esté "ready"
	criticalComponents := map[string]bool{
		"mongodb":    true,
		"postgresql": true,
	}

	hasCriticalFailure := false
	hasOptionalFailure := false

	for componentName, result := range results {
		if result.Status != health.StatusHealthy {
			if criticalComponents[componentName] {
				hasCriticalFailure = true
			} else {
				hasOptionalFailure = true
			}
		}
	}

	status := "ready"
	statusCode := http.StatusOK
	message := "Application is ready to serve traffic"

	if hasCriticalFailure {
		// Si hay falla crítica, el servicio NO está ready
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
		message = "Application is not ready: critical components are unhealthy"
	} else if hasOptionalFailure {
		// Si solo hay fallas opcionales, el servicio está ready pero degradado
		status = "ready_degraded"
		statusCode = http.StatusOK
		message = "Application is ready but some optional components are unhealthy"
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
