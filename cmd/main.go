package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	rabbit "github.com/EduGoGroup/edugo-shared/messaging/rabbit"
	"github.com/EduGoGroup/edugo-worker/internal/bootstrap"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/metrics"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/ratelimiter"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/shutdown"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	log.Println("🔄 EduGo Worker iniciando...")

	ctx := context.Background()

	// 1. Cargar configuración
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("❌ Error cargando configuración:", err)
	}

	// 2. Inicializar infraestructura usando ResourceBuilder
	resources, cleanup, err := bootstrap.NewResourceBuilder(ctx, cfg).
		WithLogger().
		WithSharedMetrics().
		WithRabbitMQ().
		WithAuthClient().
		WithM2MClients().
		WithLLMProvider().
		WithInfrastructure().
		WithProcessors().
		WithHealthChecks().
		WithMetricsServer().
		Build()

	if err != nil {
		log.Fatal("❌ Error inicializando infraestructura:", err)
	}
	// Nota: No usamos defer cleanup() aquí porque lo gestionamos
	// a través del graceful shutdown usando patrón LIFO (Last In, First Out)
	// para cerrar recursos en orden inverso a su inicialización

	resources.Logger.Info("✅ Worker iniciado correctamente")

	// 3. Configurar RabbitMQ queue y exchange
	if err := setupRabbitMQ(resources.RabbitMQChannel, cfg); err != nil {
		resources.Logger.Error("Error configurando RabbitMQ", "error", err.Error())
		log.Fatal(err)
	}

	// 4. Configurar rate limiter
	var rateLimiter *ratelimiter.MultiRateLimiter
	rateLimiterCfg := cfg.GetRateLimiterConfigWithDefaults()

	if rateLimiterCfg.Enabled {
		// Convertir configuración a formato esperado por MultiRateLimiter
		configs := make(map[string]ratelimiter.Config)
		for eventType, eventCfg := range rateLimiterCfg.ByEventType {
			configs[eventType] = ratelimiter.Config{
				RequestsPerSecond: eventCfg.RequestsPerSecond,
				BurstSize:         eventCfg.BurstSize,
			}
		}

		// Configuración por defecto
		defaultCfg := &ratelimiter.Config{
			RequestsPerSecond: rateLimiterCfg.Default.RequestsPerSecond,
			BurstSize:         rateLimiterCfg.Default.BurstSize,
		}

		rateLimiter = ratelimiter.NewMulti(configs, defaultCfg)
		resources.Logger.Info("✅ Rate limiter habilitado",
			"configured_events", len(configs),
			"default_rps", defaultCfg.RequestsPerSecond,
			"default_burst", defaultCfg.BurstSize)
	} else {
		resources.Logger.Info("⚠️  Rate limiter deshabilitado")
	}

	// 5. Crear consumer compartido con soporte DLQ
	prefetchCount := cfg.Messaging.RabbitMQ.PrefetchCount
	if prefetchCount == 0 {
		prefetchCount = 5
	}

	dlqCfg := cfg.GetDLQConfigWithDefaults().ToShared()

	consumerCfg := rabbit.ConsumerConfig{
		Name:          "edugo-worker",
		AutoAck:       false,
		PrefetchCount: prefetchCount,
		DLQ:           dlqCfg,
	}

	consumer, ok := rabbit.NewConsumer(resources.RabbitMQConn, consumerCfg).(*rabbit.RabbitMQConsumer)
	if !ok {
		log.Fatal("failed to create RabbitMQ consumer: unexpected type")
	}

	// Consumer del carril de PREPARACIÓN (plan 042 F2a): instancia PROPIA porque el
	// consumer compartido tiene guard `running` (una sola ConsumeWithDLQ por
	// instancia) y porque su DLQ es propia (DLXRoutingKey = cola de prep + ".dlq").
	// Comparte el mismo ProcessorRegistry (enruta por event_type).
	prepDLQCfg := dlqCfg
	prepDLQCfg.DLXRoutingKey = cfg.GetQueuesConfigWithDefaults().PrepDLQName()
	prepConsumerCfg := rabbit.ConsumerConfig{
		Name:          "edugo-worker-prep",
		AutoAck:       false,
		PrefetchCount: prefetchCount,
		DLQ:           prepDLQCfg,
	}
	prepConsumer, ok := rabbit.NewConsumer(resources.RabbitMQConn, prepConsumerCfg).(*rabbit.RabbitMQConsumer)
	if !ok {
		log.Fatal("failed to create RabbitMQ prep consumer: unexpected type")
	}

	// Consumer del carril MATERIAL→EVALUACIÓN (plan 043 F3c): instancia PROPIA por la
	// misma razón que el de prep (guard `running` + DLQ propia). Comparte el mismo
	// ProcessorRegistry (enruta por event_type).
	materialDLQCfg := dlqCfg
	materialDLQCfg.DLXRoutingKey = cfg.GetQueuesConfigWithDefaults().MaterialAssessmentDLQName()
	materialConsumerCfg := rabbit.ConsumerConfig{
		Name:          "edugo-worker-material",
		AutoAck:       false,
		PrefetchCount: prefetchCount,
		DLQ:           materialDLQCfg,
	}
	materialConsumer, ok := rabbit.NewConsumer(resources.RabbitMQConn, materialConsumerCfg).(*rabbit.RabbitMQConsumer)
	if !ok {
		log.Fatal("failed to create RabbitMQ material consumer: unexpected type")
	}

	// 6. Crear handler con rate limiting integrado
	handler := func(ctx context.Context, body []byte) error {
		if rateLimiter != nil {
			eventType := "unknown"
			var base struct {
				EventType string `json:"event_type"`
			}
			if err := json.Unmarshal(body, &base); err == nil && base.EventType != "" {
				eventType = base.EventType
			}

			start := time.Now()
			if err := rateLimiter.Wait(ctx, eventType); err != nil {
				return fmt.Errorf("rate limiter interrupted: %w", err)
			}
			waitDuration := time.Since(start).Seconds()
			metrics.RecordRateLimiterWait(eventType, waitDuration)
			metrics.RecordRateLimiterAllowed(eventType)
			if tokens := rateLimiter.Tokens(eventType); tokens >= 0 {
				metrics.UpdateRateLimiterTokens(eventType, tokens)
			}
		}

		resources.Logger.Info("Mensaje recibido", "size", len(body))
		if err := resources.ProcessorRegistry.Process(ctx, body); err != nil {
			resources.Logger.Error("Error procesando evento", "error", err.Error())
			return err
		}
		resources.Logger.Info("Evento procesado exitosamente")
		return nil
	}

	// 7. Iniciar consumo con soporte DLQ (un consumer por riel, mismo handler/registry)
	queuesCfg := cfg.GetQueuesConfigWithDefaults()
	reviewQueue := queuesCfg.AttemptReviewRequested
	prepQueue := queuesCfg.QuestionPrepRequested
	materialQueue := queuesCfg.MaterialAssessmentRequested
	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	if err := consumer.ConsumeWithDLQ(consumerCtx, reviewQueue, handler); err != nil {
		resources.Logger.Error("Error iniciando consumer de revisión", "error", err.Error())
		log.Fatal(err)
	}
	if err := prepConsumer.ConsumeWithDLQ(consumerCtx, prepQueue, handler); err != nil {
		resources.Logger.Error("Error iniciando consumer de preparación", "error", err.Error())
		log.Fatal(err)
	}
	if err := materialConsumer.ConsumeWithDLQ(consumerCtx, materialQueue, handler); err != nil {
		resources.Logger.Error("Error iniciando consumer de materiales", "error", err.Error())
		log.Fatal(err)
	}

	resources.Logger.Info("Worker escuchando eventos",
		"review_queue", reviewQueue,
		"prep_queue", prepQueue,
		"material_queue", materialQueue,
		"prefetch_count", prefetchCount,
		"dlq_enabled", dlqCfg.Enabled,
		"max_retries", dlqCfg.MaxRetries)

	// 8. Configurar graceful shutdown
	shutdownCfg := cfg.GetShutdownConfigWithDefaults()
	gracefulShutdown := shutdown.NewGracefulShutdown(shutdownCfg.Timeout, resources.Logger)

	// 9. Registrar tareas de shutdown en orden inverso de inicialización
	// Ultimo en inicializarse, primero en cerrarse (LIFO)

	// 9.1 Detener consumers (dejar de aceptar nuevos mensajes en ambos rieles)
	gracefulShutdown.Register("consumer", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Deteniendo consumers de mensajes...")
		cancelConsumer()
		consumer.Stop()
		prepConsumer.Stop()
		materialConsumer.Stop()

		if shutdownCfg.WaitForMessages {
			resources.Logger.Info("Esperando que terminen los mensajes en proceso...")
			if err := consumer.Wait(); err != nil {
				resources.Logger.Warn("Consumer de revisión detenido con error", "error", err.Error())
			}
			if err := prepConsumer.Wait(); err != nil {
				resources.Logger.Warn("Consumer de preparación detenido con error", "error", err.Error())
			}
			if err := materialConsumer.Wait(); err != nil {
				resources.Logger.Warn("Consumer de materiales detenido con error", "error", err.Error())
			}
			resources.Logger.Info("Todos los mensajes fueron procesados")
		}

		return nil
	})

	// 9.2 Cerrar servidor de metricas
	gracefulShutdown.Register("metrics_server", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Cerrando servidor de métricas...")
		if resources.MetricsServer != nil {
			return resources.MetricsServer.Shutdown(shutdownCtx)
		}
		return nil
	})

	// 9.3 Ejecutar cleanup de recursos (RabbitMQ, logger, etc.)
	gracefulShutdown.Register("infrastructure_cleanup", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Ejecutando cleanup de infraestructura...")
		return cleanup()
	})

	// 10. Esperar senal de shutdown y ejecutar
	resources.Logger.Info("Worker listo - esperando mensajes...")

	if err := gracefulShutdown.WaitForSignal(); err != nil {
		resources.Logger.Error("❌ Errores durante shutdown", "error", err.Error())
		log.Fatal(err)
	}

	resources.Logger.Info("✅ Worker cerrado correctamente")
}

