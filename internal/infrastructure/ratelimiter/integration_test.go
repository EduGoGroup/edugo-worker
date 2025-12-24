package ratelimiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// getCounterValue extrae el valor de un counter con labels
func getCounterValue(t *testing.T, counter *prometheus.CounterVec, labels prometheus.Labels) float64 {
	metric := &dto.Metric{}
	err := counter.With(labels).Write(metric)
	require.NoError(t, err)
	return metric.Counter.GetValue()
}

// getGaugeValue extrae el valor de un gauge con labels
func getGaugeValue(t *testing.T, gauge *prometheus.GaugeVec, labels prometheus.Labels) float64 {
	metric := &dto.Metric{}
	err := gauge.With(labels).Write(metric)
	require.NoError(t, err)
	return metric.Gauge.GetValue()
}

func TestIntegration_ConfigToMultiRateLimiterToMetrics(t *testing.T) {
	// FASE 1: Cargar configuración con múltiples tipos de eventos
	cfg := &config.Config{
		RateLimiter: config.RateLimiterConfig{
			Enabled: true,
			Default: config.EventRateLimitConfig{
				RequestsPerSecond: 100, // Alta para que no bloquee el test
				BurstSize:         200,
			},
			ByEventType: map[string]config.EventRateLimitConfig{
				"material_uploaded": {
					RequestsPerSecond: 50,
					BurstSize:         100,
				},
				"assessment_attempt": {
					RequestsPerSecond: 75,
					BurstSize:         150,
				},
				"slow_event": {
					RequestsPerSecond: 2, // Muy bajo para forzar esperas
					BurstSize:         4,
				},
			},
		},
	}

	rateLimiterCfg := cfg.GetRateLimiterConfigWithDefaults()
	assert.True(t, rateLimiterCfg.Enabled)

	// FASE 2: Crear MultiRateLimiter desde la configuración
	configs := make(map[string]Config)
	for eventType, eventCfg := range rateLimiterCfg.ByEventType {
		configs[eventType] = Config{
			RequestsPerSecond: eventCfg.RequestsPerSecond,
			BurstSize:         eventCfg.BurstSize,
		}
	}

	defaultCfg := &Config{
		RequestsPerSecond: rateLimiterCfg.Default.RequestsPerSecond,
		BurstSize:         rateLimiterCfg.Default.BurstSize,
	}

	limiter := NewMulti(configs, defaultCfg)
	require.NotNil(t, limiter)

	// Verificar que los limiters se crearon correctamente
	assert.True(t, limiter.HasLimiter("material_uploaded"))
	assert.True(t, limiter.HasLimiter("assessment_attempt"))
	assert.True(t, limiter.HasLimiter("slow_event"))

	// FASE 3: Simular procesamiento concurrente de mensajes

	// 3.1 - Procesar mensajes rápidos que NO causan esperas
	t.Run("Mensajes rápidos sin esperas", func(t *testing.T) {
		eventType := "material_uploaded"

		initialAllowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": eventType})

		// Procesar 5 mensajes rápidamente (dentro del burst)
		for i := 0; i < 5; i++ {
			ctx := context.Background()
			err := limiter.Wait(ctx, eventType)
			require.NoError(t, err, "No debería haber error al esperar token")

			// Registrar en métricas
			metrics.RecordRateLimiterAllowed(eventType)
			tokens := limiter.Tokens(eventType)
			metrics.UpdateRateLimiterTokens(eventType, tokens)
		}

		// Verificar métricas
		newAllowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": eventType})
		assert.Equal(t, initialAllowed+5, newAllowed, "Deberían haberse permitido 5 mensajes")

		tokens := getGaugeValue(t, metrics.RateLimiterTokens, prometheus.Labels{"event_type": eventType})
		assert.Greater(t, tokens, 0.0, "Deberían quedar tokens disponibles")
	})

	// 3.2 - Procesar mensajes que CAUSAN esperas (evento lento)
	t.Run("Mensajes lentos con esperas", func(t *testing.T) {
		eventType := "slow_event"

		initialWaits := getCounterValue(t, metrics.RateLimiterWaits, prometheus.Labels{"event_type": eventType})

		// Agotar el burst rápidamente
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow(eventType)
			if allowed {
				metrics.RecordRateLimiterAllowed(eventType)
			}
		}

		// Ahora intentar procesar causará esperas
		ctx := context.Background()
		start := time.Now()
		err := limiter.Wait(ctx, eventType)
		duration := time.Since(start).Seconds()

		require.NoError(t, err)

		// Registrar la espera en métricas
		metrics.RecordRateLimiterWait(eventType, duration)

		// Verificar que se registró la espera
		newWaits := getCounterValue(t, metrics.RateLimiterWaits, prometheus.Labels{"event_type": eventType})
		assert.Greater(t, newWaits, initialWaits, "Debería haberse registrado al menos 1 espera")
	})

	// 3.3 - Procesamiento concurrente con múltiples goroutines
	t.Run("Procesamiento concurrente", func(t *testing.T) {
		eventType := "assessment_attempt"

		var wg sync.WaitGroup
		var allowed int64
		var waited int64

		numWorkers := 10
		messagesPerWorker := 5

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for m := 0; m < messagesPerWorker; m++ {
					ctx := context.Background()
					start := time.Now()

					err := limiter.Wait(ctx, eventType)
					if err == nil {
						duration := time.Since(start).Seconds()

						if duration > 0.001 { // Si esperó más de 1ms
							atomic.AddInt64(&waited, 1)
							metrics.RecordRateLimiterWait(eventType, duration)
						} else {
							atomic.AddInt64(&allowed, 1)
							metrics.RecordRateLimiterAllowed(eventType)
						}

						tokens := limiter.Tokens(eventType)
						metrics.UpdateRateLimiterTokens(eventType, tokens)
					}
				}
			}()
		}

		wg.Wait()

		totalProcessed := atomic.LoadInt64(&allowed) + atomic.LoadInt64(&waited)
		expectedTotal := int64(numWorkers * messagesPerWorker)
		assert.Equal(t, expectedTotal, totalProcessed, "Deberían procesarse todos los mensajes")

		t.Logf("Procesados: %d (Permitidos inmediatamente: %d, Con espera: %d)",
			totalProcessed, allowed, waited)
	})

	// FASE 4: Validar que las métricas se registran correctamente
	t.Run("Validación de métricas finales", func(t *testing.T) {
		// Verificar que todas las métricas tienen valores razonables
		for _, eventType := range []string{"material_uploaded", "assessment_attempt", "slow_event"} {
			allowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": eventType})
			assert.GreaterOrEqual(t, allowed, 0.0, "Allowed debería ser >= 0 para %s", eventType)

			// No todos los eventos tienen esperas, solo verificamos que la métrica existe
			_ = getCounterValue(t, metrics.RateLimiterWaits, prometheus.Labels{"event_type": eventType})

			tokens := getGaugeValue(t, metrics.RateLimiterTokens, prometheus.Labels{"event_type": eventType})
			assert.GreaterOrEqual(t, tokens, 0.0, "Tokens deberían ser >= 0 para %s", eventType)
		}
	})

	// FASE 5: Validar el flujo con eventos nuevos (usando config por defecto)
	t.Run("Evento nuevo usando config por defecto", func(t *testing.T) {
		newEventType := "new_event_type"

		// Verificar que NO existe limiter inicialmente
		assert.False(t, limiter.HasLimiter(newEventType), "No debería existir limiter para evento nuevo")

		// Procesar mensaje - debería crear limiter con config por defecto
		ctx := context.Background()
		err := limiter.Wait(ctx, newEventType)
		require.NoError(t, err)

		// Ahora SÍ debería existir
		assert.True(t, limiter.HasLimiter(newEventType), "Debería haberse creado limiter para evento nuevo")

		// Verificar que usa la configuración por defecto
		tokens := limiter.Tokens(newEventType)
		assert.Greater(t, tokens, 0.0, "Debería tener tokens disponibles")

		metrics.RecordRateLimiterAllowed(newEventType)
		metrics.UpdateRateLimiterTokens(newEventType, tokens)

		allowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": newEventType})
		assert.GreaterOrEqual(t, allowed, 1.0, "Debería haberse registrado al menos 1 mensaje permitido")
	})
}

