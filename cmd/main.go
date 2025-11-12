package main

import (
	"context"
	"fmt"
	"encoding/json"
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

	// 2. Inicializar infraestructura con shared/bootstrap
	resources, cleanup, err := bootstrap.Initialize(ctx, cfg)
	if err != nil {
		log.Fatal("❌ Error inicializando infraestructura:", err)
	}
	defer cleanup()

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
				msg.Nack(false, true) // requeue
			} else {
				msg.Ack(false)
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
	resources.Logger.Info("📥 Mensaje recibido", "size", len(msg.Body))

	var event map[string]interface{}
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		resources.Logger.Error("Error parseando evento", "error", err.Error())
		return err
	}

	resources.Logger.Info("✅ Evento procesado", "type", event["event_type"])
	
	// TODO: Implementar procesamiento real con processors
	// processor := container.GetProcessor(event["event_type"])
	// return processor.Process(ctx, event)
	
	return nil
}
