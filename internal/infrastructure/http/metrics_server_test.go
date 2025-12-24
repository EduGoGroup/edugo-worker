package http

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForServerReady espera a que el servidor esté listo usando retry con backoff
func waitForServerReady(t *testing.T, url string, maxAttempts int, interval time.Duration) {
	t.Helper()

	for i := 0; i < maxAttempts; i++ {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}

		if i < maxAttempts-1 {
			time.Sleep(interval)
		}
	}

	t.Fatalf("servidor no estuvo listo después de %d intentos", maxAttempts)
}

func TestNewMetricsServer(t *testing.T) {
	// Arrange & Act
	server := NewMetricsServer(9090)

	// Assert
	assert.NotNil(t, server)
	assert.Equal(t, 9090, server.Port())
}

func TestMetricsServer_MetricsEndpoint(t *testing.T) {
	// Arrange
	server := NewMetricsServer(19090) // Puerto de test diferente

	go func() {
		_ = server.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// Esperar a que el servidor esté listo con retry
	waitForServerReady(t, "http://localhost:19090/health", 10, 50*time.Millisecond)

	// Act
	resp, err := http.Get("http://localhost:19090/metrics")

	// Assert
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verificar que hay contenido de Prometheus
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "# HELP")
	assert.Contains(t, bodyStr, "# TYPE")
}

func TestMetricsServer_HealthEndpoint(t *testing.T) {
	// Arrange
	server := NewMetricsServer(19091) // Puerto de test diferente

	go func() {
		_ = server.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// Esperar a que el servidor esté listo con retry
	waitForServerReady(t, "http://localhost:19091/health", 10, 50*time.Millisecond)

	// Act
	resp, err := http.Get("http://localhost:19091/health")

	// Assert
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "OK", string(body))
}

func TestMetricsServer_Shutdown(t *testing.T) {
	// Arrange
	server := NewMetricsServer(19092) // Puerto de test diferente

	go func() {
		_ = server.Start()
	}()

	// Esperar a que el servidor esté listo con retry
	waitForServerReady(t, "http://localhost:19092/health", 10, 50*time.Millisecond)

	// Act
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)

	// Assert
	assert.NoError(t, err)

	// Verificar que el servidor ya no responde
	_, err = http.Get("http://localhost:19092/health")
	assert.Error(t, err)
}
