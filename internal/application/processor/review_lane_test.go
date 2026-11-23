package processor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// laneFakeProvider es un proveedor simulado para ReviewAnswerLane: cuenta cada
// llamada y devuelve lo que se le fija. Los métodos que el carril no usa quedan
// en la interfaz embebida (nil) y harían panic si se llamaran.
type laneFakeProvider struct {
	llm.LLMProvider

	reviewVerdict llm.Verdict
	reviewScore   float64
	reviewErr     error
	lastReview    llm.ReviewRequest

	criterionVerdict llm.Verdict

	reviewCalls, pairCalls, criterionCalls, extractCalls int
}

func (f *laneFakeProvider) Name() string { return "lane-fake" }

func (f *laneFakeProvider) ReviewAnswer(_ context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	f.reviewCalls++
	f.lastReview = req
	if f.reviewErr != nil {
		return llm.ReviewResult{}, f.reviewErr
	}
	return llm.ReviewResult{Verdict: f.reviewVerdict, Score: f.reviewScore, Feedback: "global"}, nil
}

func (f *laneFakeProvider) JudgePairEquivalence(_ context.Context, _ llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	f.pairCalls++
	return llm.ReviewResult{Verdict: llm.VerdictIncorrect}, nil
}

func (f *laneFakeProvider) CheckCriterion(_ context.Context, _ llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	f.criterionCalls++
	return llm.ReviewResult{Verdict: f.criterionVerdict}, nil
}

func (f *laneFakeProvider) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	f.extractCalls++
	return []string{"idea"}, nil
}

func laneAnswer(questionType, student string, prep string) m2m.PendingAnswer {
	ans := m2m.PendingAnswer{
		AnswerID:       "ans-1",
		QuestionType:   questionType,
		QuestionText:   "¿Pregunta?",
		ExpectedAnswer: "esperada",
		StudentAnswer:  student,
		Points:         4,
	}
	if prep != "" {
		ans.LLMPrep = json.RawMessage(prep)
	}
	return ans
}

const (
	laneListPrep = `{"version":1,"question_type":"short_answer","content_kind":"list",` +
		`"items":["ecuador","venezuela"],"items_verbatim":["Ecuador","Venezuela"]}`
	laneTermPrep = `{"version":1,"question_type":"short_answer","content_kind":"term",` +
		`"items":["lima"],"items_verbatim":["Lima"]}`
	laneCriteriaPrep = `{"version":1,"question_type":"open_ended","question_intent":"medir algo",` +
		`"main_ideas":["idea"],"criteria":["criterio uno","criterio dos"]}`
	laneIdeasPrep = `{"version":1,"question_type":"open_ended","question_intent":"medir algo",` +
		`"main_ideas":["idea principal"]}`
)

// Los cinco carriles que elige reviewOne, con un veredicto válido.
func TestReviewAnswerLane_ChoosesLane(t *testing.T) {
	t.Run("lista: match determinista, sin llamar al modelo", func(t *testing.T) {
		f := &laneFakeProvider{}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "Venezuela y Ecuador", laneListPrep), newTestLogger())
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if got.Result.Verdict != llm.VerdictCorrect || got.Points != 4 {
			t.Fatalf("se esperaba correct con 4 puntos, salió %s con %.2f", got.Result.Verdict, got.Points)
		}
		if f.reviewCalls+f.pairCalls+f.criterionCalls != 0 {
			t.Fatalf("la lista completa no llama al modelo: review=%d pares=%d criterios=%d", f.reviewCalls, f.pairCalls, f.criterionCalls)
		}
	})

	t.Run("otro content_kind: prompt global con la esperada enriquecida", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect, reviewScore: 1}
		if _, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "Lima", laneTermPrep), newTestLogger()); err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if f.reviewCalls != 1 || !strings.Contains(f.lastReview.ExpectedAnswer, "lima") {
			t.Fatalf("se esperaba una llamada global con la esperada enriquecida; llamadas=%d esperada=%q", f.reviewCalls, f.lastReview.ExpectedAnswer)
		}
	})

	t.Run("criterios: una llamada por criterio y agregación en Go", func(t *testing.T) {
		f := &laneFakeProvider{criterionVerdict: llm.VerdictCorrect}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep), newTestLogger())
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if f.criterionCalls != 2 || f.reviewCalls != 0 {
			t.Fatalf("se esperaban 2 criterios y 0 globales; criterios=%d globales=%d", f.criterionCalls, f.reviewCalls)
		}
		if got.Result.Verdict != llm.VerdictCorrect || got.Points != 4 {
			t.Fatalf("se esperaba correct con 4 puntos, salió %s con %.2f", got.Result.Verdict, got.Points)
		}
	})

	t.Run("prep sin criterios: prompt global con las ideas", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: llm.VerdictPartial, reviewScore: 0.5}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneIdeasPrep), newTestLogger())
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if f.reviewCalls != 1 || f.lastReview.Prep == nil || len(f.lastReview.Prep.MainIdeas) != 1 {
			t.Fatalf("se esperaba una llamada global con el prep; llamadas=%d prep=%v", f.reviewCalls, f.lastReview.Prep)
		}
		if got.Points != 2 {
			t.Fatalf("0,5 de 4 puntos son 2, salió %.2f", got.Points)
		}
	})

	t.Run("sin prep: prompt global tal cual", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: llm.VerdictIncorrect, reviewScore: 0}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "otra", ""), newTestLogger())
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if f.reviewCalls != 1 || f.lastReview.ExpectedAnswer != "esperada" || got.Points != 0 {
			t.Fatalf("se esperaba la llamada global sin enriquecer; llamadas=%d esperada=%q puntos=%.2f", f.reviewCalls, f.lastReview.ExpectedAnswer, got.Points)
		}
	})
}

// Un veredicto inválido es ErrInvalidVerdict y conserva el resultado crudo; un
// error del proveedor no lo es.
func TestReviewAnswerLane_Errors(t *testing.T) {
	t.Run("veredicto vacío", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: "", reviewScore: 0.3}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "x", ""), newTestLogger())
		if !errors.Is(err, ErrInvalidVerdict) {
			t.Fatalf("se esperaba ErrInvalidVerdict, salió %v", err)
		}
		if got.Result.Score != 0.3 || got.Points != 0 {
			t.Fatalf("se esperaba el resultado crudo sin puntos; score=%.2f puntos=%.2f", got.Result.Score, got.Points)
		}
	})

	t.Run("parcial en una corta", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: llm.VerdictPartial, reviewScore: 0.5}
		_, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "x", ""), newTestLogger())
		if !errors.Is(err, ErrInvalidVerdict) {
			t.Fatalf("la corta es binaria: se esperaba ErrInvalidVerdict, salió %v", err)
		}
	})

	t.Run("error del proveedor", func(t *testing.T) {
		boom := errors.New("ollama request failed: connection refused")
		f := &laneFakeProvider{reviewErr: boom}
		_, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "x", ""), newTestLogger())
		if !errors.Is(err, boom) || errors.Is(err, ErrInvalidVerdict) {
			t.Fatalf("se esperaba el error del proveedor tal cual, salió %v", err)
		}
	})
}
