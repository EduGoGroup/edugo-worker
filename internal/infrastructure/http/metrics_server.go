package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer maneja el servidor HTTP para exponer métricas de Prometheus
type MetricsServer struct {
	server *http.Server
	port   int
}

// MetricsServerConfig configuración del servidor de métricas
type MetricsServerConfig struct {
	Port          int
	HealthChecker *health.Checker
}

// NewMetricsServer crea una nueva instancia del servidor de métricas
func NewMetricsServer(port int) *MetricsServer {
	return NewMetricsServerWithConfig(MetricsServerConfig{Port: port})
}

// NewMetricsServerWithConfig crea una nueva instancia con configuración completa
func NewMetricsServerWithConfig(cfg MetricsServerConfig) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Si se proporciona health checker, registrar endpoints de health
	if cfg.HealthChecker != nil {
		healthHandler := NewHealthHandler(cfg.HealthChecker)
		mux.HandleFunc("/health", healthHandler.Health)
		mux.HandleFunc("/health/live", healthHandler.Liveness)
		mux.HandleFunc("/health/ready", healthHandler.Readiness)
	} else {
		// Endpoint de health check simple para el servidor de métricas
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})
	}

	return &MetricsServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		port: cfg.Port,
	}
}

// Start inicia el servidor de métricas
func (s *MetricsServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown detiene el servidor de métricas de forma ordenada
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Port retorna el puerto en el que está escuchando el servidor
func (s *MetricsServer) Port() int {
	return s.port
}
