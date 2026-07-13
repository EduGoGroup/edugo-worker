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

	// Formato https con dominio conocido: S3, R2, o cualquier storage con bucket en el path
	// Ejemplos:
	//   https://s3.amazonaws.com/bucket/key
	//   https://bucket.s3.amazonaws.com/key
	//   https://xxx.r2.cloudflarestorage.com/bucket/key
	if strings.HasPrefix(fileURL, "https://") || strings.HasPrefix(fileURL, "http://") {
		if idx := strings.Index(fileURL, "//"); idx != -1 {
			rest := fileURL[idx+2:]
			if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
				pathPart := rest[slashIdx+1:]
				// pathPart = "bucket/path/to/file" — saltar el bucket
				if strings.Contains(pathPart, "/") {
					parts := strings.SplitN(pathPart, "/", 2)
					if len(parts) >= 2 {
						return parts[1]
					}
				}
				return pathPart
			}
		}
	}

	// 3. Fallback: retornar FileURL completo (ya es un path relativo)
	return fileURL
}

// GetAuthorID helper para mapear TeacherID → AuthorID
func (e MaterialUploadedEvent) GetAuthorID() string {
	return e.Payload.TeacherID
}
