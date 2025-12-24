package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Métricas de eventos procesados
var (
	// EventsProcessedTotal cuenta el total de eventos procesados por tipo y estado
	EventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_events_processed_total",
			Help: "Total number of events processed by type and status",
		},
		[]string{"event_type", "status"},
	)

	// ProcessingDuration mide la duración del procesamiento de eventos
	ProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_processing_duration_seconds",
			Help:    "Duration of event processing in seconds",
			Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
		},
		[]string{"event_type"},
	)

	// EventsInQueue mide el número de eventos en cola pendientes
	EventsInQueue = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_events_in_queue",
			Help: "Number of events currently in the queue",
		},
	)
)

// Métricas de OpenAI/NLP
var (
	// OpenAIRequestsTotal cuenta las solicitudes a OpenAI por estado
	OpenAIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_openai_requests_total",
			Help: "Total number of OpenAI API requests by status",
		},
		[]string{"status"}, // success, error, rate_limited, timeout
	)

	// OpenAILatency mide la latencia de las solicitudes a OpenAI
	OpenAILatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_openai_latency_seconds",
			Help:    "Latency of OpenAI API requests in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60}, // Buckets personalizados para APIs externas
		},
	)

	// OpenAITokensUsed cuenta el total de tokens consumidos
	OpenAITokensUsed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "worker_openai_tokens_used_total",
			Help: "Total number of tokens used in OpenAI requests",
		},
	)

	// OpenAIErrors cuenta los errores de OpenAI por tipo
	OpenAIErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_openai_errors_total",
			Help: "Total number of OpenAI errors by type",
		},
		[]string{"error_type"}, // rate_limit, timeout, server_error, invalid_request
	)
)

// Métricas de almacenamiento (S3)
var (
	// S3OperationsTotal cuenta las operaciones de S3 por tipo y estado
	S3OperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_s3_operations_total",
			Help: "Total number of S3 operations by type and status",
		},
		[]string{"operation", "status"}, // operation: download, upload, delete
	)

	// S3OperationDuration mide la duración de operaciones S3
	S3OperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_s3_operation_duration_seconds",
			Help:    "Duration of S3 operations in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"operation"},
	)
)

// Métricas de extracción de PDF
var (
	// PDFExtractionTotal cuenta las extracciones de PDF por estado
	PDFExtractionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_pdf_extraction_total",
			Help: "Total number of PDF extraction attempts by status",
		},
		[]string{"status"}, // success, error
	)

	// PDFExtractionDuration mide la duración de la extracción de PDF
	PDFExtractionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_pdf_extraction_duration_seconds",
			Help:    "Duration of PDF text extraction in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
	)

	// PDFPagesProcessed cuenta el número de páginas procesadas
	PDFPagesProcessed = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_pdf_pages_processed",
			Help:    "Number of pages processed per PDF",
			Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
		},
	)
)

// Métricas de base de datos
var (
	// DBOperationsTotal cuenta las operaciones de base de datos
	DBOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_db_operations_total",
			Help: "Total number of database operations by type and status",
		},
		[]string{"db_type", "operation", "status"}, // db_type: postgres, mongodb
	)

	// DBOperationDuration mide la duración de operaciones de DB
	DBOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_db_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"db_type", "operation"},
	)
)

// Métricas de circuit breaker
var (
	// CircuitBreakerState indica el estado del circuit breaker por servicio
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_circuit_breaker_state",
			Help: "Current state of circuit breakers (0=closed, 1=half-open, 2=open)",
		},
		[]string{"service"}, // openai, mongodb, postgres
	)

	// CircuitBreakerTransitions cuenta las transiciones de estado
	CircuitBreakerTransitions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_circuit_breaker_transitions_total",
			Help: "Total number of circuit breaker state transitions",
		},
		[]string{"service", "from_state", "to_state"},
	)
)

// Métricas de rate limiter
var (
	// RateLimiterWaits cuenta las veces que se esperó por rate limiter
	RateLimiterWaits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_rate_limiter_waits_total",
			Help: "Total number of times rate limiter caused waiting by event type",
		},
		[]string{"event_type"},
	)

	// RateLimiterWaitDuration mide el tiempo de espera por rate limiter
	RateLimiterWaitDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_rate_limiter_wait_duration_seconds",
			Help:    "Duration of rate limiter waits in seconds by event type",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"event_type"},
	)

	// RateLimiterTokens indica tokens disponibles por tipo de evento
	RateLimiterTokens = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_rate_limiter_tokens_available",
			Help: "Number of tokens currently available by event type",
		},
		[]string{"event_type"},
	)

	// RateLimiterAllowed cuenta requests permitidos por tipo de evento
	RateLimiterAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_rate_limiter_allowed_total",
			Help: "Total number of requests allowed by rate limiter by event type",
		},
		[]string{"event_type"},
	)
)

