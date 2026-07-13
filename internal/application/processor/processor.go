package processor

import "context"

// Processor interfaz común para todos los procesadores de eventos
//
// Cada processor debe implementar esta interfaz para ser registrado
// en el ProcessorRegistry y procesar eventos de RabbitMQ.
type Processor interface {
	// EventType retorna el tipo de evento que este processor maneja
	// Ejemplos: "material.uploaded", "material.reprocess"
	EventType() string

	// Process procesa el payload del evento
	// El payload es el mensaje raw en JSON que viene de RabbitMQ
	Process(ctx context.Context, payload []byte) error
}
