// Package ollama implementa llm.LLMProvider hablando con un servidor Ollama
// local (HTTP). Es la variante "local" de D-039.3: sin credenciales externas, el
// modelo corre en la máquina/host indicado por config.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// Config configura el provider Ollama. Se inyecta desde bootstrap/config; el
// provider NUNCA lee env directo (D-039.3).
type Config struct {
	// BaseURL del servidor Ollama (ej. http://localhost:11434).
	BaseURL string
	// Model a usar (ej. "llama3.1", "qwen2.5:7b").
	Model string
	// Timeout de la request HTTP. Generar puede ser lento en CPU: default 120s.
	Timeout time.Duration
}

// Provider es la implementación Ollama de llm.LLMProvider.
type Provider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// New construye el provider Ollama a partir de su config.
func New(cfg Config) *Provider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Provider{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Name identifica al provider.
func (p *Provider) Name() string { return "ollama:" + p.model }

// generateRequest es el body de POST /api/generate. format:"json" fuerza a
// Ollama a emitir JSON válido (reduce alucinaciones de formato).
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
	// Think desactiva el "thinking" de los modelos que lo soportan (qwen3, etc.).
	// Con format:"json" el razonamiento se corta y deja el objeto vacío `{}`
	// (verdict/score/feedback en cero): una review IA engañosa. Forzar think:false
	// hace que el modelo emita directamente el JSON pedido. En modelos sin thinking
	// Ollama ignora el campo (inocuo). Se envía siempre (sin omitempty) para que el
	// false explícito llegue al servidor.
	Think bool `json:"think"`
}

// generateResponse es la respuesta (con stream:false, un solo objeto).
type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// GenerateAssessment pide al modelo un JSON del contrato assessment_import v1.
func (p *Provider) GenerateAssessment(ctx context.Context, material llm.MaterialInput, params llm.GenerationParams) (json.RawMessage, error) {
	prompt := llm.BuildGenerationPrompt(material, params)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return llm.ExtractJSON(out)
}

// ReviewAnswer pide al modelo la corrección de una respuesta.
func (p *Provider) ReviewAnswer(ctx context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	prompt := llm.BuildReviewPrompt(req)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	var result llm.ReviewResult
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		return llm.ReviewResult{}, fmt.Errorf("respuesta de corrección no parseable: %w", err)
	}
	return result, nil
}

// PrepareQuestion pide al modelo el artefacto de preparación (JSON crudo del
// contrato llm_prep v1). Hereda el mismo camino que review: format:"json" +
// think:false (fix e7c70fe) para que qwen3 emita el objeto directo, sin el `{}` del
// thinking. El caller valida el JSON contra el contrato antes de persistirlo.
func (p *Provider) PrepareQuestion(ctx context.Context, req llm.PrepRequest) (json.RawMessage, error) {
	prompt := llm.BuildPrepPrompt(req)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return llm.ExtractJSON(out)
}

// JudgePairEquivalence pide la equivalencia binaria de un par (plan 042 F3c). Mismo
// camino que ReviewAnswer: el resultado es un ReviewResult (verdict/score/feedback).
func (p *Provider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	prompt := llm.BuildPairEquivalencePrompt(req)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	var result llm.ReviewResult
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		return llm.ReviewResult{}, fmt.Errorf("respuesta de equivalencia no parseable: %w", err)
	}
	return result, nil
}

// generate ejecuta POST /api/generate y devuelve el texto crudo del modelo.
func (p *Provider) generate(ctx context.Context, prompt string) (string, error) {
	reqBody := generateRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
		Think:  false,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling ollama request: %w", err)
	}

	url := p.baseURL + "/api/generate"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ollama response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var gr generateResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("parsing ollama response: %w", err)
	}
	return gr.Response, nil
}
