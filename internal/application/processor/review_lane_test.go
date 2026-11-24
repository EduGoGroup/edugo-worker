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
		// HU-INF.5: la salida inválida se reintenta una vez antes de ser un error.
		if f.reviewCalls != 2 {
			t.Fatalf("se esperaban 2 llamadas (un reintento), hubo %d", f.reviewCalls)
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

// seqReviewProvider fija la salida del juicio global llamada a llamada (la última se
// repite) y guarda la temperatura de cada una (HU-INF.5).
type seqReviewProvider struct {
	laneFakeProvider
	outs  []llm.ReviewResult
	temps []*float64
}

func (f *seqReviewProvider) ReviewAnswer(_ context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	f.reviewCalls++
	f.lastReview = req
	f.temps = append(f.temps, req.Temperature)
	return f.outs[min(f.reviewCalls, len(f.outs))-1], nil
}

// HU-INF.5: salida vacía y salida fuera de esquema son un error tras un reintento,
// sin propuesta; si el reintento vale, sigue.
func TestReviewAnswerLane_InvalidOutput(t *testing.T) {
	cases := []struct {
		name string
		out  llm.ReviewResult
	}{
		{"salida vacía", llm.ReviewResult{}},
		{"fuera de esquema: falta score", llm.ReviewResult{Verdict: llm.VerdictCorrect, Raw: json.RawMessage(`{"verdict":"correct","feedback":"x"}`)}},
		{"fuera de esquema: score como cadena", llm.ReviewResult{Verdict: llm.VerdictCorrect, Raw: json.RawMessage(`{"verdict":"correct","score":"1","feedback":"x"}`)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &seqReviewProvider{outs: []llm.ReviewResult{c.out}}
			got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "x", ""), newTestLogger())
			if !errors.Is(err, ErrInvalidVerdict) || !errors.Is(err, llm.ErrInvalidOutput) {
				t.Fatalf("se esperaba ErrInvalidVerdict y ErrInvalidOutput, salió %v", err)
			}
			if got.Points != 0 || f.reviewCalls != 2 {
				t.Fatalf("sin propuesta y con un reintento: puntos=%.2f llamadas=%d", got.Points, f.reviewCalls)
			}
			if f.temps[0] != nil || f.temps[1] == nil || *f.temps[1] != llm.InvalidOutputRetryTemperature {
				t.Fatalf("la primera a la temperatura del proveedor y el reintento a %.1f", llm.InvalidOutputRetryTemperature)
			}
		})
	}

	t.Run("el reintento vale", func(t *testing.T) {
		f := &seqReviewProvider{outs: []llm.ReviewResult{{Verdict: "maybe"}, {Verdict: llm.VerdictCorrect, Score: 0.2}}}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "x", ""), newTestLogger())
		if err != nil || got.Points != 4 {
			t.Fatalf("la segunda salida decide: err=%v puntos=%.2f", err, got.Points)
		}
	})
}

// HU-INF.5 (P8): en los carriles globales el número sale del veredicto; el score del
// modelo se ignora. Un veredicto en mayúsculas vale.
func TestReviewAnswerLane_NumberFromVerdict(t *testing.T) {
	cases := []struct {
		name         string
		questionType string
		verdict      llm.Verdict
		score        float64
		want         float64
	}{
		{"correcta con 0,3: todos los puntos", llm.QuestionTypeShortAnswer, llm.VerdictCorrect, 0.3, 4},
		{"incorrecta con 0,9: cero", llm.QuestionTypeShortAnswer, llm.VerdictIncorrect, 0.9, 0},
		{"parcial en una larga global: la mitad", llm.QuestionTypeOpenEnded, llm.VerdictPartial, 0.9, 2},
		{"correcta con 0,1 en una larga: todos", llm.QuestionTypeOpenEnded, llm.VerdictCorrect, 0.1, 4},
		{"veredicto en mayúsculas", llm.QuestionTypeShortAnswer, "CORRECT", 0, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &laneFakeProvider{reviewVerdict: c.verdict, reviewScore: c.score}
			got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(c.questionType, "respuesta", ""), newTestLogger())
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got.Points != c.want || f.reviewCalls != 1 {
				t.Fatalf("se esperaban %.2f puntos en una llamada, salieron %.2f en %d", c.want, got.Points, f.reviewCalls)
			}
		})
	}

	t.Run("los criterios conservan su fórmula", func(t *testing.T) {
		f := &laneFakeProvider{}
		got, err := ReviewAnswerLane(context.Background(), &halfCriteria{laneFakeProvider: f}, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep), newTestLogger())
		if err != nil {
			t.Fatal(err)
		}
		// 1 de 2 criterios: 0,3 + 0,4·0,5 = 0,5 → 2 de 4.
		if got.Result.Verdict != llm.VerdictPartial || got.Points != 2 {
			t.Fatalf("se esperaba parcial con 2 puntos, salió %s con %.2f", got.Result.Verdict, got.Points)
		}
	})
}

