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

// ErrRepeatLimit marks Ollama's 500 "token repeat limit reached": the model got stuck
// repeating one token and Ollama aborted the prediction. The server is fine; it is the
// model's output that failed, so it wraps llm.ErrLLMQuality (HU-IA.9).
var ErrRepeatLimit = fmt.Errorf("%w: ollama cortó un bucle de repetición del modelo", llm.ErrLLMQuality)

// repeatLimitBody is the text Ollama 0.34.4 puts in the body of that 500.
const repeatLimitBody = "token repeat limit reached"

// materialRetrySeed is the fixed, non-zero seed of the material quality retries: at the
// retry temperature it makes the draw repeatable from one run to the next (HU-IA.9).
const materialRetrySeed = 7919

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

// generateRequest es el body de POST /api/generate. Format es "json" (fuerza a
// Ollama a emitir JSON válido) o, en las llamadas del carril de revisión, el JSON
// Schema de la salida de esa llamada (HU-INF.5): Ollama restringe la forma al
// generar. Medido con Ollama 0.34.4 (P9.1).
type generateRequest struct {
	Model  string          `json:"model"`
	Prompt string          `json:"prompt"`
	Stream bool            `json:"stream"`
	Format json.RawMessage `json:"format,omitempty"`
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

// generateOptions son las opciones de muestreo de Ollama que el worker fija: la
// temperatura siempre y, solo en los reintentos del material, la semilla (HU-IA.9).
type generateOptions struct {
	Temperature float64 `json:"temperature"`
	// Seed es nil salvo en un reintento de «leer» o «preguntar»: sin ella, el cuerpo
	// de la petición es el de siempre.
	Seed *int `json:"seed,omitempty"`
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

// jsonFormat es el modo JSON sin esquema de Ollama, el de las llamadas que no son del
// carril de revisión (generación, preparación, material, relevancia).
var jsonFormat = json.RawMessage(`"json"`)

// ReviewAnswer pide al modelo la corrección de una respuesta, con el esquema de su
// tipo de pregunta en format. El resultado lleva el crudo: lo revalida quien llama.
func (p *Provider) ReviewAnswer(ctx context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	schema := llm.VerdictSchema(llm.ReviewVerdicts(req.QuestionType)).JSON()
	out, err := p.generateFormat(ctx, llm.BuildReviewPrompt(req), p.temperatureFor(req.Temperature), schema)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	return llm.DecodeVerdictOutput(out, "corrección")
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

// JudgePairEquivalence pide la equivalencia binaria de un par (plan 042 F3c), con el
// esquema binario en format. Mismo camino que ReviewAnswer. El esquema le alcanza
// también al juez del dedupe, que usa esta llamada: solo le restringe la forma.
func (p *Provider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	schema := llm.VerdictSchema(llm.BinaryVerdicts).JSON()
	out, err := p.generateFormat(ctx, llm.BuildPairEquivalencePrompt(req), p.temperatureFor(req.Temperature), schema)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	return llm.DecodeVerdictOutput(out, "equivalencia")
}

// CheckCriterion pide el cumplimiento binario de un criterio (plan 042 F4b), con el
// esquema binario en format. Mismo camino que ReviewAnswer.
func (p *Provider) CheckCriterion(ctx context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	schema := llm.VerdictSchema(llm.BinaryVerdicts).JSON()
	out, err := p.generateFormat(ctx, llm.BuildCriterionCheckPrompt(req), p.temperatureFor(req.Temperature), schema)
	if err != nil {
		return llm.ReviewResult{}, err
	}
	return llm.DecodeVerdictOutput(out, "criterio")
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

// ExtractIdeas descompone la respuesta del alumno en ideas atómicas (plan 045 F4),
// con IdeasSchema en format. Build prompt → generar → ExtractJSON → revalidar la forma
// {"ideas":[…]} (ParseExtractedIdeas). Una extracción que no parsea es un error de
// calidad; el caller decide el fallback a la respuesta cruda y lo marca.
func (p *Provider) ExtractIdeas(ctx context.Context, req llm.ExtractIdeasRequest) ([]string, error) {
	out, err := p.generateFormat(ctx, llm.BuildExtractIdeasPrompt(req), p.temperature, llm.IdeasSchema.JSON())
	if err != nil {
		return nil, err
	}
	rawJSON, err := llm.ExtractJSON(out)
	if err != nil {
		return nil, fmt.Errorf("%w: respuesta de extracción de ideas no parseable: %v", llm.ErrLLMQuality, err)
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
	// nil = temperatura por instancia (determinista). Aplica a AMBAS mitades, y con él
	// viaja la semilla fija del reintento.
	options := p.materialOptions(in.Temperature)

	// A1: summary encadenable + tema (la mitad que sostiene el pipeline, va primero).
	outS, err := p.generateWith(ctx, llm.BuildDigestSummaryPrompt(in), options, jsonFormat)
	if err != nil {
		// Transporte o HTTP: INFRA, sube SIN el sentinel de calidad. La excepción es el
		// 500 del bucle de repetición (ErrRepeatLimit), que ya llega como calidad.
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
	outI, err := p.generateWith(ctx, llm.BuildDigestIdeasPrompt(in), options, jsonFormat)
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
	// Override opcional de temperatura (jitter del reintento por calidad de la fase B),
	// con la semilla fija del reintento.
	out, err := p.generateWith(ctx, prompt, p.materialOptions(in.Temperature), jsonFormat)
	if err != nil {
		// Transporte o HTTP: INFRA, sube SIN el sentinel de calidad. La excepción es el
		// 500 del bucle de repetición (ErrRepeatLimit), que ya llega como calidad.
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
	return p.generateFormat(ctx, prompt, p.temperature, jsonFormat)
}

// materialOptions son las opciones de una llamada del material («leer» o «preguntar»):
// sin override, la temperatura del proveedor y nada más; con el override del reintento
// de calidad, esa temperatura y la semilla fija.
func (p *Provider) materialOptions(override *float64) generateOptions {
	if override == nil {
		return generateOptions{Temperature: p.temperature}
	}
	seed := materialRetrySeed
	return generateOptions{Temperature: *override, Seed: &seed}
}

// temperatureFor devuelve la temperatura de una llamada: la de la petición si viene
// (el reintento de una salida inválida) o la del proveedor.
func (p *Provider) temperatureFor(override *float64) float64 {
	if override != nil {
		return *override
	}
	return p.temperature
}

// generateFormat ejecuta POST /api/generate con una temperatura y un format
// explícitos ("json" o un JSON Schema) y devuelve el texto crudo del modelo.
func (p *Provider) generateFormat(ctx context.Context, prompt string, temperature float64, format json.RawMessage) (string, error) {
	return p.generateWith(ctx, prompt, generateOptions{Temperature: temperature}, format)
}

// generateWith ejecuta POST /api/generate con las opciones y el format dados. Un estado
// no 2xx es un error de infraestructura, salvo el 500 con que Ollama corta un bucle de
// repetición del modelo: ese sale como ErrRepeatLimit (calidad) con el mensaje original.
func (p *Provider) generateWith(ctx context.Context, prompt string, options generateOptions, format json.RawMessage) (string, error) {
	reqBody := generateRequest{
		Model:   p.model,
		Prompt:  prompt,
		Stream:  false,
		Format:  format,
		Think:   false,
		Options: &options,
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
		text := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusInternalServerError && strings.Contains(text, repeatLimitBody) {
			return "", fmt.Errorf("%w: ollama returned status %d: %s", ErrRepeatLimit, resp.StatusCode, text)
		}
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, text)
	}

	var gr generateResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("parsing ollama response: %w", err)
	}
	return gr.Response, nil
}
