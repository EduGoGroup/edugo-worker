package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/streadway/amqp"
)

// MaterialUploadedEvent representa el evento de material subido
type MaterialUploadedEvent struct {
	EventType         string    `json:"event_type"`
	MaterialID        string    `json:"material_id"`
	AuthorID          string    `json:"author_id"`
	S3Key             string    `json:"s3_key"`
	PreferredLanguage string    `json:"preferred_language"`
	Timestamp         time.Time `json:"timestamp"`
}

func main() {
	log.Println("🔄 EduGo Worker iniciando...")

	// Conectar a RabbitMQ
	conn, err := amqp.Dial("amqp://admin:fOrus.1305.@localhost:5672/")
	if err != nil {
		log.Fatal("Error conectando a RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Error abriendo canal:", err)
	}
	defer ch.Close()

	// Declarar exchange
	err = ch.ExchangeDeclare(
		"edugo_events", // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Fatal("Error declarando exchange:", err)
	}

	// Declarar cola de alta prioridad
	q, err := ch.QueueDeclare(
		"material_processing_high", // name
		true,                       // durable
		false,                      // delete when unused
		false,                      // exclusive
		false,                      // no-wait
		amqp.Table{
			"x-max-priority":         10,
			"x-dead-letter-exchange": "edugo_dlq",
		},
	)
	if err != nil {
		log.Fatal("Error declarando cola:", err)
	}

	// Bind cola al exchange
	err = ch.QueueBind(
		q.Name,              // queue name
		"material.uploaded", // routing key
		"edugo_events",      // exchange
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Error binding cola:", err)
	}

	// Consumir mensajes
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatal("Error registrando consumer:", err)
	}

	log.Println("✅ Worker escuchando cola 'material_processing_high'")
	log.Println("   Esperando eventos material.uploaded...")

	// Procesar mensajes
	forever := make(chan bool)

	go func() {
		for msg := range msgs {
			log.Printf("📥 Mensaje recibido: %s", msg.Body)

			var event MaterialUploadedEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				log.Printf("❌ Error parseando evento: %v", err)
				msg.Nack(false, false) // No requeue
				continue
			}

			// Procesar evento
			if err := ProcessMaterialUploaded(event); err != nil {
				log.Printf("❌ Error procesando material: %v", err)
				msg.Nack(false, true) // Requeue para reintento
				continue
			}

			log.Printf("✅ Material %s procesado exitosamente", event.MaterialID)
			msg.Ack(false)
		}
	}()

	log.Println("⏸️  Presiona CTRL+C para salir")
	<-forever
}

// ProcessMaterialUploaded procesa el evento material_uploaded
func ProcessMaterialUploaded(event MaterialUploadedEvent) error {
	log.Printf("🔧 Procesando material: %s", event.MaterialID)

	// TODO: 1. Descargar PDF desde S3
	log.Println("  1. Descargando PDF desde S3...")
	time.Sleep(2 * time.Second) // Mock

	// TODO: 2. Extraer texto del PDF
	log.Println("  2. Extrayendo texto del PDF...")
	time.Sleep(1 * time.Second) // Mock

	// TODO: 3. Llamar NLP API para generar resumen
	log.Println("  3. Generando resumen con IA (OpenAI GPT-4)...")
	time.Sleep(3 * time.Second) // Mock

	// TODO: 4. Guardar resumen en MongoDB
	log.Println("  4. Guardando resumen en MongoDB...")
	time.Sleep(1 * time.Second) // Mock

	// TODO: 5. Generar quiz con NLP
	log.Println("  5. Generando quiz con IA...")
	time.Sleep(2 * time.Second) // Mock

	// TODO: 6. Guardar quiz en MongoDB
	log.Println("  6. Guardando quiz en MongoDB...")
	time.Sleep(1 * time.Second) // Mock

	// TODO: 7. Actualizar PostgreSQL (material_summary_link, assessment)
	log.Println("  7. Actualizando PostgreSQL...")
	time.Sleep(1 * time.Second) // Mock

	// TODO: 8. Notificar docente
	log.Println("  8. Notificando docente...")

	log.Printf("✨ Material %s listo", event.MaterialID)
	return nil
}
