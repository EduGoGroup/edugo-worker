package constants

// EventType constants - Tipos de eventos de auditoría
const (
	EventTypeMaterialUploaded  = "material_uploaded"
	EventTypeMaterialReprocess = "material_reprocess"
	EventTypeMaterialDeleted   = "material_deleted"
	EventTypeAssessmentAttempt = "assessment_attempt"
	EventTypeStudentEnrolled   = "student_enrolled"
	EventTypeStudentUnenrolled = "student_unenrolled"
)

// EventStatus constants - Estados de procesamiento de eventos
const (
	EventStatusPending    = "pending"
	EventStatusProcessing = "processing"
	EventStatusCompleted  = "completed"
	EventStatusFailed     = "failed"
)
