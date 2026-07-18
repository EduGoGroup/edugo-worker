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
)

// EmbedConfig configura el cliente de embeddings Ollama. Se inyecta desde
// bootstrap/config; el cliente NUNCA lee env directo (D-039.3). No hay temperatura:
// embeder es determinista por definición (no muestrea).
type EmbedConfig struct {
	// BaseURL del servidor Ollama (ej. http://localhost:11434).
	BaseURL string
	// Model de embeddings a usar (ej. "nomic-embed-text"). El harness (plan 044 F1b)
	// decide el modelo final midiendo pares duplicados/no-duplicados reales.
	Model string
	// Timeout de la request HTTP. Embeder un lote chico es rápido: default 60s.
	Timeout time.Duration
}

// EmbedProvider es la implementación Ollama de llm.Embedder. Pega a POST
// {baseURL}/api/embed con model + input batch (contrato Ollama 0.31.x).
type EmbedProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewEmbedder construye el cliente de embeddings Ollama a partir de su config.
func NewEmbedder(cfg EmbedConfig) *EmbedProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &EmbedProvider{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      cfg.Model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Name identifica al cliente (para logs).
func (p *EmbedProvider) Name() string { return "ollama-embed:" + p.model }

// embedRequest es el body de POST /api/embed. input es un lote de textos.
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponse es la respuesta: un vector por texto de entrada, mismo orden.
type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed vectoriza un lote de textos contra POST /api/embed. Devuelve un vector por
// texto en el mismo orden; un lote vacío no llama al backend. Valida que la cantidad
// de vectores devueltos coincida con la de textos.
func (p *EmbedProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := embedRequest{Model: p.model, Input: texts}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama embed request: %w", err)
	}

	url := p.baseURL + "/api/embed"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating ollama embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ollama embed response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama embed returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var er embedResponse
	if err := json.Unmarshal(body, &er); err != nil {
		return nil, fmt.Errorf("parsing ollama embed response: %w", err)
	}
	if len(er.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embed returned %d vectors for %d texts", len(er.Embeddings), len(texts))
	}
	return er.Embeddings, nil
}
