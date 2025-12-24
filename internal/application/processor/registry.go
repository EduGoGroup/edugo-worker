package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
)

// Registry mantiene un registro de processors por event type
//
// Permite registrar processors y rutear mensajes al processor correcto
// basado en el campo event_type del mensaje JSON.
type Registry struct {
	processors map[string]Processor
	logger     logger.Logger
}

// NewRegistry crea un nuevo registry vacío
func NewRegistry(logger logger.Logger) *Registry {
	return &Registry{
		processors: make(map[string]Processor),
		logger:     logger,
	}
}

// Register registra un processor para su event type
//
// Si ya existe un processor para el mismo event type, se sobrescribe
// y se loguea una advertencia.
func (r *Registry) Register(p Processor) {
	eventType := p.EventType()

	if _, exists := r.processors[eventType]; exists {
		r.logger.Warn("overwriting existing processor", "event_type", eventType)
	}

	r.processors[eventType] = p
	r.logger.Info("processor registered", "event_type", eventType)
}

// Process procesa un mensaje usando el processor correcto
//
// Extrae el event_type del mensaje JSON, busca el processor registrado
// y delega el procesamiento. Si no hay processor para el event_type,
// loguea una advertencia pero NO retorna error (para no hacer nack del mensaje).
func (r *Registry) Process(ctx context.Context, payload []byte) error {
	// Extraer event_type del mensaje
	var base struct {
		EventType string `json:"event_type"`
	}

	if err := json.Unmarshal(payload, &base); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	if base.EventType == "" {
		return fmt.Errorf("missing event_type field in message")
	}

	// Buscar processor
	processor, ok := r.processors[base.EventType]
	if !ok {
		// No es error: simplemente no tenemos processor para este tipo
		r.logger.Warn("no processor registered for event type",
			"event_type", base.EventType,
			"available_processors", r.RegisteredTypes(),
		)
		return nil
	}

	// Procesar con el processor correcto
	r.logger.Debug("routing to processor", "event_type", base.EventType)
	return processor.Process(ctx, payload)
}

// RegisteredTypes retorna la lista de event types registrados
func (r *Registry) RegisteredTypes() []string {
	types := make([]string, 0, len(r.processors))
	for eventType := range r.processors {
		types = append(types, eventType)
	}
	return types
}

// Count retorna el número de processors registrados
func (r *Registry) Count() int {
	return len(r.processors)
}
