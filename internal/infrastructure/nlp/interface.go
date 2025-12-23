package nlp

import (
	"context"
	"time"
)

// Client define la interfaz para clientes de procesamiento de lenguaje natural
// Permite abstraer OpenAI, Claude, Gemini, Fallback, etc.
type Client interface {
	// GenerateSummary genera un resumen del texto proporcionado
	GenerateSummary(ctx context.Context, text string) (*Summary, error)

	// GenerateQuiz genera un quiz basado en el texto
	GenerateQuiz(ctx context.Context, text string, questionCount int) (*Quiz, error)

	// HealthCheck verifica la salud del servicio
	HealthCheck(ctx context.Context) error
}

// Summary representa el resumen generado de un material
type Summary struct {
	MainIdeas   []string          `json:"main_ideas"`
	KeyConcepts map[string]string `json:"key_concepts"`
	Sections    []Section         `json:"sections"`
	Glossary    map[string]string `json:"glossary"`
	WordCount   int               `json:"word_count"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// Section representa una sección del resumen
type Section struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Points  []string `json:"points"`
}

// Quiz representa un cuestionario generado
type Quiz struct {
	Questions   []Question `json:"questions"`
	GeneratedAt time.Time  `json:"generated_at"`
}

// Question representa una pregunta del quiz
type Question struct {
	ID            string   `json:"id"`
	QuestionText  string   `json:"question_text"`
	QuestionType  string   `json:"question_type"` // "multiple_choice", "true_false", "open"
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
	Difficulty    string   `json:"difficulty"` // "easy", "medium", "hard"
	Points        int      `json:"points"`
}

// Config contiene la configuración para clientes NLP
type Config struct {
	Provider    string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
}
