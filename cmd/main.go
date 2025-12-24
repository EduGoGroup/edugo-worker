package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/EduGoGroup/edugo-worker/internal/bootstrap"
	"github.com/EduGoGroup/edugo-worker/internal/config"
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
	defer func() {
		if err := cleanup(); err != nil {
			// Usar logger si está disponible, sino usar log estándar
			if resources != nil && resources.Logger != nil {
				resources.Logger.Error("Error en cleanup", "error", err.Error())
			} else {
				log.Printf("Error en cleanup: %v", err)
			}
		}
	}()

	resources.Logger.Info("✅ Worker iniciado correctamente")

	// 3. Configurar RabbitMQ queue y exchange
	if err := setupRabbitMQ(resources.RabbitMQChannel, cfg); err != nil {
		resources.Logger.Error("Error configurando RabbitMQ", "error", err.Error())
		log.Fatal(err)
	}

	// 4. Consumir mensajes
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

	// 5. Procesar mensajes
	go func() {
		for msg := range msgs {
			if err := processMessage(msg, resources, cfg); err != nil {
				resources.Logger.Error("Error procesando mensaje", "error", err.Error())
				if err := msg.Nack(false, true); err != nil {
					resources.Logger.Error("Error en Nack", "error", err.Error())
				}
			} else {
				if err := msg.Ack(false); err != nil {
					resources.Logger.Error("Error en Ack", "error", err.Error())
				}
			}
		}
	}()

	// 6. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	resources.Logger.Info("🛑 Señal de apagado recibida, cerrando worker...")
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
