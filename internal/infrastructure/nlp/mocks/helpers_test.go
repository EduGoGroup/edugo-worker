package mocks

import (
	"context"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/stretchr/testify/assert"
)

func TestNewSuccessfulMockClient(t *testing.T) {
	mockClient := NewSuccessfulMockClient(t)
	ctx := context.Background()

	// Test GenerateSummary
	summary, err := mockClient.GenerateSummary(ctx, "texto de prueba")
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.NotEmpty(t, summary.MainIdeas)
	assert.NotEmpty(t, summary.KeyConcepts)

	// Test GenerateQuiz
	quiz, err := mockClient.GenerateQuiz(ctx, "texto de prueba", 5)
	assert.NoError(t, err)
	assert.NotNil(t, quiz)
	assert.Len(t, quiz.Questions, 5)

	// Test HealthCheck
	err = mockClient.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestNewFailingMockClient(t *testing.T) {
	customErr := errors.New("custom NLP error")
	mockClient := NewFailingMockClient(t, customErr)
	ctx := context.Background()

	// Test GenerateSummary falla
	summary, err := mockClient.GenerateSummary(ctx, "texto de prueba")
	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
	assert.Nil(t, summary)

	// Test GenerateQuiz falla
	quiz, err := mockClient.GenerateQuiz(ctx, "texto de prueba", 5)
	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
	assert.Nil(t, quiz)

	// Test HealthCheck falla
	err = mockClient.HealthCheck(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
}

func TestNewTimeoutMockClient(t *testing.T) {
	mockClient := NewTimeoutMockClient(t)
	ctx := context.Background()

	summary, err := mockClient.GenerateSummary(ctx, "texto de prueba")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, summary)
}

func TestWithSummaryResponse(t *testing.T) {
	mockClient := NewMockClient(t)
	customSummary := &nlp.Summary{
		MainIdeas: []string{"Idea personalizada"},
		WordCount: 100,
	}
	text := "texto específico"

	WithSummaryResponse(mockClient, text, customSummary, nil)
	ctx := context.Background()

	summary, err := mockClient.GenerateSummary(ctx, text)
	assert.NoError(t, err)
	assert.Equal(t, customSummary, summary)
}

func TestWithQuizResponse(t *testing.T) {
	mockClient := NewMockClient(t)
	customQuiz := &nlp.Quiz{
		Questions: []nlp.Question{
			{ID: "q1", QuestionText: "Pregunta personalizada"},
		},
	}
	text := "texto específico"
	questionCount := 1

	WithQuizResponse(mockClient, text, questionCount, customQuiz, nil)
	ctx := context.Background()

	quiz, err := mockClient.GenerateQuiz(ctx, text, questionCount)
	assert.NoError(t, err)
	assert.Equal(t, customQuiz, quiz)
}

func TestNewFlakeyMockClient(t *testing.T) {
	failCount := 2
	customErr := errors.New("temporary error")
	mockClient := NewFlakeyMockClient(t, failCount, customErr)
	ctx := context.Background()
	text := "texto de prueba"

	// Primeras 2 llamadas a GenerateSummary deben fallar
	for i := 0; i < failCount; i++ {
		summary, err := mockClient.GenerateSummary(ctx, text)
		assert.Error(t, err, "intento %d debería fallar", i+1)
		assert.ErrorIs(t, err, customErr)
		assert.Nil(t, summary)
	}

	// Tercera llamada debe tener éxito
	summary, err := mockClient.GenerateSummary(ctx, text)
	assert.NoError(t, err)
	assert.NotNil(t, summary)

	// Primeras 2 llamadas a GenerateQuiz deben fallar
	for i := 0; i < failCount; i++ {
		quiz, err := mockClient.GenerateQuiz(ctx, text, 5)
		assert.Error(t, err, "intento %d debería fallar", i+1)
		assert.ErrorIs(t, err, customErr)
		assert.Nil(t, quiz)
	}

	// Tercera llamada debe tener éxito
	quiz, err := mockClient.GenerateQuiz(ctx, text, 5)
	assert.NoError(t, err)
	assert.NotNil(t, quiz)

	// HealthCheck siempre debe funcionar
	err = mockClient.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestCreateMockSummary(t *testing.T) {
	summary := CreateMockSummary()

	assert.NotNil(t, summary)
	assert.NotEmpty(t, summary.MainIdeas)
	assert.NotEmpty(t, summary.KeyConcepts)
	assert.NotEmpty(t, summary.Sections)
	assert.Greater(t, summary.WordCount, 0)
	assert.False(t, summary.GeneratedAt.IsZero())
}

func TestCreateMockQuiz(t *testing.T) {
	questionCount := 5
	quiz := CreateMockQuiz(questionCount)

	assert.NotNil(t, quiz)
	assert.Len(t, quiz.Questions, questionCount)
	assert.False(t, quiz.GeneratedAt.IsZero())

	// Verificar que las preguntas tienen IDs únicos
	for i, q := range quiz.Questions {
		assert.NotEmpty(t, q.ID)
		assert.NotEmpty(t, q.QuestionText)
		assert.NotEmpty(t, q.Options)
		assert.Greater(t, len(q.Options), 0)

		// Verificar que la dificultad varía según el índice
		if i%3 == 0 {
			assert.Equal(t, "easy", q.Difficulty)
		} else if i%3 == 1 {
			assert.Equal(t, "medium", q.Difficulty)
		} else {
			assert.Equal(t, "hard", q.Difficulty)
		}
	}
}

func TestCreateCustomMockQuiz(t *testing.T) {
	customQuestions := []nlp.Question{
		{ID: "q1", QuestionText: "Pregunta 1"},
		{ID: "q2", QuestionText: "Pregunta 2"},
	}

	quiz := CreateCustomMockQuiz(customQuestions)

	assert.NotNil(t, quiz)
	assert.Equal(t, customQuestions, quiz.Questions)
	assert.False(t, quiz.GeneratedAt.IsZero())
}

func TestCreateMockQuestion(t *testing.T) {
	id := "test-q1"
	text := "¿Pregunta de prueba?"
	qType := "multiple_choice"
	options := []string{"A", "B", "C", "D"}
	correctAnswer := "A"

	question := CreateMockQuestion(id, text, qType, options, correctAnswer)

	assert.Equal(t, id, question.ID)
	assert.Equal(t, text, question.QuestionText)
	assert.Equal(t, qType, question.QuestionType)
	assert.Equal(t, options, question.Options)
	assert.Equal(t, correctAnswer, question.CorrectAnswer)
	assert.NotEmpty(t, question.Explanation)
	assert.Equal(t, "medium", question.Difficulty)
}

func TestCreateMockSection(t *testing.T) {
	title := "Sección de Prueba"
	content := "Contenido de la sección"
	points := []string{"Punto 1", "Punto 2"}

	section := CreateMockSection(title, content, points)

	assert.Equal(t, title, section.Title)
	assert.Equal(t, content, section.Content)
	assert.Equal(t, points, section.Points)
}

func TestCreateCustomMockSummary(t *testing.T) {
	mainIdeas := []string{"Idea 1", "Idea 2"}
	keyConcepts := map[string]string{"concepto": "definición"}
	sections := []nlp.Section{
		{Title: "Sección 1", Content: "Contenido 1"},
	}

	summary := CreateCustomMockSummary(mainIdeas, keyConcepts, sections)

	assert.NotNil(t, summary)
	assert.Equal(t, mainIdeas, summary.MainIdeas)
	assert.Equal(t, keyConcepts, summary.KeyConcepts)
	assert.Equal(t, sections, summary.Sections)
	assert.NotNil(t, summary.Glossary)
	assert.Greater(t, summary.WordCount, 0)
	assert.False(t, summary.GeneratedAt.IsZero())
}
