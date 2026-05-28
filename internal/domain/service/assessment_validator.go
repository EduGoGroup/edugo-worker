package service

import (
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// AssessmentValidator proporciona validación y cálculos de negocio para MaterialAssessment
// Extrae la lógica de negocio que antes estaba en la entity
type AssessmentValidator struct{}

// NewAssessmentValidator crea una nueva instancia de AssessmentValidator
func NewAssessmentValidator() *AssessmentValidator {
	return &AssessmentValidator{}
}

// IsValid valida que el assessment cumpla con las reglas de negocio
func (v *AssessmentValidator) IsValid(assessment *mongoentities.MaterialAssessment) bool {
	if assessment.MaterialID == "" {
		return false
	}
	if len(assessment.Questions) < 3 || len(assessment.Questions) > 20 {
		return false
	}
	if assessment.TotalQuestions != len(assessment.Questions) {
		return false
	}
	if assessment.TotalPoints < 1 {
		return false
	}
	if assessment.Version < 1 {
		return false
	}
	if !v.isValidAIModel(assessment.AIModel) {
		return false
	}

	// Validar cada pregunta
	for _, q := range assessment.Questions {
		if !v.isValidQuestion(&q) {
			return false
		}
	}

	return true
}

// isValidAIModel valida que el modelo de IA sea válido
func (v *AssessmentValidator) isValidAIModel(model string) bool {
	validModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"}
	for _, m := range validModels {
		if model == m {
			return true
		}
	}
	return false
}

// isValidQuestion valida que una pregunta sea válida
func (v *AssessmentValidator) isValidQuestion(q *mongoentities.Question) bool {
	if q.QuestionID == "" || q.QuestionText == "" {
		return false
	}
	if q.QuestionType != "multiple_choice" && q.QuestionType != "true_false" && q.QuestionType != "open" {
		return false
	}
	if q.CorrectAnswer == "" {
		return false
	}
	if q.Points < 1 {
		return false
	}
	if q.Difficulty != "easy" && q.Difficulty != "medium" && q.Difficulty != "hard" {
		return false
	}

	// Para multiple_choice y true_false debe tener opciones
	if q.QuestionType == "multiple_choice" || q.QuestionType == "true_false" {
		if len(q.Options) < 2 {
			return false
		}
	}

	return true
}

// CalculateAverageDifficulty calcula la dificultad promedio del assessment
func (v *AssessmentValidator) CalculateAverageDifficulty(assessment *mongoentities.MaterialAssessment) string {
	if len(assessment.Questions) == 0 {
		return "medium"
	}

	difficultyScore := 0
	for _, q := range assessment.Questions {
		switch q.Difficulty {
		case "easy":
			difficultyScore += 1
		case "medium":
			difficultyScore += 2
		case "hard":
			difficultyScore += 3
		}
	}

	avg := float64(difficultyScore) / float64(len(assessment.Questions))
	if avg <= 1.5 {
		return "easy"
	} else if avg <= 2.5 {
		return "medium"
	}
	return "hard"
}

// IncrementVersion incrementa la versión del assessment
func (v *AssessmentValidator) IncrementVersion(assessment *mongoentities.MaterialAssessment) {
	assessment.Version++
	assessment.UpdatedAt = time.Now()
}

// CalculateTotalPoints calcula el total de puntos del assessment
func (v *AssessmentValidator) CalculateTotalPoints(questions []mongoentities.Question) int {
	totalPoints := 0
	for _, q := range questions {
		totalPoints += q.Points
	}
	return totalPoints
}
