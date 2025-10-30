package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
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
	log.Println("📨 Script para enviar mensaje de prueba a RabbitMQ")

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

	// Crear evento de prueba
	event := MaterialUploadedEvent{
		EventType:         "material.uploaded",
		MaterialID:        uuid.New().String(),
		AuthorID:          "teacher-uuid-" + uuid.New().String()[:8],
		S3Key:             "materials/" + uuid.New().String() + "/document.pdf",
		PreferredLanguage: "es",
		Timestamp:         time.Now(),
	}

	// Serializar evento
	body, err := json.Marshal(event)
	if err != nil {
		log.Fatal("Error serializando evento:", err)
	}

	// Publicar mensaje
	err = ch.Publish(
		"edugo_events",      // exchange
		"material.uploaded", // routing key
		false,               // mandatory
		false,               // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Mensaje persistente
			Priority:     10,              // Prioridad alta
		})
	if err != nil {
		log.Fatal("Error publicando mensaje:", err)
	}

	log.Printf("✅ Mensaje enviado exitosamente:")
	log.Printf("   Material ID: %s", event.MaterialID)
	log.Printf("   Author ID: %s", event.AuthorID)
	log.Printf("   S3 Key: %s", event.S3Key)
	log.Println("   El worker debería procesar este mensaje si está corriendo")
}
