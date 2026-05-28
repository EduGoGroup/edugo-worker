package openai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implementa logger.Logger para testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...interface{})  {}
func (m *mockLogger) Info(msg string, fields ...interface{})   {}
func (m *mockLogger) Warn(msg string, fields ...interface{})   {}
func (m *mockLogger) Error(msg string, fields ...interface{})  {}
func (m *mockLogger) Fatal(msg string, fields ...interface{})  {}
func (m *mockLogger) With(fields ...interface{}) logger.Logger { return m }
func (m *mockLogger) Sync() error                              { return nil }

func createTestLogger() logger.Logger {
	return &mockLogger{}
}

func TestNewClient(t *testing.T) {
	log := createTestLogger()

	tests := []struct {
		name        string
		config      Config
		expectError error
	}{
		{
			name: "configuración válida con GPT-4",
			config: Config{
				APIKey:      "sk-test-key-123",
				Model:       ModelGPT4,
				MaxTokens:   1000,
				Temperature: 0.5,
				Timeout:     30 * time.Second,
			},
			expectError: nil,
		},
		{
			name: "configuración válida con GPT-3.5-turbo",
			config: Config{
				APIKey: "sk-test-key-456",
				Model:  ModelGPT35Turbo,
			},
			expectError: nil,
		},
		{
			name: "API key vacía",
			config: Config{
				APIKey: "",
				Model:  ModelGPT4,
			},
			expectError: ErrInvalidAPIKey,
		},
		{
			name: "API key solo espacios",
			config: Config{
				APIKey: "   ",
				Model:  ModelGPT4,
			},
			expectError: ErrInvalidAPIKey,
		},
		{
			name: "modelo inválido",
			config: Config{
				APIKey: "sk-test-key",
				Model:  "gpt-invalid",
			},
			expectError: ErrInvalidModel,
		},
		{
			name: "valores por defecto aplicados",
			config: Config{
				APIKey:      "sk-test-key",
				Model:       ModelGPT35Turbo,
				MaxTokens:   0, // debe usar defaultMaxTokens
				Temperature: 0, // debe usar defaultTemperature
				Timeout:     0, // debe usar defaultTimeout
			},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, log)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectError)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Verificar que se aplicaron valores por defecto si corresponde
				if tt.config.MaxTokens == 0 || tt.config.Temperature == 0 || tt.config.Timeout == 0 {
					c := client.(*Client)
					if tt.config.MaxTokens == 0 {
						assert.Equal(t, defaultMaxTokens, c.maxTokens)
					}
					if tt.config.Temperature == 0 {
						assert.Equal(t, defaultTemperature, c.temperature)
					}
					if tt.config.Timeout == 0 {
						assert.Equal(t, defaultTimeout, c.timeout)
					}
				}
			}
		})
	}
}

func TestClient_validateText(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		text        string
		expectError error
	}{
		{
			name:        "texto válido",
			text:        strings.Repeat("Este es un texto de prueba. ", 10),
			expectError: nil,
		},
		{
			name:        "texto vacío",
			text:        "",
			expectError: ErrEmptyText,
		},
		{
			name:        "texto solo espacios",
			text:        "   \n\t   ",
			expectError: ErrEmptyText,
		},
		{
			name:        "texto muy corto",
			text:        "Corto",
			expectError: ErrEmptyText,
		},
		{
			name:        "texto en el límite mínimo (50 caracteres)",
			text:        strings.Repeat("a", 50),
			expectError: nil,
		},
		{
			name:        "texto en el límite máximo (50000 caracteres)",
			text:        strings.Repeat("a", maxTextLength),
			expectError: nil,
		},
		{
			name:        "texto demasiado largo",
			text:        strings.Repeat("a", maxTextLength+1),
			expectError: ErrTextTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateText(tt.text)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_validateQuestionCount(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		count       int
		expectError bool
	}{
		{
			name:        "count válido (5)",
			count:       5,
			expectError: false,
		},
		{
			name:        "count mínimo (1)",
			count:       1,
			expectError: false,
		},
		{
			name:        "count máximo (50)",
			count:       50,
			expectError: false,
		},
		{
			name:        "count cero",
			count:       0,
			expectError: true,
		},
		{
			name:        "count negativo",
			count:       -5,
			expectError: true,
		},
		{
			name:        "count demasiado grande",
			count:       51,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateQuestionCount(tt.count)

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidQuestionCount)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_buildSummaryPrompt(t *testing.T) {
	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)
	require.NoError(t, err)

	c := client.(*Client)
	text := "Este es un texto de prueba sobre educación."

	prompt := c.buildSummaryPrompt(text)

	assert.Contains(t, prompt, text)
	assert.Contains(t, prompt, "main_ideas")
	assert.Contains(t, prompt, "key_concepts")
	assert.Contains(t, prompt, "sections")
	assert.Contains(t, prompt, "glossary")
	assert.Contains(t, prompt, "JSON")
}

func TestClient_buildQuizPrompt(t *testing.T) {
	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)
	require.NoError(t, err)

	c := client.(*Client)
	text := "Este es un texto de prueba sobre educación."
	questionCount := 5

	prompt := c.buildQuizPrompt(text, questionCount)

	assert.Contains(t, prompt, text)
	assert.Contains(t, prompt, "5 preguntas")
	assert.Contains(t, prompt, "question_text")
	assert.Contains(t, prompt, "question_type")
	assert.Contains(t, prompt, "correct_answer")
	assert.Contains(t, prompt, "JSON")
}

func TestClient_GenerateSummary_Validation(t *testing.T) {
	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		text        string
		expectError error
	}{
		{
			name:        "texto vacío",
			text:        "",
			expectError: ErrEmptyText,
		},
		{
			name:        "texto muy corto",
			text:        "Muy corto",
			expectError: ErrEmptyText,
		},
		{
			name:        "texto demasiado largo",
			text:        strings.Repeat("a", maxTextLength+1),
			expectError: ErrTextTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GenerateSummary(ctx, tt.text)

			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expectError)
		})
	}
}

