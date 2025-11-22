package service

import (
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// EventStateMachine maneja la máquina de estados y validaciones de MaterialEvent
// Extrae la lógica de negocio que antes estaba en la entity
type EventStateMachine struct {
	maxRetries int
}

// NewEventStateMachine crea una nueva instancia de EventStateMachine
func NewEventStateMachine(maxRetries int) *EventStateMachine {
	return &EventStateMachine{
		maxRetries: maxRetries,
	}
}

// IsValid valida que el event cumpla con las reglas de negocio
func (sm *EventStateMachine) IsValid(event *mongoentities.MaterialEvent) bool {
	if event.EventType == "" {
		return false
	}
	if !sm.isValidEventType(event.EventType) {
		return false
	}
	if event.Status == "" {
		return false
	}
	if !sm.isValidEventStatus(event.Status) {
		return false
	}
	if event.Payload == nil {
		return false
	}
	if event.RetryCount < 0 {
		return false
	}
	return true
}

// isValidEventType valida si el tipo de evento es válido
func (sm *EventStateMachine) isValidEventType(eventType string) bool {
	validTypes := []string{
		"material_uploaded",
		"material_reprocess",
		"material_deleted",
		"assessment_attempt",
		"student_enrolled",
		"student_unenrolled",
	}

	for _, t := range validTypes {
		if eventType == t {
			return true
		}
	}
	return false
}

// isValidEventStatus valida si el estado del evento es válido
func (sm *EventStateMachine) isValidEventStatus(status string) bool {
	validStatuses := []string{
		"pending",
		"processing",
		"completed",
		"failed",
	}

	for _, s := range validStatuses {
		if status == s {
			return true
		}
	}
	return false
}

// MarkAsProcessing marca el evento como en procesamiento
func (sm *EventStateMachine) MarkAsProcessing(event *mongoentities.MaterialEvent) {
	event.Status = "processing"
	event.UpdatedAt = time.Now()
}

// MarkAsCompleted marca el evento como completado
func (sm *EventStateMachine) MarkAsCompleted(event *mongoentities.MaterialEvent) {
	event.Status = "completed"
	now := time.Now()
	event.ProcessedAt = &now
	event.UpdatedAt = now
}

// MarkAsFailed marca el evento como fallido
func (sm *EventStateMachine) MarkAsFailed(event *mongoentities.MaterialEvent, errorMsg, stackTrace string) {
	event.Status = "failed"
	ptrErrorMsg := errorMsg
	ptrStackTrace := stackTrace
	event.ErrorMsg = &ptrErrorMsg
	event.StackTrace = &ptrStackTrace
	now := time.Now()
	event.ProcessedAt = &now
	event.UpdatedAt = now
}

// IncrementRetry incrementa el contador de reintentos
func (sm *EventStateMachine) IncrementRetry(event *mongoentities.MaterialEvent) {
	event.RetryCount++
	event.UpdatedAt = time.Now()
}

// CanRetry determina si el evento puede ser reintentado
func (sm *EventStateMachine) CanRetry(event *mongoentities.MaterialEvent) bool {
	return event.Status == "failed" && event.RetryCount < sm.maxRetries
}
