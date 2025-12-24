package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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
	// a través del graceful shutdown para tener mejor control del orden

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

	// 5. Consumir mensajes
	msgs, err := resources.RabbitMQChannel.Consume(
		cfg.Messaging.RabbitMQ.Queues.MaterialUploaded, // queue
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		resources.Logger.Error("Error registrando consumer", "error", err.Error())
		log.Fatal(err)
	}

	resources.Logger.Info("✅ Worker escuchando eventos",
		"queue", cfg.Messaging.RabbitMQ.Queues.MaterialUploaded)

	// 6. Configurar graceful shutdown
	shutdownCfg := cfg.GetShutdownConfigWithDefaults()
	gracefulShutdown := shutdown.NewGracefulShutdown(shutdownCfg.Timeout, resources.Logger)

	// 7. Procesar mensajes con rate limiting
	var processingWG sync.WaitGroup
	consumerCtx, cancelConsumer := context.WithCancel(ctx)

	go func() {
		for msg := range msgs {
			// Si el contexto está cancelado, no procesar más mensajes
			select {
			case <-consumerCtx.Done():
				// Rechazar mensaje para que se reintente después del shutdown
				if err := msg.Nack(false, true); err != nil {
					resources.Logger.Error("Error en Nack durante shutdown", "error", err.Error())
				}
				return
			default:
			}

			processingWG.Add(1)

			go func(m amqp.Delivery) {
				defer processingWG.Done()

				// Extraer tipo de evento del routing key
				eventType := m.RoutingKey
				if eventType == "" {
					eventType = "unknown"
				}

				// Aplicar rate limiting si está habilitado
				if rateLimiter != nil {
					start := time.Now()

					if err := rateLimiter.Wait(consumerCtx, eventType); err != nil {
						resources.Logger.Warn("Rate limiter interrumpido",
							"event_type", eventType,
							"error", err.Error())

						// Rechazar mensaje para que se reintente después
						if err := m.Nack(false, true); err != nil {
							resources.Logger.Error("Error en Nack después de rate limit",
								"error", err.Error())
						}
						return
					}

					// Registrar métricas de rate limiting
					waitDuration := time.Since(start).Seconds()
					if waitDuration > 0.001 { // Solo si esperó más de 1ms
						metrics.RecordRateLimiterWait(eventType, waitDuration)
					}
					metrics.RecordRateLimiterAllowed(eventType)

					// Actualizar tokens disponibles
					tokens := rateLimiter.Tokens(eventType)
					if tokens >= 0 {
						metrics.UpdateRateLimiterTokens(eventType, tokens)
					}
				}

				// Procesar mensaje
				if err := processMessage(m, resources, cfg); err != nil {
					resources.Logger.Error("Error procesando mensaje",
						"event_type", eventType,
						"error", err.Error())

					if err := m.Nack(false, true); err != nil {
						resources.Logger.Error("Error en Nack", "error", err.Error())
					}
				} else {
					if err := m.Ack(false); err != nil {
						resources.Logger.Error("Error en Ack", "error", err.Error())
					}
				}
			}(msg)
		}
	}()

	// 8. Registrar tareas de shutdown en orden inverso de inicialización
	// Último en inicializarse, primero en cerrarse (LIFO)

	// 8.1 Detener consumer (dejar de aceptar nuevos mensajes)
	gracefulShutdown.Register("consumer", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Deteniendo consumer de mensajes...")
		cancelConsumer()

		if shutdownCfg.WaitForMessages {
			resources.Logger.Info("Esperando que terminen los mensajes en proceso...")

			// Esperar con timeout
			done := make(chan struct{})
			go func() {
				processingWG.Wait()
				close(done)
			}()

			select {
			case <-done:
				resources.Logger.Info("✅ Todos los mensajes fueron procesados")
			case <-shutdownCtx.Done():
				resources.Logger.Warn("⚠️  Timeout esperando mensajes en proceso")
			}
		}

		return nil
	})

	// 8.2 Cerrar servidor de métricas
	gracefulShutdown.Register("metrics_server", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Cerrando servidor de métricas...")
		if resources.MetricsServer != nil {
			return resources.MetricsServer.Shutdown(shutdownCtx)
		}
		return nil
	})

	// 8.3 Ejecutar cleanup de recursos (RabbitMQ, MongoDB, PostgreSQL, etc.)
	gracefulShutdown.Register("infrastructure_cleanup", func(shutdownCtx context.Context) error {
		resources.Logger.Info("Ejecutando cleanup de infraestructura...")
		return cleanup()
	})

	// 9. Esperar señal de shutdown y ejecutar
	resources.Logger.Info("✅ Worker listo - esperando mensajes...")

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

	return nil
}

// processMessage procesa un mensaje de RabbitMQ
func processMessage(msg amqp.Delivery, resources *bootstrap.Resources, cfg *config.Config) error {
	ctx := context.Background()

	resources.Logger.Info("📥 Mensaje recibido", "size", len(msg.Body))

	// Usar el ProcessorRegistry para procesar el evento
	if err := resources.ProcessorRegistry.Process(ctx, msg.Body); err != nil {
		resources.Logger.Error("Error procesando evento", "error", err.Error())
		return err
	}

	resources.Logger.Info("✅ Evento procesado exitosamente")
	return nil
}
