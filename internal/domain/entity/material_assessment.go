package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaterialAssessment representa una evaluación (quiz) generada automáticamente por IA
// Se almacena en MongoDB en la collection "material_assessment"
type MaterialAssessment struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	MaterialID      string             `bson:"material_id"`    // UUID del material en PostgreSQL
	Questions       []Question         `bson:"questions"`      // Array de preguntas (min 3, max 20)
	TotalQuestions  int                `bson:"total_questions"` // Total de preguntas
	TotalPoints     int                `bson:"total_points"`    // Puntos totales del assessment
	Version         int                `bson:"version"`         // Versión del assessment (>= 1)
	AIModel         string             `bson:"ai_model"`        // "gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"
	ProcessingTimeMs int               `bson:"processing_time_ms"` // Tiempo de procesamiento en ms
	TokenUsage      *TokenUsage        `bson:"token_usage,omitempty"` // Metadata de tokens (opcional)
	Metadata        *AssessmentMetadata `bson:"metadata,omitempty"`   // Metadata adicional (opcional)
	CreatedAt       time.Time          `bson:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at"`
}

// Question representa una pregunta del assessment
type Question struct {
	QuestionID   string   `bson:"question_id"`   // ID único de la pregunta
	QuestionText string   `bson:"question_text"` // Texto de la pregunta
	QuestionType string   `bson:"question_type"` // "multiple_choice", "true_false", "open"
	Options      []Option `bson:"options,omitempty"` // Opciones (solo para multiple_choice/true_false)
	CorrectAnswer string  `bson:"correct_answer"` // Respuesta correcta
	Explanation  string   `bson:"explanation"`    // Explicación de la respuesta
	Points       int      `bson:"points"`         // Puntos de esta pregunta
	Difficulty   string   `bson:"difficulty"`     // "easy", "medium", "hard"
	Tags         []string `bson:"tags,omitempty"` // Tags opcionales
}

// Option representa una opción de respuesta en preguntas de opción múltiple
type Option struct {
	OptionID   string `bson:"option_id"`   // ID único de la opción (A, B, C, D)
	OptionText string `bson:"option_text"` // Texto de la opción
}

// AssessmentMetadata contiene metadata adicional del assessment
type AssessmentMetadata struct {
	AverageDifficulty string `bson:"average_difficulty"` // "easy", "medium", "hard"
	EstimatedTimeMin  int    `bson:"estimated_time_min"` // Tiempo estimado en minutos
}

// NewMaterialAssessment crea una nueva instancia de MaterialAssessment
func NewMaterialAssessment(materialID string, questions []Question, aiModel string) *MaterialAssessment {
	now := time.Now()
	totalPoints := 0
	for _, q := range questions {
		totalPoints += q.Points
	}

	return &MaterialAssessment{
		MaterialID:     materialID,
		Questions:      questions,
		TotalQuestions: len(questions),
		TotalPoints:    totalPoints,
		Version:        1,
		AIModel:        aiModel,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewQuestion crea una nueva pregunta
func NewQuestion(questionID, questionText, questionType, correctAnswer, explanation string, points int, difficulty string) Question {
	return Question{
		QuestionID:   questionID,
		QuestionText: questionText,
		QuestionType: questionType,
		CorrectAnswer: correctAnswer,
		Explanation:  explanation,
		Points:       points,
		Difficulty:   difficulty,
		Options:      []Option{},
		Tags:         []string{},
	}
}

// AddOption agrega una opción a una pregunta
func (q *Question) AddOption(optionID, optionText string) {
	q.Options = append(q.Options, Option{
		OptionID:   optionID,
		OptionText: optionText,
	})
}

// IsValid valida que la entidad cumpla con las reglas de negocio
func (ma *MaterialAssessment) IsValid() bool {
	if ma.MaterialID == "" {
		return false
	}
	if len(ma.Questions) < 3 || len(ma.Questions) > 20 {
		return false
	}
	if ma.TotalQuestions != len(ma.Questions) {
		return false
	}
	if ma.TotalPoints < 1 {
		return false
	}
	if ma.Version < 1 {
		return false
	}
	if ma.AIModel != "gpt-4" && ma.AIModel != "gpt-3.5-turbo" && ma.AIModel != "gpt-4-turbo" && ma.AIModel != "gpt-4o" {
		return false
	}

	// Validar cada pregunta
	for _, q := range ma.Questions {
		if !q.IsValid() {
			return false
		}
	}

	return true
}

// IsValid valida que una pregunta sea válida
func (q *Question) IsValid() bool {
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

// IncrementVersion incrementa la versión del assessment
func (ma *MaterialAssessment) IncrementVersion() {
	ma.Version++
	ma.UpdatedAt = time.Now()
}

// CalculateAverageDifficulty calcula la dificultad promedio del assessment
func (ma *MaterialAssessment) CalculateAverageDifficulty() string {
	if len(ma.Questions) == 0 {
		return "medium"
	}

	difficultyScore := 0
	for _, q := range ma.Questions {
		switch q.Difficulty {
		case "easy":
			difficultyScore += 1
		case "medium":
			difficultyScore += 2
		case "hard":
			difficultyScore += 3
		}
	}

	avg := float64(difficultyScore) / float64(len(ma.Questions))
	if avg <= 1.5 {
		return "easy"
	} else if avg <= 2.5 {
		return "medium"
	}
	return "hard"
}
