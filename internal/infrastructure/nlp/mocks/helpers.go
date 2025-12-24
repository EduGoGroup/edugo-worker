package mocks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/stretchr/testify/mock"
)

// NewSuccessfulMockClient crea un mock NLP que siempre tiene éxito
func NewSuccessfulMockClient(t *testing.T) *MockClient {
	mockClient := NewMockClient(t)

	// Mock para GenerateSummary
	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(CreateMockSummary(), nil).
		Maybe()

	// Mock para GenerateQuiz
	mockClient.On("GenerateQuiz", mock.Anything, mock.Anything, mock.Anything).
		Return(CreateMockQuiz(5), nil).
		Maybe()

	// Mock para HealthCheck
	mockClient.On("HealthCheck", mock.Anything).
		Return(nil).
		Maybe()

	return mockClient
}

// NewFailingMockClient crea un mock NLP que siempre falla
func NewFailingMockClient(t *testing.T, err error) *MockClient {
	mockClient := NewMockClient(t)

	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(nil, err).
		Maybe()

	mockClient.On("GenerateQuiz", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, err).
		Maybe()

	mockClient.On("HealthCheck", mock.Anything).
		Return(err).
		Maybe()

	return mockClient
}

// NewTimeoutMockClient crea un mock que simula timeout
func NewTimeoutMockClient(t *testing.T) *MockClient {
	return NewFailingMockClient(t, context.DeadlineExceeded)
}

// WithSummaryResponse configura un mock para retornar un resumen específico
func WithSummaryResponse(mockClient *MockClient, text string, summary *nlp.Summary, err error) *MockClient {
	mockClient.On("GenerateSummary", mock.Anything, text).
		Return(summary, err).
		Once()
	return mockClient
}

// WithQuizResponse configura un mock para retornar un quiz específico
func WithQuizResponse(mockClient *MockClient, text string, questionCount int, quiz *nlp.Quiz, err error) *MockClient {
	mockClient.On("GenerateQuiz", mock.Anything, text, questionCount).
		Return(quiz, err).
		Once()
	return mockClient
}

// NewFlakeyMockClient crea un mock que falla las primeras N veces y luego tiene éxito
// Útil para testing de retry logic
func NewFlakeyMockClient(t *testing.T, failCount int, failErr error) *MockClient {
	mockClient := NewMockClient(t)

	// GenerateSummary falla N veces
	for i := 0; i < failCount; i++ {
		mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
			Return(nil, failErr).
			Once()
	}
	mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
		Return(CreateMockSummary(), nil).
		Maybe()

	// GenerateQuiz falla N veces
	for i := 0; i < failCount; i++ {
		mockClient.On("GenerateQuiz", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, failErr).
			Once()
	}
	mockClient.On("GenerateQuiz", mock.Anything, mock.Anything, mock.Anything).
		Return(CreateMockQuiz(5), nil).
		Maybe()

	// HealthCheck siempre funciona
	mockClient.On("HealthCheck", mock.Anything).
		Return(nil).
		Maybe()

	return mockClient
}

// CreateMockSummary crea un resumen de prueba
func CreateMockSummary() *nlp.Summary {
	return &nlp.Summary{
		MainIdeas: []string{
			"Primera idea principal del contenido",
			"Segunda idea principal del contenido",
			"Tercera idea principal del contenido",
		},
		KeyConcepts: map[string]string{
			"concepto1": "Definición del primer concepto",
			"concepto2": "Definición del segundo concepto",
			"concepto3": "Definición del tercer concepto",
		},
		Sections: []nlp.Section{
			{
				Title:   "Introducción",
				Content: "Contenido de la introducción",
				Points:  []string{"Punto 1", "Punto 2"},
			},
			{
				Title:   "Desarrollo",
				Content: "Contenido del desarrollo",
				Points:  []string{"Punto 3", "Punto 4"},
			},
			{
				Title:   "Conclusión",
				Content: "Contenido de la conclusión",
				Points:  []string{"Punto 5", "Punto 6"},
			},
		},
		Glossary: map[string]string{
			"término1": "Definición del término 1",
			"término2": "Definición del término 2",
		},
		WordCount:   500,
		GeneratedAt: time.Now(),
	}
}

// CreateMockQuiz crea un quiz de prueba con N preguntas
func CreateMockQuiz(questionCount int) *nlp.Quiz {
	questions := make([]nlp.Question, questionCount)

	for i := 0; i < questionCount; i++ {
		var difficulty string
		switch i % 3 {
		case 1:
			difficulty = "medium"
		case 2:
			difficulty = "hard"
		default:
			difficulty = "easy"
		}

		questions[i] = nlp.Question{
			ID:           fmt.Sprintf("question-%d", i+1),
			QuestionText: fmt.Sprintf("¿Pregunta de prueba número %d?", i+1),
			QuestionType: "multiple_choice",
			Options: []string{
				"Opción A",
				"Opción B",
				"Opción C",
				"Opción D",
			},
			CorrectAnswer: "Opción A",
			Explanation:   "Esta es la explicación de por qué la respuesta es correcta",
			Difficulty:    difficulty,
			Points:        10,
		}
	}

	return &nlp.Quiz{
		Questions:   questions,
		GeneratedAt: time.Now(),
	}
}

// CreateCustomMockQuiz crea un quiz personalizado
func CreateCustomMockQuiz(questions []nlp.Question) *nlp.Quiz {
	return &nlp.Quiz{
		Questions:   questions,
		GeneratedAt: time.Now(),
	}
}

// CreateMockQuestion crea una pregunta de prueba
func CreateMockQuestion(id, text, qType string, options []string, correctAnswer string) nlp.Question {
	return nlp.Question{
		ID:            id,
		QuestionText:  text,
		QuestionType:  qType,
		Options:       options,
		CorrectAnswer: correctAnswer,
		Explanation:   "Explicación de la respuesta",
		Difficulty:    "medium",
		Points:        10,
	}
}

// CreateMockSection crea una sección de resumen de prueba
func CreateMockSection(title, content string, points []string) nlp.Section {
	return nlp.Section{
		Title:   title,
		Content: content,
		Points:  points,
	}
}

// CreateCustomMockSummary crea un resumen personalizado
func CreateCustomMockSummary(mainIdeas []string, keyConcepts map[string]string, sections []nlp.Section) *nlp.Summary {
	return &nlp.Summary{
		MainIdeas:   mainIdeas,
		KeyConcepts: keyConcepts,
		Sections:    sections,
		Glossary:    make(map[string]string),
		WordCount:   100,
		GeneratedAt: time.Now(),
	}
}
