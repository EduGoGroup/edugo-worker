package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
)

var (
	// ErrInvalidAPIKey se retorna cuando la API key es inválida o está vacía
	ErrInvalidAPIKey = errors.New("API key de OpenAI inválida o vacía")

	// ErrInvalidModel se retorna cuando el modelo especificado no es válido
	ErrInvalidModel = errors.New("modelo de OpenAI inválido")

	// ErrEmptyText se retorna cuando el texto a procesar está vacío
	ErrEmptyText = errors.New("texto vacío, no se puede procesar")

	// ErrTextTooLong se retorna cuando el texto excede el límite
	ErrTextTooLong = errors.New("texto demasiado largo para procesar")

	// ErrInvalidQuestionCount se retorna cuando el número de preguntas es inválido
	ErrInvalidQuestionCount = errors.New("número de preguntas inválido (debe ser entre 1 y 50)")

	// ErrAPITimeout se retorna cuando la API no responde en el tiempo esperado
	ErrAPITimeout = errors.New("timeout esperando respuesta de OpenAI")

	// ErrAPIRateLimit se retorna cuando se excede el límite de tasa
	ErrAPIRateLimit = errors.New("límite de tasa excedido en OpenAI API")

	// ErrAPIQuotaExceeded se retorna cuando se excede la cuota
	ErrAPIQuotaExceeded = errors.New("cuota excedida en OpenAI API")

	// ErrAPIUnauthorized se retorna cuando la autenticación falla
	ErrAPIUnauthorized = errors.New("autenticación fallida con OpenAI API")
)

const (
	// Modelos soportados de OpenAI
	ModelGPT4Turbo     = "gpt-4-turbo-preview"
	ModelGPT4          = "gpt-4"
	ModelGPT35Turbo    = "gpt-3.5-turbo"
	ModelGPT35Turbo16k = "gpt-3.5-turbo-16k"

	// Límites de texto
	maxTextLength    = 50000 // ~50k caracteres
	minTextLength    = 50    // mínimo 50 caracteres
	maxQuestionCount = 50
	minQuestionCount = 1

	// Configuración por defecto
	defaultMaxTokens   = 2000
	defaultTemperature = 0.7
	defaultTimeout     = 60 * time.Second
)

// Client implementa nlp.Client usando OpenAI API
type Client struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	timeout     time.Duration
	logger      logger.Logger
}

// Config contiene la configuración para el cliente OpenAI
type Config struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
}

// NewClient crea una nueva instancia del cliente OpenAI
func NewClient(cfg Config, log logger.Logger) (nlp.Client, error) {
	// Validar API key
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrInvalidAPIKey
	}

	// Validar modelo
	if !isValidModel(cfg.Model) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidModel, cfg.Model)
	}

	// Valores por defecto
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	temperature := cfg.Temperature
	if temperature <= 0 {
		temperature = defaultTemperature
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	client := &Client{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   maxTokens,
		temperature: temperature,
		timeout:     timeout,
		logger:      log,
	}

	log.Info("cliente OpenAI creado",
		"model", cfg.Model,
		"maxTokens", maxTokens,
		"temperature", temperature,
		"timeout", timeout,
	)

	return client, nil
}

// GenerateSummary genera un resumen del texto usando OpenAI
func (c *Client) GenerateSummary(ctx context.Context, text string) (*nlp.Summary, error) {
	start := time.Now()
	c.logger.Info("generando resumen con OpenAI", "model", c.model, "textLength", len(text))

	// Validar entrada
	if err := c.validateText(text); err != nil {
		return nil, err
	}

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Construir prompt para resumen
	prompt := c.buildSummaryPrompt(text)

	// Llamar a OpenAI API (implementación futura)
	response, err := c.callOpenAIAPI(ctx, prompt)
	if err != nil {
		c.logger.Error("error llamando OpenAI API", "error", err, "duration", time.Since(start))
		return nil, err
	}

	// Parsear respuesta a Summary
	summary, err := c.parseSummaryResponse(response)
	if err != nil {
		c.logger.Error("error parseando respuesta", "error", err)
		return nil, err
	}

	summary.WordCount = len(strings.Fields(text))
	summary.GeneratedAt = time.Now()

	c.logger.Info("resumen generado exitosamente",
		"duration", time.Since(start),
		"mainIdeas", len(summary.MainIdeas),
		"concepts", len(summary.KeyConcepts),
		"sections", len(summary.Sections),
	)

	return summary, nil
}

