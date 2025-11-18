package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaterialSummary representa un resumen de material educativo generado por IA
// Se almacena en MongoDB en la collection "material_summary"
type MaterialSummary struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	MaterialID       string             `bson:"material_id"`       // UUID del material en PostgreSQL
	Summary          string             `bson:"summary"`           // Resumen completo generado por OpenAI
	KeyPoints        []string           `bson:"key_points"`        // Puntos clave extraídos (1-10)
	Language         string             `bson:"language"`          // "es", "en", "pt"
	WordCount        int                `bson:"word_count"`        // Número de palabras del resumen
	Version          int                `bson:"version"`           // Versión del resumen (>= 1)
	AIModel          string             `bson:"ai_model"`          // "gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"
	ProcessingTimeMs int                `bson:"processing_time_ms"` // Tiempo de procesamiento en ms
	TokenUsage       *TokenUsage        `bson:"token_usage,omitempty"` // Metadata de tokens consumidos (opcional)
	Metadata         *SummaryMetadata   `bson:"metadata,omitempty"`    // Metadata adicional (opcional)
	CreatedAt        time.Time          `bson:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at"`
}

// TokenUsage representa el uso de tokens de OpenAI
type TokenUsage struct {
	PromptTokens     int `bson:"prompt_tokens"`
	CompletionTokens int `bson:"completion_tokens"`
	TotalTokens      int `bson:"total_tokens"`
}

// SummaryMetadata contiene metadata adicional del resumen
type SummaryMetadata struct {
	SourceLength int  `bson:"source_length"` // Longitud del texto fuente
	HasImages    bool `bson:"has_images"`    // Si el material tiene imágenes
}

// NewMaterialSummary crea una nueva instancia de MaterialSummary con valores por defecto
func NewMaterialSummary(materialID, summary string, keyPoints []string, language, aiModel string) *MaterialSummary {
	now := time.Now()
	return &MaterialSummary{
		MaterialID:   materialID,
		Summary:      summary,
		KeyPoints:    keyPoints,
		Language:     language,
		WordCount:    countWords(summary),
		Version:      1,
		AIModel:      aiModel,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// countWords cuenta las palabras en un string
func countWords(text string) int {
	if text == "" {
		return 0
	}
	// Implementación simple: dividir por espacios
	// TODO: Mejorar para manejar múltiples espacios y caracteres especiales
	words := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return words
}

// IsValid valida que la entidad cumpla con las reglas de negocio
func (ms *MaterialSummary) IsValid() bool {
	if ms.MaterialID == "" {
		return false
	}
	if ms.Summary == "" || len(ms.Summary) < 10 || len(ms.Summary) > 5000 {
		return false
	}
	if len(ms.KeyPoints) < 1 || len(ms.KeyPoints) > 10 {
		return false
	}
	if ms.Language != "es" && ms.Language != "en" && ms.Language != "pt" {
		return false
	}
	if ms.WordCount < 1 {
		return false
	}
	if ms.Version < 1 {
		return false
	}
	if ms.AIModel != "gpt-4" && ms.AIModel != "gpt-3.5-turbo" && ms.AIModel != "gpt-4-turbo" && ms.AIModel != "gpt-4o" {
		return false
	}
	if ms.ProcessingTimeMs < 0 {
		return false
	}
	return true
}

// IncrementVersion incrementa la versión del resumen
func (ms *MaterialSummary) IncrementVersion() {
	ms.Version++
	ms.UpdatedAt = time.Now()
}
