package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaterialEvent representa un evento de auditoría del worker
// Se almacena en MongoDB en la collection "material_event" con TTL de 90 días
type MaterialEvent struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	EventType   string             `bson:"event_type"`   // Tipo de evento (material_uploaded, assessment_attempt, etc.)
	MaterialID  string             `bson:"material_id,omitempty"` // UUID del material (opcional)
	UserID      string             `bson:"user_id,omitempty"`     // UUID del usuario (opcional)
	Payload     primitive.M        `bson:"payload"`       // Payload del evento (flexible)
	Status      string             `bson:"status"`        // "pending", "processing", "completed", "failed"
	ErrorMsg    string             `bson:"error_msg,omitempty"`    // Mensaje de error (solo si failed)
	StackTrace  string             `bson:"stack_trace,omitempty"`  // Stack trace (solo si failed)
	RetryCount  int                `bson:"retry_count"`   // Número de reintentos
	ProcessedAt *time.Time         `bson:"processed_at,omitempty"` // Fecha de procesamiento
	CreatedAt   time.Time          `bson:"created_at"`    // Fecha de creación (para TTL index)
	UpdatedAt   time.Time          `bson:"updated_at"`
}

// EventType constants
const (
	EventTypeMaterialUploaded     = "material_uploaded"
	EventTypeMaterialReprocess    = "material_reprocess"
	EventTypeMaterialDeleted      = "material_deleted"
	EventTypeAssessmentAttempt    = "assessment_attempt"
	EventTypeStudentEnrolled      = "student_enrolled"
	EventTypeStudentUnenrolled    = "student_unenrolled"
)

// EventStatus constants
const (
	EventStatusPending    = "pending"
	EventStatusProcessing = "processing"
	EventStatusCompleted  = "completed"
	EventStatusFailed     = "failed"
)

// NewMaterialEvent crea una nueva instancia de MaterialEvent
func NewMaterialEvent(eventType string, payload primitive.M) *MaterialEvent {
	now := time.Now()
	return &MaterialEvent{
		EventType:  eventType,
		Payload:    payload,
		Status:     EventStatusPending,
		RetryCount: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewMaterialEventWithMaterialID crea un evento con material_id
func NewMaterialEventWithMaterialID(eventType, materialID string, payload primitive.M) *MaterialEvent {
	event := NewMaterialEvent(eventType, payload)
	event.MaterialID = materialID
	return event
}

// IsValid valida que la entidad cumpla con las reglas de negocio
func (me *MaterialEvent) IsValid() bool {
	if me.EventType == "" {
		return false
	}
	if !isValidEventType(me.EventType) {
		return false
	}
	if me.Status == "" {
		return false
	}
	if !isValidEventStatus(me.Status) {
		return false
	}
	if me.Payload == nil {
		return false
	}
	if me.RetryCount < 0 {
		return false
	}
	return true
}

// isValidEventType valida si el tipo de evento es válido
func isValidEventType(eventType string) bool {
	validTypes := []string{
		EventTypeMaterialUploaded,
		EventTypeMaterialReprocess,
		EventTypeMaterialDeleted,
		EventTypeAssessmentAttempt,
		EventTypeStudentEnrolled,
		EventTypeStudentUnenrolled,
	}

	for _, t := range validTypes {
		if eventType == t {
			return true
		}
	}
	return false
}

// isValidEventStatus valida si el estado del evento es válido
func isValidEventStatus(status string) bool {
	validStatuses := []string{
		EventStatusPending,
		EventStatusProcessing,
		EventStatusCompleted,
		EventStatusFailed,
	}

	for _, s := range validStatuses {
		if status == s {
			return true
		}
	}
	return false
}

// MarkAsProcessing marca el evento como en procesamiento
func (me *MaterialEvent) MarkAsProcessing() {
	me.Status = EventStatusProcessing
	me.UpdatedAt = time.Now()
}

// MarkAsCompleted marca el evento como completado
func (me *MaterialEvent) MarkAsCompleted() {
	me.Status = EventStatusCompleted
	now := time.Now()
	me.ProcessedAt = &now
	me.UpdatedAt = now
}

// MarkAsFailed marca el evento como fallido
func (me *MaterialEvent) MarkAsFailed(errorMsg, stackTrace string) {
	me.Status = EventStatusFailed
	me.ErrorMsg = errorMsg
	me.StackTrace = stackTrace
	now := time.Now()
	me.ProcessedAt = &now
	me.UpdatedAt = now
}

// IncrementRetry incrementa el contador de reintentos
func (me *MaterialEvent) IncrementRetry() {
	me.RetryCount++
	me.UpdatedAt = time.Now()
}

// CanRetry determina si el evento puede ser reintentado
func (me *MaterialEvent) CanRetry(maxRetries int) bool {
	return me.Status == EventStatusFailed && me.RetryCount < maxRetries
}