// GenerateQuiz genera un quiz usando OpenAI
func (c *Client) GenerateQuiz(ctx context.Context, text string, questionCount int) (*nlp.Quiz, error) {
	start := time.Now()
	c.logger.Info("generando quiz con OpenAI",
		"model", c.model,
		"textLength", len(text),
		"questionCount", questionCount,
	)

	// Validar entrada
	if err := c.validateText(text); err != nil {
		return nil, err
	}

	if err := c.validateQuestionCount(questionCount); err != nil {
		return nil, err
	}

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Construir prompt para quiz
	prompt := c.buildQuizPrompt(text, questionCount)

	// Llamar a OpenAI API (implementación futura)
	response, err := c.callOpenAIAPI(ctx, prompt)
	if err != nil {
		c.logger.Error("error llamando OpenAI API", "error", err, "duration", time.Since(start))
		return nil, err
	}

	// Parsear respuesta a Quiz
	quiz, err := c.parseQuizResponse(response)
	if err != nil {
		c.logger.Error("error parseando respuesta quiz", "error", err)
		return nil, err
	}

	quiz.GeneratedAt = time.Now()

	c.logger.Info("quiz generado exitosamente",
		"duration", time.Since(start),
		"questions", len(quiz.Questions),
	)

	return quiz, nil
}

// HealthCheck verifica la salud del servicio OpenAI
func (c *Client) HealthCheck(ctx context.Context) error {
	c.logger.Debug("ejecutando health check de OpenAI")

	// Crear contexto con timeout corto
	// TODO: Usar ctxWithTimeout cuando se implemente la llamada real a la API
	_, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Intentar una llamada simple a la API
	// Por ahora retornamos nil (implementación futura con API real)
	c.logger.Debug("health check OpenAI: OK (modo preparación)")
	return nil
}

// validateText valida que el texto sea adecuado para procesamiento
func (c *Client) validateText(text string) error {
	trimmed := strings.TrimSpace(text)

	if trimmed == "" {
		return ErrEmptyText
	}

	if len(trimmed) < minTextLength {
		return fmt.Errorf("%w: mínimo %d caracteres, recibido %d", ErrEmptyText, minTextLength, len(trimmed))
	}

	if len(trimmed) > maxTextLength {
		return fmt.Errorf("%w: máximo %d caracteres, recibido %d", ErrTextTooLong, maxTextLength, len(trimmed))
	}

	return nil
}

// validateQuestionCount valida el número de preguntas solicitadas
func (c *Client) validateQuestionCount(count int) error {
	if count < minQuestionCount || count > maxQuestionCount {
		return fmt.Errorf("%w: recibido %d", ErrInvalidQuestionCount, count)
	}
	return nil
}

// buildSummaryPrompt construye el prompt para generar resúmenes
func (c *Client) buildSummaryPrompt(text string) string {
	return fmt.Sprintf(`Analiza el siguiente texto educativo y genera un resumen estructurado en formato JSON.

El resumen debe incluir:
1. main_ideas: array de las 3-5 ideas principales
2. key_concepts: objeto con conceptos clave y sus definiciones
3. sections: array de secciones (título, contenido, puntos)
4. glossary: objeto con términos importantes y sus definiciones

Texto a analizar:
%s

Responde ÚNICAMENTE con un objeto JSON válido sin texto adicional.`, text)
}

