package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getCounterValue extrae el valor actual de un counter con labels específicos
func getCounterValue(t *testing.T, counter *prometheus.CounterVec, labels prometheus.Labels) float64 {
	metric := &dto.Metric{}
	err := counter.With(labels).(prometheus.Counter).Write(metric)
	require.NoError(t, err, "Debería poder leer el counter")
	return metric.Counter.GetValue()
}

// getGaugeValue extrae el valor actual de un gauge con labels específicos
func getGaugeValue(t *testing.T, gauge *prometheus.GaugeVec, labels prometheus.Labels) float64 {
	metric := &dto.Metric{}
	err := gauge.With(labels).(prometheus.Gauge).Write(metric)
	require.NoError(t, err, "Debería poder leer el gauge")
	return metric.Gauge.GetValue()
}

// getHistogramCount extrae el número de observaciones de un histogram
func getHistogramCount(t *testing.T, hist *prometheus.HistogramVec, labels prometheus.Labels) uint64 {
	metric := &dto.Metric{}
	err := hist.With(labels).(prometheus.Histogram).Write(metric)
	require.NoError(t, err, "Debería poder leer el histogram")
	return metric.Histogram.GetSampleCount()
}

func TestRecordRateLimiterWait(t *testing.T) {
	eventType := "test_event_wait"
	duration := 0.5

	// Obtener valor inicial
	initialWaits := getCounterValue(t, RateLimiterWaits, prometheus.Labels{"event_type": eventType})
	initialHistCount := getHistogramCount(t, RateLimiterWaitDuration, prometheus.Labels{"event_type": eventType})

	// Registrar espera
	RecordRateLimiterWait(eventType, duration)

	// Verificar incremento en contador
	newWaits := getCounterValue(t, RateLimiterWaits, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, initialWaits+1, newWaits, "El contador de esperas debería incrementar en 1")

	// Verificar que se registró la observación en el histogram
	newHistCount := getHistogramCount(t, RateLimiterWaitDuration, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, initialHistCount+1, newHistCount, "El histogram debería tener una observación adicional")
}

func TestRecordRateLimiterAllowed(t *testing.T) {
	eventType := "test_event_allowed"

	// Obtener valor inicial
	initialAllowed := getCounterValue(t, RateLimiterAllowed, prometheus.Labels{"event_type": eventType})

	// Registrar requests permitidos
	RecordRateLimiterAllowed(eventType)
	RecordRateLimiterAllowed(eventType)
	RecordRateLimiterAllowed(eventType)

	// Verificar incremento
	newAllowed := getCounterValue(t, RateLimiterAllowed, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, initialAllowed+3, newAllowed, "El contador de allowed debería incrementar en 3")
}

func TestUpdateRateLimiterTokens(t *testing.T) {
	eventType := "test_event_tokens"

	// Actualizar tokens disponibles
	UpdateRateLimiterTokens(eventType, 50.0)
	tokens1 := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, 50.0, tokens1, "Los tokens deberían ser 50.0")

	// Actualizar nuevamente
	UpdateRateLimiterTokens(eventType, 25.5)
	tokens2 := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, 25.5, tokens2, "Los tokens deberían actualizarse a 25.5")

	// Actualizar a cero
	UpdateRateLimiterTokens(eventType, 0)
	tokens3 := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, 0.0, tokens3, "Los tokens deberían ser 0")
}