// halfCriteria cumple el primer criterio y no el segundo.
type halfCriteria struct{ *laneFakeProvider }

func (h *halfCriteria) CheckCriterion(_ context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	h.criterionCalls++
	if req.Criterion == "criterio uno" {
		return llm.ReviewResult{Verdict: llm.VerdictCorrect, Score: 1}, nil
	}
	return llm.ReviewResult{Verdict: llm.VerdictIncorrect}, nil
}

// HU-INF.5 (decisiones 3 y 4): una respuesta en blanco no llama al proveedor, en
// ningún carril: incorrecta, cero puntos y el comentario fijo.
func TestReviewAnswerLane_BlankAnswer_NoProviderCalls(t *testing.T) {
	lanes := []struct {
		name, questionType, prep string
	}{
		{"corta global", llm.QuestionTypeShortAnswer, ""},
		{"corta con lista", llm.QuestionTypeShortAnswer, laneListPrep},
		{"corta con otro content_kind", llm.QuestionTypeShortAnswer, laneTermPrep},
		{"larga con criterios", llm.QuestionTypeOpenEnded, laneCriteriaPrep},
		{"larga con ideas", llm.QuestionTypeOpenEnded, laneIdeasPrep},
		{"larga global", llm.QuestionTypeOpenEnded, ""},
	}
	for _, lane := range lanes {
		for _, blank := range []string{"", "   ", "...", " - ", "?"} {
			t.Run(lane.name+" «"+blank+"»", func(t *testing.T) {
				f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect, reviewScore: 1, criterionVerdict: llm.VerdictCorrect}
				got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(lane.questionType, blank, lane.prep), newTestLogger())
				if err != nil {
					t.Fatalf("error inesperado: %v", err)
				}
				if calls := f.reviewCalls + f.pairCalls + f.criterionCalls + f.extractCalls; calls != 0 {
					t.Fatalf("una respuesta en blanco no llama al modelo, hubo %d llamadas", calls)
				}
				if got.Result.Verdict != llm.VerdictIncorrect || got.Points != 0 || !got.BlankByRule || got.Result.Feedback != "Respuesta en blanco." {
					t.Fatalf("se esperaba incorrecta por regla con 0 puntos, salió %+v", got)
				}
			})
		}
	}

	t.Run("«no sé» no está en blanco", func(t *testing.T) {
		f := &laneFakeProvider{reviewVerdict: llm.VerdictIncorrect}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeShortAnswer, "no sé", ""), newTestLogger())
		if err != nil || got.BlankByRule || f.reviewCalls != 1 {
			t.Fatalf("«no sé» va al modelo: err=%v regla=%v llamadas=%d", err, got.BlankByRule, f.reviewCalls)
		}
	})
}

// HU-INF.5 (P19): el answer_spec llega al prompt global de la corta, no al de la larga.
func TestReviewAnswerLane_AnswerSpecReachesShortPrompt(t *testing.T) {
	spec := json.RawMessage(`{"version":1,"alternatives":[{"text":"Lima"},{"text":"  "}],"rules":{"accents":false}}`)

	short := laneAnswer(llm.QuestionTypeShortAnswer, "lima", "")
	short.AnswerSpec = spec
	f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect}
	if _, err := ReviewAnswerLane(context.Background(), f, short, newTestLogger()); err != nil {
		t.Fatal(err)
	}
	got := f.lastReview.AnswerSpec
	if got == nil || len(got.Alternatives) != 1 || got.Alternatives[0] != "Lima" || got.Rules["accents"] {
		t.Fatalf("se esperaba el spec con una alternativa y accents=false, salió %+v", got)
	}

	open := laneAnswer(llm.QuestionTypeOpenEnded, "larga", "")
	open.AnswerSpec = spec
	f = &laneFakeProvider{reviewVerdict: llm.VerdictCorrect}
	if _, err := ReviewAnswerLane(context.Background(), f, open, newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if f.lastReview.AnswerSpec != nil {
		t.Fatalf("la larga no lleva answer_spec, salió %+v", f.lastReview.AnswerSpec)
	}

	broken := laneAnswer(llm.QuestionTypeShortAnswer, "lima", "")
	broken.AnswerSpec = json.RawMessage(`{"alternatives":"no"}`)
	f = &laneFakeProvider{reviewVerdict: llm.VerdictCorrect}
	if _, err := ReviewAnswerLane(context.Background(), f, broken, newTestLogger()); err != nil {
		t.Fatal(err)
	}
	if f.lastReview.AnswerSpec != nil {
		t.Fatal("un answer_spec que no parsea se ignora")
	}
}