// buildQuizPrompt construye el prompt para generar quizzes
func (c *Client) buildQuizPrompt(text string, questionCount int) string {
	return fmt.Sprintf(`Genera un quiz educativo de %d preguntas basado en el siguiente texto.

Cada pregunta debe tener:
- id: identificador único
- question_text: texto de la pregunta
- question_type: "multiple_choice", "true_false", o "open"
- options: array de opciones (para multiple_choice)
- correct_answer: respuesta correcta
- explanation: explicación de la respuesta
- difficulty: "easy", "medium", o "hard"
- points: puntos asignados (10, 15, o 20)

Texto a analizar:
%s

Responde ÚNICAMENTE con un objeto JSON con un campo "questions" que contenga el array de preguntas.`, questionCount, text)
}

// callOpenAIAPI llama a la API de OpenAI
// NOTA: Esta es una implementación preparada para integración futura
// Por ahora retorna un error indicando que requiere configuración
func (c *Client) callOpenAIAPI(ctx context.Context, prompt string) (string, error) {
	// TODO: Implementar integración real con OpenAI API cuando se tenga API key
	//
	// Pasos para implementación futura:
	// 1. Usar el SDK oficial de OpenAI: github.com/sashabaranov/go-openai
	// 2. Crear cliente con c.apiKey
	// 3. Construir ChatCompletionRequest con:
	//    - Model: c.model
	//    - Messages: []openai.ChatCompletionMessage con el prompt
	//    - MaxTokens: c.maxTokens
	//    - Temperature: c.temperature
	// 4. Manejar errores específicos:
	//    - Rate limit → ErrAPIRateLimit
	//    - Quota exceeded → ErrAPIQuotaExceeded
	//    - Unauthorized → ErrAPIUnauthorized
	//    - Timeout → ErrAPITimeout
	// 5. Extraer response.Choices[0].Message.Content
	// 6. Implementar retry logic con backoff exponencial

	c.logger.Warn("OpenAI API no configurada todavía - requiere API key real",
		"model", c.model,
		"promptLength", len(prompt),
	)

	return "", fmt.Errorf("OpenAI API requiere configuración con API key real - funcionalidad preparada para uso futuro")
}

// parseSummaryResponse parsea la respuesta de OpenAI a un Summary
func (c *Client) parseSummaryResponse(response string) (*nlp.Summary, error) {
	var summary nlp.Summary

	// Limpiar respuesta (remover markdown code blocks si existen)
	cleaned := cleanJSONResponse(response)

	if err := json.Unmarshal([]byte(cleaned), &summary); err != nil {
		return nil, fmt.Errorf("error parseando JSON de resumen: %w", err)
	}

	return &summary, nil
}

// parseQuizResponse parsea la respuesta de OpenAI a un Quiz
func (c *Client) parseQuizResponse(response string) (*nlp.Quiz, error) {
	var quiz nlp.Quiz

	// Limpiar respuesta (remover markdown code blocks si existen)
	cleaned := cleanJSONResponse(response)

	if err := json.Unmarshal([]byte(cleaned), &quiz); err != nil {
		return nil, fmt.Errorf("error parseando JSON de quiz: %w", err)
	}

	return &quiz, nil
}

// cleanJSONResponse limpia la respuesta removiendo markdown code blocks
func cleanJSONResponse(response string) string {
	// Remover ```json y ``` si existen
	cleaned := strings.TrimSpace(response)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

// isValidModel verifica si el modelo especificado es válido
func isValidModel(model string) bool {
	validModels := map[string]bool{
		ModelGPT4Turbo:     true,
		ModelGPT4:          true,
		ModelGPT35Turbo:    true,
		ModelGPT35Turbo16k: true,
	}
	return validModels[model]
}

// Verificar que Client implementa nlp.Client
var _ nlp.Client = (*Client)(nil)