func TestRateLimiterMetrics_Integration(t *testing.T) {
	eventType := "integration_test_event"

	// Escenario: Simular procesamiento con rate limiting

	// 1. Estado inicial - tokens llenos
	UpdateRateLimiterTokens(eventType, 100.0)
	tokens := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, 100.0, tokens, "Deberían haber 100 tokens disponibles")

	// 2. Procesar varios requests permitidos
	for i := 0; i < 5; i++ {
		RecordRateLimiterAllowed(eventType)
	}
	allowed := getCounterValue(t, RateLimiterAllowed, prometheus.Labels{"event_type": eventType})
	assert.GreaterOrEqual(t, allowed, 5.0, "Deberían haberse registrado al menos 5 requests permitidos")

	// 3. Simular que se agotan tokens - causar esperas
	UpdateRateLimiterTokens(eventType, 2.0)
	RecordRateLimiterWait(eventType, 0.1)
	RecordRateLimiterWait(eventType, 0.2)

	waits := getCounterValue(t, RateLimiterWaits, prometheus.Labels{"event_type": eventType})
	assert.GreaterOrEqual(t, waits, 2.0, "Deberían haberse registrado al menos 2 esperas")

	waitCount := getHistogramCount(t, RateLimiterWaitDuration, prometheus.Labels{"event_type": eventType})
	assert.GreaterOrEqual(t, waitCount, uint64(2), "Debería haber al menos 2 observaciones de duración")

	// 4. Recuperar tokens
	UpdateRateLimiterTokens(eventType, 50.0)
	tokensRecovered := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, 50.0, tokensRecovered, "Los tokens deberían recuperarse a 50")
}

func TestRecordRateLimiterWait_MultiplesEventTypes(t *testing.T) {
	// Verificar que diferentes event types mantienen métricas independientes
	eventType1 := "event_type_1"
	eventType2 := "event_type_2"

	// Registrar esperas en diferentes tipos
	RecordRateLimiterWait(eventType1, 0.5)
	RecordRateLimiterWait(eventType1, 0.3)
	RecordRateLimiterWait(eventType2, 0.8)

	// Verificar que los contadores son independientes
	waits1 := getCounterValue(t, RateLimiterWaits, prometheus.Labels{"event_type": eventType1})
	waits2 := getCounterValue(t, RateLimiterWaits, prometheus.Labels{"event_type": eventType2})

	assert.GreaterOrEqual(t, waits1, 2.0, "event_type_1 debería tener al menos 2 esperas")
	assert.GreaterOrEqual(t, waits2, 1.0, "event_type_2 debería tener al menos 1 espera")
}

func TestUpdateRateLimiterTokens_ValoresNegativos(t *testing.T) {
	eventType := "test_negative_tokens"

	// Prometheus permite valores negativos en gauges
	UpdateRateLimiterTokens(eventType, -10.0)
	tokens := getGaugeValue(t, RateLimiterTokens, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, -10.0, tokens, "El gauge debería aceptar valores negativos")
}

func TestRecordEventProcessing(t *testing.T) {
	eventType := "test_processing"
	status := "success"
	duration := 1.5

	initialCount := getCounterValue(t, EventsProcessedTotal, prometheus.Labels{
		"event_type": eventType,
		"status":     status,
	})
	initialHistCount := getHistogramCount(t, ProcessingDuration, prometheus.Labels{"event_type": eventType})

	RecordEventProcessing(eventType, status, duration)

	newCount := getCounterValue(t, EventsProcessedTotal, prometheus.Labels{
		"event_type": eventType,
		"status":     status,
	})
	newHistCount := getHistogramCount(t, ProcessingDuration, prometheus.Labels{"event_type": eventType})

	assert.Equal(t, initialCount+1, newCount, "El contador de eventos procesados debería incrementar")
	assert.Equal(t, initialHistCount+1, newHistCount, "El histogram de duración debería tener una observación adicional")
}

func TestRecordOpenAIRequest(t *testing.T) {
	status := "success"
	duration := 2.5
	tokens := 500

	initialRequests := getCounterValue(t, OpenAIRequestsTotal, prometheus.Labels{"status": status})

	RecordOpenAIRequest(status, duration, tokens)

	newRequests := getCounterValue(t, OpenAIRequestsTotal, prometheus.Labels{"status": status})
	assert.Equal(t, initialRequests+1, newRequests, "Las requests de OpenAI deberían incrementar")
}

func TestRecordOpenAIError(t *testing.T) {
	errorType := "rate_limit"

	initialErrors := getCounterValue(t, OpenAIErrors, prometheus.Labels{"error_type": errorType})

	RecordOpenAIError(errorType)

	newErrors := getCounterValue(t, OpenAIErrors, prometheus.Labels{"error_type": errorType})
	assert.Equal(t, initialErrors+1, newErrors, "Los errores de OpenAI deberían incrementar")
}

