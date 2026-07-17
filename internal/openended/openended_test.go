package openended

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// mockProvider resuelve CheckCriterion consultando met por texto de criterio; ausente
// ⇒ incorrect. Cuenta las llamadas para verificar «una por criterio». Para F4 también
// simula ExtractIdeas (ideas/extractErr configurables) y captura las ExtractedIdeas que
// recibió cada CheckCriterion.
type mockProvider struct {
	met       map[string]bool
	calls     int
	critError error

	// extracción de ideas (F4/D-045.9).
	ideas        []string
	extractErr   error
	extractCalls int
	// gotIdeas guarda, por cada CheckCriterion, las ExtractedIdeas que llegaron.
	gotIdeas [][]string
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
	m.gotIdeas = append(m.gotIdeas, req.ExtractedIdeas)
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
func (m *mockProvider) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	m.extractCalls++
	if m.extractErr != nil {
		return nil, m.extractErr
	}
	return m.ideas, nil
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

// TestGrade_ExtraccionNoVacia_SePasaACadaCriterio verifica el camino feliz de F4: 1
// llamada a ExtractIdeas + N a CheckCriterion, y que las ideas extraídas llegan a CADA
// comprobación de criterio (D-045.9).
func TestGrade_ExtraccionNoVacia_SePasaACadaCriterio(t *testing.T) {
	criteria := []string{"menciona la luz", "menciona la energía"}
	ideas := []string{"las plantas usan la luz del sol", "producen energía"}
	prov := &mockProvider{
		met:   map[string]bool{"menciona la luz": true, "menciona la energía": true},
		ideas: ideas,
	}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if prov.extractCalls != 1 {
		t.Fatalf("esperaba 1 llamada a ExtractIdeas, hubo %d", prov.extractCalls)
	}
	if prov.calls != 2 {
		t.Fatalf("esperaba 2 llamadas a CheckCriterion (una por criterio), hubo %d", prov.calls)
	}
	if len(prov.gotIdeas) != 2 {
		t.Fatalf("esperaba 2 capturas de ExtractedIdeas, hubo %d", len(prov.gotIdeas))
	}
	for i, got := range prov.gotIdeas {
		if len(got) != len(ideas) {
			t.Fatalf("criterio %d: ExtractedIdeas = %v, quería %v", i, got, ideas)
		}
		for j := range ideas {
			if got[j] != ideas[j] {
				t.Fatalf("criterio %d idea %d = %q, quería %q", i, j, got[j], ideas[j])
			}
		}
	}
	if res.Verdict != llm.VerdictCorrect {
		t.Fatalf("esperaba correct (2/2), hubo %s", res.Verdict)
	}
}

// TestGrade_ExtraccionVacia_CaeARaw verifica el fallback por lista vacía: CheckCriterion
// recibe ExtractedIdeas nil (comportamiento previo a F4), sin cambiar el veredicto.
func TestGrade_ExtraccionVacia_CaeARaw(t *testing.T) {
	criteria := []string{"c1", "c2"}
	prov := &mockProvider{
		met:   map[string]bool{"c1": true, "c2": true},
		ideas: nil, // extracción vacía
	}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if prov.extractCalls != 1 {
		t.Fatalf("esperaba 1 llamada a ExtractIdeas, hubo %d", prov.extractCalls)
	}
	if prov.calls != 2 {
		t.Fatalf("esperaba 2 llamadas a CheckCriterion, hubo %d", prov.calls)
	}
	for i, got := range prov.gotIdeas {
		if got != nil {
			t.Fatalf("criterio %d: esperaba ExtractedIdeas nil (fallback), hubo %v", i, got)
		}
	}
	if res.Verdict != llm.VerdictCorrect {
		t.Fatalf("esperaba correct (2/2), hubo %s", res.Verdict)
	}
}

// TestGrade_ExtraccionError_NoRompeYCaeARaw verifica que un error de ExtractIdeas NO se
// propaga como fallo del intento: se cae a la respuesta cruda (ExtractedIdeas nil) y la
// corrección por criterios sigue normal (D-045.9, fallback seguro).
func TestGrade_ExtraccionError_NoRompeYCaeARaw(t *testing.T) {
	criteria := []string{"c1", "c2"}
	prov := &mockProvider{
		met:        map[string]bool{"c1": true, "c2": true},
		extractErr: context.DeadlineExceeded, // la extracción falla
	}
	res, err := Grade(context.Background(), prov, fotosintesis(criteria))
	if err != nil {
		t.Fatalf("el error de extracción NO debe propagarse; Grade devolvió: %v", err)
	}
	if prov.extractCalls != 1 {
		t.Fatalf("esperaba 1 llamada a ExtractIdeas, hubo %d", prov.extractCalls)
	}
	if prov.calls != 2 {
		t.Fatalf("esperaba 2 llamadas a CheckCriterion (corrección siguió), hubo %d", prov.calls)
	}
	for i, got := range prov.gotIdeas {
		if got != nil {
			t.Fatalf("criterio %d: esperaba ExtractedIdeas nil tras error, hubo %v", i, got)
		}
	}
	if res.Verdict != llm.VerdictCorrect {
		t.Fatalf("esperaba correct (2/2), hubo %s", res.Verdict)
	}
}
