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
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
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
	// Temperature del muestreo. TODAS las llamadas del worker piden JSON
	// estructurado (extracción/clasificación/juicio binario), no prosa creativa:
	// el default 0 = greedy determinista hace la corrección REPRODUCIBLE (mismo
	// intento ⇒ mismo veredicto). Sin fijarla, Ollama usa 0.8 y el veredicto
	// parpadea entre corridas (medido en 045: criterios open_ended que oscilaban
	// correct/incorrect sin cambiar el input).
	Temperature float64
}

// Provider es la implementación Ollama de llm.LLMProvider.
type Provider struct {
	baseURL     string
	model       string
	temperature float64
	httpClient  *http.Client
}

// New construye el provider Ollama a partir de su config.
func New(cfg Config) *Provider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Provider{
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		model:       cfg.Model,
		temperature: cfg.Temperature,
		httpClient:  &http.Client{Timeout: timeout},
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
	// Options son las opciones de muestreo de Ollama (temperature, etc.). Se omite
	// si es nil para no alterar el comportamiento cuando no se configura nada.
	Options *generateOptions `json:"options,omitempty"`
}

// generateOptions son las opciones de muestreo de Ollama que el worker fija.
// Hoy solo temperature (determinismo); ampliable (top_p, seed) sin tocar callers.
type generateOptions struct {
	Temperature float64 `json:"temperature"`
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

// CheckCriterion pide el cumplimiento binario de un criterio (plan 042 F4b). Mismo
// camino que ReviewAnswer: el resultado es un ReviewResult (verdict/score/feedback).
func (p *Provider) CheckCriterion(ctx context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	prompt := llm.BuildCriterionCheckPrompt(req)
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
		return llm.ReviewResult{}, fmt.Errorf("respuesta de criterio no parseable: %w", err)
	}
	return result, nil
}

// ScoreRelevance puntúa la relevancia 0..1 de una candidata contra las ideas del job
// (plan 044 F2a, pasada 2 del reduce). Mismo camino que las demás llamadas: build prompt
// → generar → ExtractJSON → ParseRelevanceResult (valida forma y rango [0,1]). Una salida
// que no parsea o cae fuera de rango es error; el caller (RelevancePass) reintenta una vez
// y, si persiste, deja el score nil sin descartar la candidata (conservador). No está en
// el puerto llm.LLMProvider: la pasada la consume por una interfaz mínima propia (ISP).
func (p *Provider) ScoreRelevance(ctx context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error) {
	prompt := llm.BuildRelevancePrompt(req)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return llm.RelevanceResult{}, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return llm.RelevanceResult{}, err
	}
	return llm.ParseRelevanceResult(rawJSON)
}

// ExtractIdeas descompone la respuesta del alumno en ideas atómicas (plan 045 F4).
// Mismo camino que las demás llamadas: build prompt → generar → ExtractJSON → validar
// la forma {"ideas":[…]}. Una extracción que no parsea es fallo transitorio (el caller
// decide el fallback a la respuesta cruda).
func (p *Provider) ExtractIdeas(ctx context.Context, req llm.ExtractIdeasRequest) ([]string, error) {
	prompt := llm.BuildExtractIdeasPrompt(req)
	out, err := p.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return nil, err
	}
	return llm.ParseExtractedIdeas(rawJSON)
}

// DigestChunk ejecuta la llamada A ("leer") del pipeline material→evaluación (plan 043
// F3) en su forma "tarea partida" (digest_split.go): dos llamadas mínimas —A1 solo
// summary+topic, A2 solo ideas— ensambladas en el mismo DigestChunkResult del contrato.
// Medido 2026-07-19 con gemma4:e4b sobre las zonas duras del CONASET: la llamada única
// degeneraba el summary (66% válidos) y la partición lo rescata (97%) sin dañar el
// control, a ~+22% de latencia. El caller valida contra ChunkArtifactsV1, igual.
func (p *Provider) DigestChunk(ctx context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	// Override opcional de temperatura (jitter del reintento por calidad, plan 043);
	// nil = temperatura por instancia (determinista). Aplica a AMBAS mitades.
	temperature := p.temperature
	if in.Temperature != nil {
		temperature = *in.Temperature
	}

	// A1: summary encadenable + tema (la mitad que sostiene el pipeline, va primero).
	outS, err := p.generateWithTemperature(ctx, llm.BuildDigestSummaryPrompt(in), temperature)
	if err != nil {
		// Fallo de transporte/HTTP: es INFRA, sube SIN el sentinel de calidad.
		return nil, err
	}
	rawS, err := llm.ExtractJSON(outS)
	if err != nil {
		// El modelo respondió, pero su salida no trae un objeto JSON usable: CALIDAD.
		return nil, fmt.Errorf("%w: digest A1: %v", llm.ErrLLMQuality, err)
	}
	summaryPart, err := llm.ParseDigestSummaryPart(rawS)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", llm.ErrLLMQuality, err)
	}

	// A2: solo las ideas del trozo.
	outI, err := p.generateWithTemperature(ctx, llm.BuildDigestIdeasPrompt(in), temperature)
	if err != nil {
		return nil, err
	}
	rawI, err := llm.ExtractJSON(outI)
	if err != nil {
		return nil, fmt.Errorf("%w: digest A2: %v", llm.ErrLLMQuality, err)
	}
	ideasPart, err := llm.ParseDigestIdeasPart(rawI)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", llm.ErrLLMQuality, err)
	}

	return llm.CombineDigestParts(summaryPart, ideasPart), nil
}

// ProposeCandidates ejecuta la llamada B ("preguntar") del pipeline (plan 043 F3).
// Mismo camino: build prompt → generar → ExtractJSON → ParseCandidates. El caller valida
// cada candidata contra CandidatePayloadV1.
func (p *Provider) ProposeCandidates(ctx context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	prompt := llm.BuildProposeCandidatesPrompt(in)
	// Override opcional de temperatura (jitter del reintento por calidad de la fase B).
	temperature := p.temperature
	if in.Temperature != nil {
		temperature = *in.Temperature
	}
	out, err := p.generateWithTemperature(ctx, prompt, temperature)
	if err != nil {
		// Fallo de transporte/HTTP: INFRA, sube SIN el sentinel de calidad.
		return nil, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", llm.ErrLLMQuality, err)
	}
	candidates, err := llm.ParseCandidates(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", llm.ErrLLMQuality, err)
	}
	return candidates, nil
}

// generate ejecuta POST /api/generate con la temperatura por instancia del provider.
func (p *Provider) generate(ctx context.Context, prompt string) (string, error) {
	return p.generateWithTemperature(ctx, prompt, p.temperature)
}

// generateWithTemperature ejecuta POST /api/generate con una temperatura explícita y
// devuelve el texto crudo del modelo. La usa DigestChunk para aplicar el jitter del
// reintento por calidad sin cambiar el default determinista del resto de llamadas.
func (p *Provider) generateWithTemperature(ctx context.Context, prompt string, temperature float64) (string, error) {
	reqBody := generateRequest{
		Model:   p.model,
		Prompt:  prompt,
		Stream:  false,
		Format:  "json",
		Think:   false,
		Options: &generateOptions{Temperature: temperature},
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