func TestRecordS3Operation(t *testing.T) {
	operation := "upload"
	status := "success"
	duration := 3.0

	initialOps := getCounterValue(t, S3OperationsTotal, prometheus.Labels{
		"operation": operation,
		"status":    status,
	})

	RecordS3Operation(operation, status, duration)

	newOps := getCounterValue(t, S3OperationsTotal, prometheus.Labels{
		"operation": operation,
		"status":    status,
	})
	assert.Equal(t, initialOps+1, newOps, "Las operaciones de S3 deberían incrementar")
}

func TestRecordPDFExtraction(t *testing.T) {
	status := "success"
	duration := 5.0
	pageCount := 10

	initialExtractions := getCounterValue(t, PDFExtractionTotal, prometheus.Labels{"status": status})

	RecordPDFExtraction(status, duration, pageCount)

	newExtractions := getCounterValue(t, PDFExtractionTotal, prometheus.Labels{"status": status})
	assert.Equal(t, initialExtractions+1, newExtractions, "Las extracciones de PDF deberían incrementar")
}

func TestRecordDBOperation(t *testing.T) {
	dbType := "postgres"
	operation := "select"
	status := "success"
	duration := 0.05

	initialOps := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    status,
	})

	RecordDBOperation(dbType, operation, status, duration)

	newOps := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    status,
	})
	assert.Equal(t, initialOps+1, newOps, "Las operaciones de DB deberían incrementar")
}

func TestSetCircuitBreakerState(t *testing.T) {
	service := "nlp-test-cb"

	// Estado closed (0)
	SetCircuitBreakerState(service, 0)
	state := getGaugeValue(t, CircuitBreakerState, prometheus.Labels{"service": service})
	assert.Equal(t, 0.0, state, "El estado debería ser closed (0)")

	// Estado half-open (1)
	SetCircuitBreakerState(service, 1)
	state = getGaugeValue(t, CircuitBreakerState, prometheus.Labels{"service": service})
	assert.Equal(t, 1.0, state, "El estado debería ser half-open (1)")

	// Estado open (2)
	SetCircuitBreakerState(service, 2)
	state = getGaugeValue(t, CircuitBreakerState, prometheus.Labels{"service": service})
	assert.Equal(t, 2.0, state, "El estado debería ser open (2)")
}

func TestRecordCircuitBreakerTransition(t *testing.T) {
	service := "storage-test-cb"
	fromState := "closed"
	toState := "open"

	initialTransitions := getCounterValue(t, CircuitBreakerTransitions, prometheus.Labels{
		"service":    service,
		"from_state": fromState,
		"to_state":   toState,
	})

	RecordCircuitBreakerTransition(service, fromState, toState)

	newTransitions := getCounterValue(t, CircuitBreakerTransitions, prometheus.Labels{
		"service":    service,
		"from_state": fromState,
		"to_state":   toState,
	})
	assert.Equal(t, initialTransitions+1, newTransitions, "Las transiciones deberían incrementar")
}

func TestRecordDatabaseOperation(t *testing.T) {
	dbType := "mongodb"
	operation := "insert"
	duration := 0.1

	// Test con éxito
	initialSuccess := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    "success",
	})

	RecordDatabaseOperation(dbType, operation, duration, true)

	newSuccess := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    "success",
	})
	assert.Equal(t, initialSuccess+1, newSuccess, "Las operaciones exitosas deberían incrementar")

	// Test con error
	initialError := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    "error",
	})

	RecordDatabaseOperation(dbType, operation, duration, false)

	newError := getCounterValue(t, DBOperationsTotal, prometheus.Labels{
		"db_type":   dbType,
		"operation": operation,
		"status":    "error",
	})
	assert.Equal(t, initialError+1, newError, "Las operaciones con error deberían incrementar")
}
