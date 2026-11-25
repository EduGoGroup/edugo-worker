package main

// material_probe.go — envoltorios del proveedor y del embedder que el modo material
// pone entre el riel real y el modelo (AR-IA.3). No cambian ninguna respuesta: solo
// cuentan llamadas, tiempos y errores, y guardan lo que el modelo propuso ANTES de
// que el procesador lo filtre por contrato.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// railModel es lo que el riel de material necesita del proveedor LOCAL: las dos
// llamadas de la fase 1 y los dos jueces de la fase 2. *ollama.Provider lo cumple.
type railModel interface {
	DigestChunk(ctx context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error)
	ProposeCandidates(ctx context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error)
	JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error)
	ScoreRelevance(ctx context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error)
	Name() string
}

// Clases de error de una llamada al modelo. Una salida que no se pudo interpretar es
// un dato del modelo; lo demás (red, timeout, 5xx) es del proveedor.
const (
	errKindInvalidOutput = "invalid_output"
	errKindProvider      = "provider_error"
)

// classifyModelError separa la salida inválida del modelo del fallo del proveedor.
func classifyModelError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, llm.ErrLLMQuality) {
		return errKindInvalidOutput
	}
	return errKindProvider
}

// callStats son las cuentas de un tipo de llamada, acumuladas en toda la corrida del
// caso (también en las reentregas).
type callStats struct {
	Calls          int   `json:"calls"`
	ElapsedMS      int64 `json:"elapsed_ms"`
	InvalidOutputs int   `json:"invalid_outputs"`
	ProviderErrors int   `json:"provider_errors"`
	// QualityRetries son las llamadas que el procesador repitió con otra temperatura
	// tras una salida inválida (su reintento de calidad, conducta de producción).
	QualityRetries int `json:"quality_retries,omitempty"`
}

func (c *callStats) add(elapsed time.Duration, err error, retry bool) {
	c.Calls++
	c.ElapsedMS += elapsed.Milliseconds()
	switch classifyModelError(err) {
	case errKindInvalidOutput:
		c.InvalidOutputs++
	case errKindProvider:
		c.ProviderErrors++
	}
	if retry {
		c.QualityRetries++
	}
}

// proposedCandidate es una candidata tal como la propuso el modelo, antes del filtro
// del procesador, con lo que diría de ella el validador del contrato.
type proposedCandidate struct {
	Payload json.RawMessage `json:"payload"`
	Type    string          `json:"question_type"`
	Valid   bool            `json:"valid"`
	Issues  []string        `json:"issues,omitempty"`
	// Deictic es la frase que cita el contexto del prompt («según el texto»), o vacío.
	Deictic string `json:"deictic,omitempty"`
}

// proposal es una llamada a «preguntar» y lo que devolvió.
type proposal struct {
	ChunkText  string              `json:"-"`
	Retry      bool                `json:"retry"`
	ErrorKind  string              `json:"error_kind,omitempty"`
	Error      string              `json:"error,omitempty"`
	GivenIdeas []string            `json:"given_ideas"`
	Candidates []proposedCandidate `json:"candidates"`
}

// probeModel envuelve el proveedor del riel.
type probeModel struct {
	inner railModel

	mu        sync.Mutex
	digest    callStats
	propose   callStats
	judge     callStats
	relevance callStats
	lastChunk string // el texto del último trozo leído: «preguntar» no lo recibe
	proposals []proposal
}

func newProbeModel(inner railModel) *probeModel { return &probeModel{inner: inner} }

func (p *probeModel) Name() string { return p.inner.Name() }

func (p *probeModel) DigestChunk(ctx context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	start := time.Now()
	res, err := p.inner.DigestChunk(ctx, in)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.digest.add(time.Since(start), err, in.Temperature != nil)
	p.lastChunk = in.ChunkText
	return res, err
}

func (p *probeModel) ProposeCandidates(ctx context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	start := time.Now()
	cands, err := p.inner.ProposeCandidates(ctx, in)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.propose.add(time.Since(start), err, in.Temperature != nil)
	prop := proposal{
		ChunkText:  p.lastChunk,
		Retry:      in.Temperature != nil,
		GivenIdeas: append(append([]string{}, in.Artifacts.MainIdeas...), in.Artifacts.SecondaryIdeas...),
		Candidates: []proposedCandidate{},
	}
	if err != nil {
		prop.ErrorKind = classifyModelError(err)
		prop.Error = err.Error()
	}
	for _, c := range cands {
		prop.Candidates = append(prop.Candidates, describeProposed(c))
	}
	p.proposals = append(p.proposals, prop)
	return cands, err
}

// describeProposed pasa una candidata propuesta por el MISMO validador que usa el
// procesador para filtrarla, y por el detector de citas al contexto.
func describeProposed(c materialpipeline.CandidatePayloadV1) proposedCandidate {
	out := proposedCandidate{Type: c.QuestionType, Deictic: materialpipeline.DetectDeicticReference(c.QuestionText)}
	raw, err := c.Marshal()
	if err != nil {
		out.Issues = []string{"no serializable: " + err.Error()}
		return out
	}
	out.Payload = raw
	if _, verr := materialpipeline.ValidateCandidatePayload(raw); verr != nil {
		var ve *materialpipeline.ValidationError
		if errors.As(verr, &ve) {
			for _, iss := range ve.Issues {
				out.Issues = append(out.Issues, iss.String())
			}
		} else {
			out.Issues = []string{verr.Error()}
		}
		return out
	}
	out.Valid = true
	return out
}

func (p *probeModel) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	start := time.Now()
	res, err := p.inner.JudgePairEquivalence(ctx, req)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.judge.add(time.Since(start), err, false)
	return res, err
}

func (p *probeModel) ScoreRelevance(ctx context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error) {
	start := time.Now()
	res, err := p.inner.ScoreRelevance(ctx, req)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relevance.add(time.Since(start), err, false)
	return res, err
}

// finalProposals devuelve, por texto de trozo, la ÚLTIMA llamada a «preguntar»: la
// que el procesador usó (si la primera salió inválida, la segunda es su reintento).
func (p *probeModel) finalProposals() map[string]proposal {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]proposal, len(p.proposals))
	for _, prop := range p.proposals {
		out[prop.ChunkText] = prop
	}
	return out
}

// embedStats son las cuentas del embedder.
type embedStats struct {
	Calls     int   `json:"calls"`
	Texts     int   `json:"texts"`
	ElapsedMS int64 `json:"elapsed_ms"`
	Errors    int   `json:"errors"`
}

// probeEmbedder envuelve el embedder y cuenta.
type probeEmbedder struct {
	inner llm.Embedder
	mu    sync.Mutex
	stats embedStats
}

func (e *probeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	start := time.Now()
	vecs, err := e.inner.Embed(ctx, texts)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats.Calls++
	e.stats.Texts += len(texts)
	e.stats.ElapsedMS += time.Since(start).Milliseconds()
	if err != nil {
		e.stats.Errors++
	}
	return vecs, err
}

// greyZoneEmbedder es un embedder FIJO: da a los textos, por turno, dos vectores
// cuyo coseno es 0,8, dentro de la zona gris de producción (0,50 a 0,93). Con él,
// toda pareja que el escalón de letras no decide llega al juez: mide al juez solo.
type greyZoneEmbedder struct{}

func (greyZoneEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		if i%2 == 0 {
			out[i] = []float32{1, 0}
		} else {
			out[i] = []float32{0.8, 0.6}
		}
	}
	return out, nil
}