// setupRabbitMQ configura exchanges, queue y bindings.
//
// Topología post-dieta (plan 040 §6.2):
//   - Declara el exchange `edugo.assessments` (topic, durable) — carril de revisión.
//   - Declara la cola `edugo.attempt.review_requested` (durable, prioridad + DLX) y la
//     bindea a `edugo.assessments` con routing key `attempt.review_requested`. Es la cola
//     que consume el worker hoy (processor AttemptReviewProcessor, registry ya no vacío).
//   - Sigue declarando el exchange `edugo.materials` aunque NO consuma su cola: learning
//     publica ahí y el publisher no declara exchanges; un Rabbit fresco rompería el publish
//     de material si el worker dejara de declararlo. El plan 041 revive la cola/consumer.
//   - DLX coherente: el arg `x-dead-letter-exchange` de la cola toma el nombre desde config
//     (antes hardcodeaba `edugo_dlq`, que nadie declaraba, mientras el consumer declara
//     `edugo_dlx`). Se añade `x-dead-letter-routing-key` para que el dead-letter nativo caiga
//     en la misma cola DLQ que declara el consumer compartido.
func setupRabbitMQ(ch *amqp.Channel, cfg *config.Config) error {
	exchanges := cfg.GetExchangesConfigWithDefaults()
	queues := cfg.GetQueuesConfigWithDefaults()
	dlq := cfg.GetDLQConfigWithDefaults()

	// Declarar exchange de materiales (solo declaración; sin cola/consumer — plan 041).
	if err := ch.ExchangeDeclare(
		exchanges.Materials,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("error declarando materials exchange: %w", err)
	}

	// Declarar exchange de evaluaciones (carril de revisión).
	if err := ch.ExchangeDeclare(
		exchanges.Assessments,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("error declarando assessments exchange: %w", err)
	}

	// Declarar cola de revisión con prioridad + DLX coherente con config.
	_, err := ch.QueueDeclare(
		queues.AttemptReviewRequested,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-max-priority":            10,
			"x-dead-letter-exchange":    dlq.DLXExchange,
			"x-dead-letter-routing-key": dlq.DLXRoutingKey,
		},
	)
	if err != nil {
		return fmt.Errorf("error declarando cola de revisión: %w", err)
	}

	// Bind cola de revisión al routing key attempt.review_requested.
	if err := ch.QueueBind(
		queues.AttemptReviewRequested,
		"attempt.review_requested",
		exchanges.Assessments,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("error binding cola de revisión: %w", err)
	}

	// Cola del carril de PREPARACIÓN (plan 042 F2a): canal propio por riel (D-042.3),
	// sobre el mismo exchange edugo.assessments pero con routing key y DLQ propias
	// (no comparte cola con revisión). Su dead-letter cae en la DLQ del riel de prep.
	if _, err := ch.QueueDeclare(
		queues.QuestionPrepRequested,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-max-priority":            10,
			"x-dead-letter-exchange":    dlq.DLXExchange,
			"x-dead-letter-routing-key": queues.PrepDLQName(),
		},
	); err != nil {
		return fmt.Errorf("error declarando cola de preparación: %w", err)
	}

	// Bind cola de preparación al routing key question.prep_requested.
	if err := ch.QueueBind(
		queues.QuestionPrepRequested,
		events.EventTypeQuestionPrepRequested,
		exchanges.Assessments,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("error binding cola de preparación: %w", err)
	}

	// Cola del carril MATERIAL→EVALUACIÓN (plan 043 F3c): canal propio por riel. A
	// diferencia de revisión/preparación, se bindea al exchange edugo.materials (donde
	// learning publica material.assessment_requested), con routing key y DLQ propias.
	if _, err := ch.QueueDeclare(
		queues.MaterialAssessmentRequested,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-max-priority":            10,
			"x-dead-letter-exchange":    dlq.DLXExchange,
			"x-dead-letter-routing-key": queues.MaterialAssessmentDLQName(),
		},
	); err != nil {
		return fmt.Errorf("error declarando cola de materiales: %w", err)
	}

	// Bind cola de materiales al routing key material.assessment_requested sobre el
	// exchange edugo.materials.
	if err := ch.QueueBind(
		queues.MaterialAssessmentRequested,
		events.EventTypeMaterialAssessmentRequested,
		exchanges.Materials,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("error binding cola de materiales: %w", err)
	}

	return nil
}
