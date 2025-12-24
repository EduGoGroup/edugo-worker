package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer maneja el servidor HTTP para exponer métricas de Prometheus
type MetricsServer struct {
	server *http.Server
	port   int
}

// NewMetricsServer crea una nueva instancia del servidor de métricas
func NewMetricsServer(port int) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Endpoint de health check simple para el servidor de métricas
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	return &MetricsServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		port: port,
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
