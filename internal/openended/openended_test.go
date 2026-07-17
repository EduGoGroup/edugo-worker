package openended

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// mockProvider resuelve CheckCriterion consultando met por texto de criterio; ausente
// ⇒ incorrect. Cuenta las llamadas para verificar «una por criterio».
type mockProvider struct {
	met       map[string]bool
	calls     int
	critError error
}

func (m *mockProvider) GenerateAssessment(_ context.Context, _ llm.MaterialInput, _ llm.GenerationParams) (json.RawMessage, error) {
	return nil, nil
}
func (m *mockProvider) ReviewAnswer(_ context.Context, _ llm.ReviewRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, nil
}
func (m *mockProvider) PrepareQuestion(_ context.Context, _ llm.PrepRequest) (json.RawMessage, error) {
	return nil, nil
}
func (m *mockProvider) JudgePairEquivalence(_ context.Context, _ llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, nil
}
func (m *mockProvider) CheckCriterion(_ context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	m.calls++
	if m.critError != nil {
		return llm.ReviewResult{}, m.critError
	}
	v := llm.VerdictIncorrect
	if m.met[req.Criterion] {
		v = llm.VerdictCorrect
	}
	score := 0.0
	if v == llm.VerdictCorrect {
		score = 1.0
	}
	return llm.ReviewResult{Verdict: v, Score: score}, nil
}
func (m *mockProvider) Name() string { return "mock" }

func TestAggregate(t *testing.T) {
	cases := []struct {
		name        string
		met, total  int
		wantVerdict llm.Verdict
		wantScore   float64
	}{
		{"ninguno", 0, 3, llm.VerdictIncorrect, 0.0},
		{"todos", 3, 3, llm.VerdictCorrect, 1.0},
		{"2 de 3", 2, 3, llm.VerdictPartial, 0.3 + 0.4*(2.0/3.0)},
		{"1 de 3", 1, 3, llm.VerdictPartial, 0.3 + 0.4*(1.0/3.0)},
		{"1 de 2", 1, 2, llm.VerdictPartial, 0.5},
		{"sin criterios (defensivo)", 0, 0, llm.VerdictIncorrect, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := aggregate(tc.met, tc.total, nil)
			if res.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %s, quería %s", res.Verdict, tc.wantVerdict)
			}
			if math.Abs(res.Score-tc.wantScore) > 1e-9 {
				t.Fatalf("score = %.4f, quería %.4f", res.Score, tc.wantScore)
			}
			// El score de partial SIEMPRE cae dentro del ancla 0.3–0.7 del prompt actual.
			if res.Verdict == llm.VerdictPartial && (res.Score < 0.3 || res.Score > 0.7) {
				t.Fatalf("partial score %.4f fuera de [0.3,0.7]", res.Score)
			}
		})
	}
}

func fotosintesis(criteria []string) GradeInput {
	return GradeInput{
		QuestionText:  "Explica el proceso de la fotosíntesis.",
		StudentAnswer: "las plantas usan la luz del sol para producir energía",
		Criteria:      criteria,
		Language:      "es",
	}
}

func TestGrade_UnaLlamadaPorCriterio_Partial(t *testing.T) {
	criteria := []string{"menciona los cloroplastos", "menciona la luz", "menciona el oxígeno"}
	prov := &mockProvider{met: map[string]bool{
		"menciona la luz":     true,
		"menciona el oxígeno": true,
		// cloroplastos NO cumplido ⇒ 2 de 3.
	}}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if prov.calls != 3 {
		t.Fatalf("esperaba 3 llamadas (una por criterio), hubo %d", prov.calls)
	}
	if res.Verdict != llm.VerdictPartial {
		t.Fatalf("esperaba partial, hubo %s", res.Verdict)
	}
	want := 0.3 + 0.4*(2.0/3.0)
	if math.Abs(res.Score-want) > 1e-9 {
		t.Fatalf("score = %.4f, quería %.4f", res.Score, want)
	}
}

func TestGrade_TodosCumplidos_Correct(t *testing.T) {
	criteria := []string{"c1", "c2"}
	prov := &mockProvider{met: map[string]bool{"c1": true, "c2": true}}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.calls != 2 {
		t.Fatalf("esperaba 2 llamadas, hubo %d", prov.calls)
	}
}

func TestGrade_NingunoCumplido_Incorrect(t *testing.T) {
	criteria := []string{"c1", "c2"}
	prov := &mockProvider{} // met vacío ⇒ todos incorrect
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictIncorrect || res.Score != 0.0 {
		t.Fatalf("esperaba incorrect/0.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
}

func TestGrade_CriteriosVaciosSeIgnoran(t *testing.T) {
	// Un criterio en blanco no gasta llamada ni cuenta en el total.
	criteria := []string{"c1", "  ", "c2"}
	prov := &mockProvider{met: map[string]bool{"c1": true, "c2": true}}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if prov.calls != 2 {
		t.Fatalf("esperaba 2 llamadas (el vacío se ignora), hubo %d", prov.calls)
	}
	if res.Verdict != llm.VerdictCorrect {
		t.Fatalf("esperaba correct (2/2), hubo %s", res.Verdict)
	}
}

func TestGrade_ErrorProviderSePropaga(t *testing.T) {
	prov := &mockProvider{critError: context.DeadlineExceeded}
	_, err := Grade(context.Background(), prov, fotosintesis([]string{"c1"}))
	if err == nil {
		t.Fatal("esperaba que el error del provider se propagara")
	}
}
