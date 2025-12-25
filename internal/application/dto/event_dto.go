package dto

import (
	"strings"
	"time"
)

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
	TeacherID     string                 `json:"teacher_id"` // API usa teacher_id
	FileURL       string                 `json:"file_url"`   // API usa file_url (no s3_key)
	FileSizeBytes int64                  `json:"file_size_bytes"`
	FileType      string                 `json:"file_type"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// GetMaterialID helper para acceder al material ID desde el payload
func (e MaterialUploadedEvent) GetMaterialID() string {
	return e.Payload.MaterialID
}

// GetS3Key helper para compatibilidad (extrae la key S3 desde FileURL)
func (e MaterialUploadedEvent) GetS3Key() string {
	// 1. Si hay metadata con s3_key, usar ese valor (preferido)
	if e.Payload.Metadata != nil {
		if key, ok := e.Payload.Metadata["s3_key"].(string); ok && key != "" {
			return key
		}
	}

	// 2. Intentar extraer key desde FileURL
	fileURL := e.Payload.FileURL

	// Formato s3://bucket/path/to/file.pdf
	if strings.HasPrefix(fileURL, "s3://") {
		// Remover prefijo s3://
		rest := strings.TrimPrefix(fileURL, "s3://")
		// Buscar el primer / para saltar el bucket
		if idx := strings.Index(rest, "/"); idx != -1 {
			return rest[idx+1:]
		}
	}

	// Formato https://s3.amazonaws.com/bucket/path/to/file.pdf o
	// https://bucket.s3.amazonaws.com/path/to/file.pdf
	if strings.Contains(fileURL, "s3.amazonaws.com") || strings.Contains(fileURL, "s3.") {
		// Buscar la posición después del dominio
		if idx := strings.Index(fileURL, "//"); idx != -1 {
			// Saltar protocolo
			rest := fileURL[idx+2:]
			// Buscar el primer /
			if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
				pathPart := rest[slashIdx+1:]
				// Si contiene otro /, probablemente es bucket/key
				if strings.Contains(pathPart, "/") {
					// Formato: .../bucket/key - saltar el bucket
					parts := strings.SplitN(pathPart, "/", 2)
					if len(parts) >= 2 {
						return parts[1]
					}
				}
				return pathPart
			}
		}
	}

	// 3. Fallback: retornar FileURL completo
	return fileURL
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
