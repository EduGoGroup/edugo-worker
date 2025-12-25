package dto

import "time"

// MaterialUploadedEvent representa el evento recibido de API Mobile
// NOTA: Este es un mapeo TEMPORAL hasta implementar DTOs compartidos en edugo-shared
type MaterialUploadedEvent struct {
	// Campos del envelope (API Mobile usa Event wrapper)
	EventID      string    `json:"event_id"`
	EventType    string    `json:"event_type"`
	EventVersion string    `json:"event_version"`
	Timestamp    time.Time `json:"timestamp"`

	// Payload anidado
	Payload MaterialUploadedPayload `json:"payload"`
}

// MaterialUploadedPayload contiene los datos del material
type MaterialUploadedPayload struct {
	MaterialID    string                 `json:"material_id"`
	SchoolID      string                 `json:"school_id"`
	TeacherID     string                 `json:"teacher_id"`    // API usa teacher_id
	FileURL       string                 `json:"file_url"`       // API usa file_url (no s3_key)
	FileSizeBytes int64                  `json:"file_size_bytes"`
	FileType      string                 `json:"file_type"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// GetMaterialID helper para acceder al material ID desde el payload
func (e MaterialUploadedEvent) GetMaterialID() string {
	return e.Payload.MaterialID
}

// GetS3Key helper para compatibilidad (FileURL es la URL completa, extraer key si es necesario)
func (e MaterialUploadedEvent) GetS3Key() string {
	// Por ahora retornar FileURL directamente
	// TODO: Si es necesario extraer solo la key del path S3, implementar lógica aquí
	return e.Payload.FileURL
}

// GetAuthorID helper para mapear TeacherID → AuthorID
func (e MaterialUploadedEvent) GetAuthorID() string {
	return e.Payload.TeacherID
}

// AssessmentAttemptEvent evento cuando se intenta un quiz
type AssessmentAttemptEvent struct {
	EventType  string                 `json:"event_type"`
	MaterialID string                 `json:"material_id"`
	UserID     string                 `json:"user_id"`
	Answers    map[string]interface{} `json:"answers"`
	Score      float64                `json:"score"`
	Timestamp  time.Time              `json:"timestamp"`
}

// MaterialDeletedEvent evento cuando se elimina un material
type MaterialDeletedEvent struct {
	EventType  string    `json:"event_type"`
	MaterialID string    `json:"material_id"`
	Timestamp  time.Time `json:"timestamp"`
}

// StudentEnrolledEvent evento cuando un estudiante se inscribe
type StudentEnrolledEvent struct {
	EventType string    `json:"event_type"`
	StudentID string    `json:"student_id"`
	UnitID    string    `json:"unit_id"`
	Timestamp time.Time `json:"timestamp"`
}
