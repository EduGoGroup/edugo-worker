package constants

// EventType constants - Tipos de eventos de auditoría
const (
	EventTypeMaterialUploaded  = "material.uploaded"
	EventTypeMaterialReprocess = "material.reprocess"
	EventTypeMaterialDeleted   = "material.deleted"
	EventTypeAssessmentAttempt = "assessment.attempt"
	EventTypeStudentEnrolled   = "student.enrolled"
	EventTypeStudentUnenrolled = "student.unenrolled"
)

// EventStatus constants - Estados de procesamiento de eventos
const (
	EventStatusPending    = "pending"
	EventStatusProcessing = "processing"
	EventStatusCompleted  = "completed"
	EventStatusFailed     = "failed"
)
