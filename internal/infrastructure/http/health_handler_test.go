package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHealthCheck es un health check de prueba configurable
type MockHealthCheck struct {
	name    string
	status  health.Status
	message string
}

func (m *MockHealthCheck) Name() string {
	return m.name
}

func (m *MockHealthCheck) Check(ctx context.Context) health.CheckResult {
	return health.CheckResult{
		Status:    m.status,
		Component: m.name,
		Message:   m.message,
		Timestamp: time.Now(),
	}
}

func TestHealthHandler_Health_AllHealthy(t *testing.T) {
	// Arrange
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusHealthy,
		message: "MongoDB is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusHealthy,
		message: "PostgreSQL is healthy",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Health(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Len(t, response.Checks, 2)
	assert.Equal(t, health.StatusHealthy, response.Checks["mongodb"].Status)
	assert.Equal(t, health.StatusHealthy, response.Checks["postgresql"].Status)
}

func TestHealthHandler_Health_SomeUnhealthy(t *testing.T) {
	// Arrange
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusUnhealthy,
		message: "MongoDB connection failed",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusHealthy,
		message: "PostgreSQL is healthy",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Health(rec, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response.Status)
	assert.Len(t, response.Checks, 2)
	assert.Equal(t, health.StatusUnhealthy, response.Checks["mongodb"].Status)
}

func TestHealthHandler_Liveness(t *testing.T) {
	// Arrange
	checker := health.NewChecker()
	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Liveness(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "alive", response.Status)
	assert.Equal(t, "Application is running", response.Message)
}

func TestHealthHandler_Readiness_AllHealthy(t *testing.T) {
	// Arrange
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusHealthy,
		message: "MongoDB is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusHealthy,
		message: "PostgreSQL is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "rabbitmq",
		status:  health.StatusHealthy,
		message: "RabbitMQ is healthy",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Readiness(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ready", response.Status)
	assert.Equal(t, "Application is ready to serve traffic", response.Message)
	assert.Len(t, response.Checks, 3)
}

func TestHealthHandler_Readiness_CriticalComponentUnhealthy(t *testing.T) {
	// Arrange - MongoDB (crítico) está degradado
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusUnhealthy,
		message: "MongoDB connection failed",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusHealthy,
		message: "PostgreSQL is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "rabbitmq",
		status:  health.StatusHealthy,
		message: "RabbitMQ is healthy",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Readiness(rec, req)

	// Assert - Debe retornar 503 porque un componente crítico está degradado
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "Application is not ready: critical components are unhealthy", response.Message)
}

func TestHealthHandler_Readiness_OnlyOptionalComponentUnhealthy(t *testing.T) {
	// Arrange - Solo RabbitMQ (opcional) está degradado
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusHealthy,
		message: "MongoDB is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusHealthy,
		message: "PostgreSQL is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "rabbitmq",
		status:  health.StatusUnhealthy,
		message: "RabbitMQ connection failed",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Readiness(rec, req)

	// Assert - Debe retornar 200 porque solo componentes opcionales están degradados
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ready_degraded", response.Status)
	assert.Equal(t, "Application is ready but some optional components are unhealthy", response.Message)
}

func TestHealthHandler_Readiness_PostgreSQLCriticalUnhealthy(t *testing.T) {
	// Arrange - PostgreSQL (crítico) está degradado
	checker := health.NewChecker()
	checker.Register(&MockHealthCheck{
		name:    "mongodb",
		status:  health.StatusHealthy,
		message: "MongoDB is healthy",
	})
	checker.Register(&MockHealthCheck{
		name:    "postgresql",
		status:  health.StatusUnhealthy,
		message: "PostgreSQL connection timeout",
	})

	handler := NewHealthHandler(checker)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.Readiness(rec, req)

	// Assert - Debe retornar 503 porque PostgreSQL es crítico
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "not_ready", response.Status)
}
