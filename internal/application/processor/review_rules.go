package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// Las reglas del carril de corrección que no dependen del modelo (HU-INF.5): la
// respuesta en blanco, de dónde sale el número en los carriles globales y el tope del
// comentario. Las usa reviewAnswer, el único sitio por el que pasan orchestrate y
// ReviewAnswerLane.

// blankAnswerFeedback es el comentario fijo de una respuesta en blanco (decisión 4).
const blankAnswerFeedback = "Respuesta en blanco."

// maxFeedbackRunes es el tope del comentario que se propone a la docente (decisión 8).
// Se corta en Go: el esquema del proveedor de pago no admite longitud máxima.
const maxFeedbackRunes = 600

// reviewAnswer produce, para una respuesta, el resultado y los puntos que se
// proponen. Una respuesta en blanco se resuelve por regla, sin ninguna llamada al
// modelo. Si no, elige el carril (reviewOne), revalida el veredicto y escala los
// puntos: en los carriles globales el Score ya viene derivado del veredicto
// (reviewGlobal); en el de lista es 1 o 0 y en el de criterios conserva su fórmula.
func (p *AttemptReviewProcessor) reviewAnswer(ctx context.Context, provider llm.LLMProvider, ans m2m.PendingAnswer) (ReviewLaneResult, error) {
	if isBlankAnswer(ans.StudentAnswer, referenceAnswer(ans)) {
		return ReviewLaneResult{
			Result:      llm.ReviewResult{Verdict: llm.VerdictIncorrect, Score: 0, Feedback: blankAnswerFeedback},
			Points:      0,
			BlankByRule: true,
		}, nil
	}

	result, err := p.reviewOne(ctx, provider, ans)
	if err != nil {
		return ReviewLaneResult{Result: result}, err
	}
	result.Verdict = llm.Verdict(strings.ToLower(strings.TrimSpace(string(result.Verdict))))
	if err := validateVerdict(ans.QuestionType, result.Verdict); err != nil {
		return ReviewLaneResult{Result: result}, err
	}
	result.Feedback = capFeedback(result.Feedback)
	return ReviewLaneResult{Result: result, Points: scaledPoints(result.Score, ans.Points)}, nil
}

// reviewGlobal es el juicio global del modelo (los carriles sin lista ni criterios).
// La salida se revalida contra el esquema de su tipo de pregunta y una inválida se
// reintenta una vez con otra temperatura; si sigue inválida, el error envuelve
// ErrInvalidVerdict. El número no es el del modelo: sale del veredicto (P8), y el
// score del modelo solo queda en el crudo (Raw) para quien mide.
func (p *AttemptReviewProcessor) reviewGlobal(ctx context.Context, provider llm.LLMProvider, req llm.ReviewRequest) (llm.ReviewResult, error) {
	res, err := llm.CallVerdict(ctx, llm.ReviewVerdicts(req.QuestionType), func(ctx context.Context, temperature *float64) (llm.ReviewResult, error) {
		r := req
		r.Temperature = temperature
		return provider.ReviewAnswer(ctx, r)
	})
	if err != nil {
		if errors.Is(err, llm.ErrLLMQuality) {
			return res, fmt.Errorf("%w: %w", ErrInvalidVerdict, err)
		}
		return res, err
	}
	res.Score = verdictFraction(res.Verdict)
	return res, nil
}

// verdictFraction es la fracción del puntaje que vale un veredicto en los carriles
// globales: correcta, todo; parcial, la mitad; incorrecta, nada (P8).
func verdictFraction(v llm.Verdict) float64 {
	switch v {
	case llm.VerdictCorrect:
		return 1
	case llm.VerdictPartial:
		return 0.5
	default:
		return 0
	}
}

// isBlankAnswer dice si la respuesta del alumno está en blanco: quitados los
// espacios, no tiene ninguna letra ni ningún número («», «   », «.», «...», «-»,
// «?»). «no sé» no lo es. Si la referencia tampoco tiene letras ni números (una
// esperada que es un símbolo), la regla no se aplica (decisión 3).
func isBlankAnswer(student, reference string) bool {
	return !hasLetterOrDigit(student) && hasLetterOrDigit(reference)
}

// referenceAnswer es contra qué se mide si una respuesta está en blanco: la esperada
// o, en una larga sin esperada, su rúbrica.
func referenceAnswer(ans m2m.PendingAnswer) string {
	if strings.TrimSpace(ans.ExpectedAnswer) != "" {
		return ans.ExpectedAnswer
	}
	return ans.Rubric
}

// hasLetterOrDigit dice si s tiene alguna letra o algún número. Cuenta cualquier
// número de Unicode (unicode.IsNumber), no solo los dígitos decimales: «½», «²» o
// «Ⅳ» son una respuesta, no un blanco.
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

// capFeedback corta el comentario en maxFeedbackRunes caracteres.
func capFeedback(s string) string {
	runes := []rune(s)
	if len(runes) <= maxFeedbackRunes {
		return s
	}
	return string(runes[:maxFeedbackRunes])
}

// answerSpecWire es la forma de answer_spec que sirve learning (contrato v1, ADR
// 0049). El worker solo la lee para el prompt.
type answerSpecWire struct {
	Version      int `json:"version"`
	Alternatives []struct {
		Text string `json:"text"`
	} `json:"alternatives"`
	Rules map[string]bool `json:"rules"`
}

// parseAnswerSpec traduce el answer_spec de una respuesta pendiente a lo que lee el
// prompt de la corta. Ausente, vacío o que no parsea ⇒ nil (el prompt de siempre);
// uno que no parsea se loguea, como un llm_prep inválido.
func (p *AttemptReviewProcessor) parseAnswerSpec(ans m2m.PendingAnswer) *llm.AnswerSpec {
	if len(ans.AnswerSpec) == 0 || string(ans.AnswerSpec) == "null" {
		return nil
	}
	var wire answerSpecWire
	if err := json.Unmarshal(ans.AnswerSpec, &wire); err != nil {
		p.logger.Warn("answer_spec inválido en pending answer, se ignora",
			"answer_id", ans.AnswerID, "motivo", err.Error())
		return nil
	}
	spec := &llm.AnswerSpec{Rules: wire.Rules}
	for _, a := range wire.Alternatives {
		if t := strings.TrimSpace(a.Text); t != "" {
			spec.Alternatives = append(spec.Alternatives, t)
		}
	}
	if len(spec.Alternatives) == 0 && len(spec.Rules) == 0 {
		return nil
	}
	return spec
}
