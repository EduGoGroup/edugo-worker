// Package api implementa llm.LLMProvider contra un proveedor de LLM por API
// (variante "api" de D-039.3). Implementa Anthropic (Claude) de forma completa y
// deja Gemini como stub explícito: la interfaz no cambia cuando se complete
// (decisión de costo/calidad diferida al encender 040/041, design 039 §6).
//
// Credenciales/URL/modelo entran por Config (inyectado desde bootstrap/config);
// el provider NUNCA lee env directo (D-039.3).
package api

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

// Proveedores soportados por el campo Config.Provider.
const (
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
	defaultMaxTokens        = 4096
)

// Config configura el provider por API.
type Config struct {
	// Provider selecciona el backend: "anthropic" o "gemini".
	Provider string
	// APIKey es la credencial del proveedor (Secret Manager en cloud).
	APIKey string
	// Model a usar (ej. "claude-sonnet-5", "gemini-2.0-flash").
	Model string
	// BaseURL opcional (default por proveedor). Útil para tests/proxies.
	BaseURL string
	// Timeout de la request HTTP. Default 60s.
	Timeout time.Duration
	// MaxTokens del completion. Default 4096.
	MaxTokens int
}

// Provider es la implementación por API de llm.LLMProvider.
type Provider struct {
	cfg        Config
	httpClient *http.Client
}

// New construye el provider por API. Devuelve error si el proveedor no está
// soportado, para fallar temprano en bootstrap en vez de en la primera llamada.
func New(cfg Config) (*Provider, error) {
	switch cfg.Provider {
	case ProviderAnthropic:
		if cfg.BaseURL == "" {
			cfg.BaseURL = defaultAnthropicBaseURL
		}
	case ProviderGemini:
		// Stub: se acepta la construcción para no bloquear el wiring, pero las
		// llamadas devuelven error claro hasta que se implemente.
	default:
		return nil, fmt.Errorf("proveedor LLM API no soportado: %q (soportados: %q, %q)", cfg.Provider, ProviderAnthropic, ProviderGemini)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Provider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Name identifica al provider.
func (p *Provider) Name() string { return "api:" + p.cfg.Provider + ":" + p.cfg.Model }

// GenerateAssessment pide un JSON del contrato assessment_import v1.
func (p *Provider) GenerateAssessment(ctx context.Context, material llm.MaterialInput, params llm.GenerationParams) (json.RawMessage, error) {
	prompt := llm.BuildGenerationPrompt(material, params)
	out, err := p.complete(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return llm.ExtractJSON(out)
}

// ReviewAnswer pide la corrección de una respuesta.
func (p *Provider) ReviewAnswer(ctx context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	prompt := llm.BuildReviewPrompt(req)
	out, err := p.complete(ctx, prompt)
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

// complete enruta al backend concreto.
func (p *Provider) complete(ctx context.Context, prompt string) (string, error) {
	switch p.cfg.Provider {
	case ProviderAnthropic:
		return p.completeAnthropic(ctx, prompt)
	case ProviderGemini:
		return "", fmt.Errorf("provider gemini aún no implementado (039 F4 entrega anthropic; gemini se completa al encender 040/041)")
	default:
		return "", fmt.Errorf("proveedor no soportado: %q", p.cfg.Provider)
	}
}

// ---- Anthropic Messages API ----

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *anthropicError         `json:"error,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (p *Provider) completeAnthropic(ctx context.Context, prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     p.cfg.Model,
		MaxTokens: p.cfg.MaxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling anthropic request: %w", err)
	}

	url := p.cfg.BaseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading anthropic response: %w", err)
	}

	var ar anthropicResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return "", fmt.Errorf("parsing anthropic response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if ar.Error != nil {
			return "", fmt.Errorf("anthropic error (status %d): %s: %s", resp.StatusCode, ar.Error.Type, ar.Error.Message)
		}
		return "", fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sb strings.Builder
	for _, block := range ar.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("anthropic devolvió una respuesta sin bloques de texto")
	}
	return sb.String(), nil
}