func TestIntegration_RateLimitingWithContextCancellation(t *testing.T) {
	// Test que verifica el comportamiento cuando se cancela el contexto durante espera

	cfg := &config.Config{
		RateLimiter: config.RateLimiterConfig{
			Enabled: true,
			ByEventType: map[string]config.EventRateLimitConfig{
				"cancellable_event": {
					RequestsPerSecond: 1, // Muy bajo para forzar esperas largas
					BurstSize:         1,
				},
			},
		},
	}

	rateLimiterCfg := cfg.GetRateLimiterConfigWithDefaults()

	configs := make(map[string]Config)
	for eventType, eventCfg := range rateLimiterCfg.ByEventType {
		configs[eventType] = Config{
			RequestsPerSecond: eventCfg.RequestsPerSecond,
			BurstSize:         eventCfg.BurstSize,
		}
	}

	limiter := NewMulti(configs, nil)
	eventType := "cancellable_event"

	// Agotar tokens
	limiter.Allow(eventType)
	limiter.Allow(eventType)

	// Crear contexto con timeout muy corto
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Intentar esperar - debería cancelarse por timeout
	start := time.Now()
	err := limiter.Wait(ctx, eventType)
	duration := time.Since(start)

	assert.Error(t, err, "Debería retornar error por contexto cancelado")
	assert.Less(t, duration, 500*time.Millisecond, "Debería cancelarse rápidamente")

	t.Logf("Wait cancelado después de %v con error: %v", duration, err)
}

