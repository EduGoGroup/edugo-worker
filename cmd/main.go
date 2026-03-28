package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
		WithPostgreSQL().
		WithMongoDB().
		WithRabbitMQ().
		WithAuthClient().
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

	// 7. Iniciar consumo con soporte DLQ
	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	if err := consumer.ConsumeWithDLQ(consumerCtx, cfg.Messaging.RabbitMQ.Queues.MaterialUploaded, handler); err != nil {
		resources.Logger.Error("Error iniciando consumer", "error", err.Error())
		log.Fatal(err)
	}

	resources.Logger.Info("Worker escuchando eventos",
		"queue", cfg.Messaging.RabbitMQ.Queues.MaterialUploaded,
		"prefetch_count", prefetchCount,
		"dlq_enabled", dlqCfg.Enabled,
		"max_retries", dlqCfg.MaxRetries)

	// 7b. Iniciar consumer de notificaciones de evaluaciones
	assessmentConsumerCfg := rabbit.ConsumerConfig{
		Name:          "edugo-worker-assessments",
		AutoAck:       false,
		PrefetchCount: prefetchCount,
		DLQ:           dlqCfg,
	}

	assessmentConsumer, ok := rabbit.NewConsumer(resources.RabbitMQConn, assessmentConsumerCfg).(*rabbit.RabbitMQConsumer)
	if !ok {
		log.Fatal("failed to create assessment consumer: unexpected type")
	}

	assessmentConsumerCtx, cancelAssessmentConsumer := context.WithCancel(ctx)
	if err := assessmentConsumer.ConsumeWithDLQ(assessmentConsumerCtx, cfg.Messaging.RabbitMQ.Queues.AssessmentNotifications, handler); err != nil {
		resources.Logger.Error("Error iniciando assessment consumer", "error", err.Error())
		log.Fatal(err)
	}

	resources.Logger.Info("Assessment notifications consumer started",
		"queue", cfg.Messaging.RabbitMQ.Queues.AssessmentNotifications)

	// 8. Configurar graceful shutdown
	shutdownCfg := cfg.GetShutdownConfigWithDefaults()
	gracefulShutdown := shutdown.NewGracefulShutdown(shutdownCfg.Timeout, resources.Logger)

	// 9. Registrar tareas de shutdown en orden inverso de inicialización
	// Ultimo en inicializarse, primero en cerrarse (LIFO)

	// 9.1 Detener consumers (dejar de aceptar nuevos mensajes)
	gracefulShutdown.Register("consumer", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Deteniendo consumers de mensajes...")
		cancelConsumer()
		cancelAssessmentConsumer()
		consumer.Stop()
		assessmentConsumer.Stop()

		if shutdownCfg.WaitForMessages {
			resources.Logger.Info("Esperando que terminen los mensajes en proceso...")
			if err := consumer.Wait(); err != nil {
				resources.Logger.Warn("Material consumer detenido con error", "error", err.Error())
			}
			if err := assessmentConsumer.Wait(); err != nil {
				resources.Logger.Warn("Assessment consumer detenido con error", "error", err.Error())
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

	// 9.3 Ejecutar cleanup de recursos (RabbitMQ, MongoDB, PostgreSQL, etc.)
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

// setupRabbitMQ configura exchange, queue y bindings
func setupRabbitMQ(ch *amqp.Channel, cfg *config.Config) error {
	// Declarar exchange
	if err := ch.ExchangeDeclare(
		cfg.Messaging.RabbitMQ.Exchanges.Materials,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("error declarando exchange: %w", err)
	}

	// Declarar cola
	_, err := ch.QueueDeclare(
		cfg.Messaging.RabbitMQ.Queues.MaterialUploaded,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-max-priority":         10,
			"x-dead-letter-exchange": "edugo_dlq",
		},
	)
	if err != nil {
		return fmt.Errorf("error declarando cola: %w", err)
	}

	// Bind cola
	if err := ch.QueueBind(
		cfg.Messaging.RabbitMQ.Queues.MaterialUploaded,
		"material.uploaded",
		cfg.Messaging.RabbitMQ.Exchanges.Materials,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("error binding cola: %w", err)
	}

	// Declarar cola de notificaciones de evaluaciones
	_, err = ch.QueueDeclare(
		cfg.Messaging.RabbitMQ.Queues.AssessmentNotifications,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange": "edugo_dlq",
		},
	)
	if err != nil {
		return fmt.Errorf("error declaring assessment notifications queue: %w", err)
	}

	// Bind routing keys de eventos de evaluaciones
	for _, routingKey := range []string{"assessment.assigned", "assessment.attempt_recorded", "assessment.reviewed"} {
		if err := ch.QueueBind(
			cfg.Messaging.RabbitMQ.Queues.AssessmentNotifications,
			routingKey,
			cfg.Messaging.RabbitMQ.Exchanges.Materials,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("error binding routing key %s: %w", routingKey, err)
		}
	}

	return nil
}