// RecordEventProcessing registra una métrica de procesamiento de evento
func RecordEventProcessing(eventType string, status string, durationSeconds float64) {
	EventsProcessedTotal.WithLabelValues(eventType, status).Inc()
	ProcessingDuration.WithLabelValues(eventType).Observe(durationSeconds)
}

// RecordOpenAIRequest registra una métrica de solicitud a OpenAI
func RecordOpenAIRequest(status string, durationSeconds float64, tokensUsed int) {
	OpenAIRequestsTotal.WithLabelValues(status).Inc()
	OpenAILatency.Observe(durationSeconds)
	if tokensUsed > 0 {
		OpenAITokensUsed.Add(float64(tokensUsed))
	}
}

// RecordOpenAIError registra un error de OpenAI
func RecordOpenAIError(errorType string) {
	OpenAIErrors.WithLabelValues(errorType).Inc()
}

// RecordS3Operation registra una operación de S3
func RecordS3Operation(operation string, status string, durationSeconds float64) {
	S3OperationsTotal.WithLabelValues(operation, status).Inc()
	S3OperationDuration.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordPDFExtraction registra una extracción de PDF
func RecordPDFExtraction(status string, durationSeconds float64, pageCount int) {
	PDFExtractionTotal.WithLabelValues(status).Inc()
	PDFExtractionDuration.Observe(durationSeconds)
	if pageCount > 0 {
		PDFPagesProcessed.Observe(float64(pageCount))
	}
}

// RecordDBOperation registra una operación de base de datos
func RecordDBOperation(dbType string, operation string, status string, durationSeconds float64) {
	DBOperationsTotal.WithLabelValues(dbType, operation, status).Inc()
	DBOperationDuration.WithLabelValues(dbType, operation).Observe(durationSeconds)
}

// SetCircuitBreakerState actualiza el estado del circuit breaker.
//
// Valores esperados para el parámetro service:
// - "nlp": Cliente de procesamiento de lenguaje natural (OpenAI/Fallback)
// - "storage": Cliente de almacenamiento (S3/MinIO)
//
// IMPORTANTE: No usar sufijos como "-test" en producción. Los sufijos solo
// deben usarse en tests unitarios para aislar las métricas de prueba.
func SetCircuitBreakerState(service string, state int) {
	CircuitBreakerState.WithLabelValues(service).Set(float64(state))
}

// RecordCircuitBreakerTransition registra una transición de estado del circuit breaker.
//
// Valores esperados para el parámetro service:
// - "nlp": Cliente de procesamiento de lenguaje natural (OpenAI/Fallback)
// - "storage": Cliente de almacenamiento (S3/MinIO)
//
// IMPORTANTE: No usar sufijos como "-test" en producción. Los sufijos solo
// deben usarse en tests unitarios para aislar las métricas de prueba.
func RecordCircuitBreakerTransition(service string, fromState string, toState string) {
	CircuitBreakerTransitions.WithLabelValues(service, fromState, toState).Inc()
}

// RecordEventProcessed registra un evento procesado con su estado.
//
// Deprecated: usar RecordEventProcessing cuando se disponga de la duración
// del procesamiento. No debe llamarse junto con RecordEventProcessing para
// el mismo evento, ya que produciría un doble conteo en la métrica
// worker_events_processed_total.
func RecordEventProcessed(eventType string, status string) {
	EventsProcessedTotal.WithLabelValues(eventType, status).Inc()
}

// RecordProcessingDuration registra la duración total de procesamiento de un evento
func RecordProcessingDuration(eventType string, durationSeconds float64) {
	ProcessingDuration.WithLabelValues(eventType).Observe(durationSeconds)
}

// RecordDatabaseOperation registra una operación de base de datos con éxito/fallo
func RecordDatabaseOperation(dbType string, operation string, durationSeconds float64, success bool) {
	status := "error"
	if success {
		status = "success"
	}
	RecordDBOperation(dbType, operation, status, durationSeconds)
}

// RecordRateLimiterWait registra una espera causada por rate limiter
func RecordRateLimiterWait(eventType string, durationSeconds float64) {
	RateLimiterWaits.WithLabelValues(eventType).Inc()
	RateLimiterWaitDuration.WithLabelValues(eventType).Observe(durationSeconds)
}

// RecordRateLimiterAllowed registra un request permitido por rate limiter
func RecordRateLimiterAllowed(eventType string) {
	RateLimiterAllowed.WithLabelValues(eventType).Inc()
}

// UpdateRateLimiterTokens actualiza la métrica de tokens disponibles
func UpdateRateLimiterTokens(eventType string, tokens float64) {
	RateLimiterTokens.WithLabelValues(eventType).Set(tokens)
}