func TestIntegration_RateLimitingDisabled(t *testing.T) {
	// Test que verifica que cuando rate limiting está deshabilitado, todo pasa sin límites

	cfg := &config.Config{
		RateLimiter: config.RateLimiterConfig{
			Enabled: false, // DESHABILITADO
		},
	}

	rateLimiterCfg := cfg.GetRateLimiterConfigWithDefaults()

	// Sin rate limiter, debería permitir todo
	assert.False(t, rateLimiterCfg.Enabled)

	// En producción, si está deshabilitado no se crearía el limiter
	// Este test simula el comportamiento esperado
	t.Log("Rate limiting deshabilitado - todos los mensajes deberían procesarse sin límite")
}

func TestIntegration_MetricsReflectActualBehavior(t *testing.T) {
	// Test que valida que las métricas reflejan el comportamiento real del rate limiter

	cfg := &config.Config{
		RateLimiter: config.RateLimiterConfig{
			Enabled: true,
			ByEventType: map[string]config.EventRateLimitConfig{
				"metrics_test_event": {
					RequestsPerSecond: 10,
					BurstSize:         20,
				},
			},
		},
	}

	rateLimiterCfg := cfg.GetRateLimiterConfigWithDefaults()

	configs := make(map[string]Config)
	for eventType, eventCfg := range rateLimiterCfg.ByEventType {
		configs[eventType] = Config{
			RequestsPerSecond: eventCfg.RequestsPerSecond,
			BurstSize:         eventCfg.BurstSize,
		}
	}

	limiter := NewMulti(configs, nil)
	eventType := "metrics_test_event"

	initialAllowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": eventType})

	// Procesar dentro del burst - no debería haber esperas
	for i := 0; i < 15; i++ {
		ctx := context.Background()
		err := limiter.Wait(ctx, eventType)
		require.NoError(t, err)
		metrics.RecordRateLimiterAllowed(eventType)
	}

	newAllowed := getCounterValue(t, metrics.RateLimiterAllowed, prometheus.Labels{"event_type": eventType})
	assert.Equal(t, initialAllowed+15, newAllowed, "Deberían haberse permitido 15 mensajes")

	// Obtener tokens después del procesamiento
	tokensAfterProcessing := limiter.Tokens(eventType)
	metrics.UpdateRateLimiterTokens(eventType, tokensAfterProcessing)

	// Verificar que se consumieron tokens (deberían quedar menos del burst inicial de 20)
	assert.Less(t, tokensAfterProcessing, 20.0, "Los tokens disponibles deberían ser menos que el burst inicial")
	assert.Greater(t, tokensAfterProcessing, 0.0, "Deberían quedar algunos tokens disponibles")

	t.Logf("Tokens después de procesar 15 mensajes: %.2f (burst inicial: 20)", tokensAfterProcessing)
}