func TestClient_GenerateQuiz_Validation(t *testing.T) {
	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)
	require.NoError(t, err)

	ctx := context.Background()
	validText := strings.Repeat("Este es un texto válido para generar quiz. ", 20)

	tests := []struct {
		name          string
		text          string
		questionCount int
		expectError   error
	}{
		{
			name:          "texto vacío",
			text:          "",
			questionCount: 5,
			expectError:   ErrEmptyText,
		},
		{
			name:          "question count cero",
			text:          validText,
			questionCount: 0,
			expectError:   ErrInvalidQuestionCount,
		},
		{
			name:          "question count negativo",
			text:          validText,
			questionCount: -1,
			expectError:   ErrInvalidQuestionCount,
		},
		{
			name:          "question count demasiado grande",
			text:          validText,
			questionCount: 51,
			expectError:   ErrInvalidQuestionCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GenerateQuiz(ctx, tt.text, tt.questionCount)

			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expectError)
		})
	}
}

func TestClient_HealthCheck(t *testing.T) {
	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)
	require.NoError(t, err)

	ctx := context.Background()
	err = client.HealthCheck(ctx)

	// Por ahora el health check siempre retorna nil (modo preparación)
	assert.NoError(t, err)
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "JSON simple sin markdown",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON con markdown code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON con markdown sin json tag",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON con espacios",
			input:    "  \n  {\"key\": \"value\"}  \n  ",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanJSONResponse(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "GPT-4 Turbo válido",
			model:    ModelGPT4Turbo,
			expected: true,
		},
		{
			name:     "GPT-4 válido",
			model:    ModelGPT4,
			expected: true,
		},
		{
			name:     "GPT-3.5 Turbo válido",
			model:    ModelGPT35Turbo,
			expected: true,
		},
		{
			name:     "GPT-3.5 Turbo 16k válido",
			model:    ModelGPT35Turbo16k,
			expected: true,
		},
		{
			name:     "modelo inválido",
			model:    "gpt-invalid",
			expected: false,
		},
		{
			name:     "string vacío",
			model:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidModel(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_InterfaceCompliance(t *testing.T) {
	// Este test verifica que Client implemente correctamente nlp.Client
	// La compilación fallará si no implementa todos los métodos

	log := createTestLogger()
	client, err := NewClient(Config{
		APIKey: "sk-test",
		Model:  ModelGPT4,
	}, log)

	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verificar que tiene todos los métodos de la interfaz
	ctx := context.Background()

	// Estos métodos deben existir y ser llamables
	_, err = client.GenerateSummary(ctx, strings.Repeat("texto ", 20))
	assert.Error(t, err) // Esperamos error porque no hay API real

	_, err = client.GenerateQuiz(ctx, strings.Repeat("texto ", 20), 5)
	assert.Error(t, err) // Esperamos error porque no hay API real

	err = client.HealthCheck(ctx)
	assert.NoError(t, err) // Health check siempre retorna nil en modo preparación
}
