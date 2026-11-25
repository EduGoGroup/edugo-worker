package processor

import (
	"context"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// La medición de una revisión (HU-IA.2): cuántas llamadas al modelo costó una
// respuesta, cuánto tardó cada una y qué campos comunes llevan la línea del veredicto
// y la del fallo. reviewAnswer crea un meteredProvider por respuesta; los carriles
// (shortanswer, openended, reviewGlobal) lo reciben como un LLMProvider más.

// Las operaciones del modelo en una revisión: el valor del campo `op` de la línea
// por llamada.
const (
	llmOpReviewAnswer   = "review_answer"
	llmOpJudgePair      = "judge_pair"
	llmOpCheckCriterion = "check_criterion"
	llmOpExtractIdeas   = "extract_ideas"
)

// meteredProvider envuelve un LLMProvider para UNA respuesta: cuenta cada llamada
// hecha por los cuatro métodos de la revisión (también las que fallan y los
// reintentos) y deja una línea DEBUG por llamada. Los demás métodos pasan al
// proveedor envuelto sin contarse. Las llamadas de una respuesta no corren en
// paralelo, así que el contador no necesita candado.
type meteredProvider struct {
	llm.LLMProvider
	log      logger.Logger
	answerID string
	calls    int
}

func newMeteredProvider(inner llm.LLMProvider, log logger.Logger, answerID string) *meteredProvider {
	return &meteredProvider{LLMProvider: inner, log: log, answerID: answerID}
}

// observe cuenta una llamada y deja su línea. ok dice si el proveedor respondió sin
// error; que la salida cumpla su esquema lo decide después quien llama.
func (m *meteredProvider) observe(op string, start time.Time, err error) {
	m.calls++
	m.log.Debug("llamada al modelo en revisión",
		"answer_id", m.answerID, "op", op,
		"duration_ms", time.Since(start).Milliseconds(), "ok", err == nil)
}

func (m *meteredProvider) ReviewAnswer(ctx context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	start := time.Now()
	res, err := m.LLMProvider.ReviewAnswer(ctx, req)
	m.observe(llmOpReviewAnswer, start, err)
	return res, err
}

func (m *meteredProvider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	start := time.Now()
	res, err := m.LLMProvider.JudgePairEquivalence(ctx, req)
	m.observe(llmOpJudgePair, start, err)
	return res, err
}

func (m *meteredProvider) CheckCriterion(ctx context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	start := time.Now()
	res, err := m.LLMProvider.CheckCriterion(ctx, req)
	m.observe(llmOpCheckCriterion, start, err)
	return res, err
}

func (m *meteredProvider) ExtractIdeas(ctx context.Context, req llm.ExtractIdeasRequest) ([]string, error) {
	start := time.Now()
	ideas, err := m.LLMProvider.ExtractIdeas(ctx, req)
	m.observe(llmOpExtractIdeas, start, err)
	return ideas, err
}

// reviewLogFields arma los campos que comparten la línea del veredicto y la del
// fallo: carril, llamadas al modelo y duración. El proveedor solo se nombra si hubo
// alguna llamada: sin llamada, ningún modelo intervino.
func reviewLogFields(lane ReviewLaneResult, providerName string) []any {
	fields := []any{
		"lane", string(lane.Lane),
		"llm_calls", lane.LLMCalls,
		"duration_ms", lane.Duration.Milliseconds(),
	}
	if lane.LLMCalls > 0 {
		fields = append(fields, "provider", providerName)
	}
	return fields
}
