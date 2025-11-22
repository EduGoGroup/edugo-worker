package service

import (
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// SummaryValidator proporciona validación y cálculos de negocio para MaterialSummary
// Extrae la lógica de negocio que antes estaba en la entity
type SummaryValidator struct{}

// NewSummaryValidator crea una nueva instancia de SummaryValidator
func NewSummaryValidator() *SummaryValidator {
	return &SummaryValidator{}
}

// IsValid valida que el summary cumpla con las reglas de negocio
func (v *SummaryValidator) IsValid(summary *mongoentities.MaterialSummary) bool {
	if summary.MaterialID == "" {
		return false
	}
	if summary.Summary == "" || len(summary.Summary) < 10 || len(summary.Summary) > 5000 {
		return false
	}
	if len(summary.KeyPoints) < 1 || len(summary.KeyPoints) > 10 {
		return false
	}
	if !v.isValidLanguage(summary.Language) {
		return false
	}
	if summary.WordCount < 1 {
		return false
	}
	if summary.Version < 1 {
		return false
	}
	if !v.isValidAIModel(summary.AIModel) {
		return false
	}
	if summary.ProcessingTimeMs < 0 {
		return false
	}
	return true
}

// isValidLanguage valida que el idioma sea válido
func (v *SummaryValidator) isValidLanguage(language string) bool {
	validLanguages := []string{"es", "en", "pt"}
	for _, l := range validLanguages {
		if language == l {
			return true
		}
	}
	return false
}

// isValidAIModel valida que el modelo de IA sea válido
func (v *SummaryValidator) isValidAIModel(model string) bool {
	validModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"}
	for _, m := range validModels {
		if model == m {
			return true
		}
	}
	return false
}

// CountWords cuenta las palabras en un string
func (v *SummaryValidator) CountWords(text string) int {
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

// IncrementVersion incrementa la versión del summary
func (v *SummaryValidator) IncrementVersion(summary *mongoentities.MaterialSummary) {
	summary.Version++
	summary.UpdatedAt = time.Now()
}
