package shortanswer

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// mockProvider implementa llm.LLMProvider; solo JudgePairEquivalence hace algo. Las
// llamadas de par se resuelven consultando pairVerdicts por el par (Expected|Candidate)
// normalizado; ausente ⇒ incorrect. Cuenta las llamadas para verificar el conteo.
type mockProvider struct {
	pairVerdicts map[string]llm.Verdict
	pairCalls    int
	pairErr      error
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
func (m *mockProvider) JudgePairEquivalence(_ context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	m.pairCalls++
	if m.pairErr != nil {
		return llm.ReviewResult{}, m.pairErr
	}
	key := normalize(req.Expected) + "|" + normalize(req.Candidate)
	v := m.pairVerdicts[key]
	if v == "" {
		v = llm.VerdictIncorrect
	}
	score := 0.0
	if v == llm.VerdictCorrect {
		score = 1.0
	}
	return llm.ReviewResult{Verdict: v, Score: score}, nil
}
func (m *mockProvider) CheckCriterion(_ context.Context, _ llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, nil
}
func (m *mockProvider) Name() string { return "mock" }

func TestSplit(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"comas y conjuncion", "Ecuador, Venezuela y Colombia", []string{"ecuador", "venezuela", "colombia"}},
		{"solo espacios y conjuncion", "Ecuador Venezuela y Colombia", []string{"ecuador venezuela", "colombia"}},
		{"barra y punto y coma", "rojo | verde; azul", []string{"rojo", "verde", "azul"}},
		{"conjuncion e (conserva ñ)", "España e Italia", []string{"españa", "italia"}},
		{"tildes y mayusculas", "Panamá , PERÚ", []string{"panama", "peru"}},
		{"no corta palabras con y/e", "hoy y ley", []string{"hoy", "ley"}},
		{"vacios se descartan", "a,, b", []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Split(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Split(%q) = %v, quería %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatchItems(t *testing.T) {
	items := []string{"ecuador", "venezuela", "colombia"}

	t.Run("sin comas, match por palabra cubre los tres", func(t *testing.T) {
		frags := Split("Ecuador Venezuela y Colombia")
		res := matchItems(items, frags)
		for i, m := range res.itemMatched {
			if !m {
				t.Fatalf("ítem %d (%q) debería casar de forma determinista", i, items[i])
			}
		}
	})

	t.Run("un ítem con typo NO casa determinista y deja fragmento libre", func(t *testing.T) {
		frags := Split("ecuador, benezuela y colombia") // benezuela != venezuela
		res := matchItems(items, frags)
		if !res.itemMatched[0] || res.itemMatched[1] || !res.itemMatched[2] {
			t.Fatalf("esperaba ecuador✓ venezuela✗ colombia✓, hubo %v", res.itemMatched)
		}
		// El fragmento "benezuela" (índice 1) debe quedar SIN usar (candidato del par).
		if res.fragUsed[1] {
			t.Fatalf("el fragmento benezuela no debería estar usado: %v", res.fragUsed)
		}
	})

	t.Run("falta un ítem y no hay fragmento sobrante", func(t *testing.T) {
		frags := Split("ecuador y colombia")
		res := matchItems(items, frags)
		if !res.itemMatched[0] || res.itemMatched[1] || !res.itemMatched[2] {
			t.Fatalf("esperaba venezuela ausente, hubo %v", res.itemMatched)
		}
		for _, u := range res.fragUsed {
			if !u {
				t.Fatalf("ambos fragmentos deberían estar usados: %v", res.fragUsed)
			}
		}
	})
}

func grandColombia(student string) GradeInput {
	return GradeInput{
		QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
		StudentAnswer: student,
		Items:         []string{"ecuador", "venezuela", "colombia"},
		ItemsVerbatim: []string{"Ecuador", "Venezuela", "Colombia"},
	}
}

func TestGrade_TodoDeterminista_SinLlamadas(t *testing.T) {
	prov := &mockProvider{}
	res, err := Grade(context.Background(), prov, grandColombia("Ecuador Venezuela y Colombia"))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 0 {
		t.Fatalf("esperaba 0 llamadas de par (todo determinista), hubo %d", prov.pairCalls)
	}
}

func TestGrade_TypoRescatadoPorUnPar(t *testing.T) {
	prov := &mockProvider{pairVerdicts: map[string]llm.Verdict{
		"venezuela|benezuela": llm.VerdictCorrect,
	}}
	res, err := Grade(context.Background(), prov, grandColombia("ecuador, benezuela y colombia"))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0 tras rescate, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 1 {
		t.Fatalf("esperaba exactamente 1 llamada de par, hubo %d", prov.pairCalls)
	}
}

func TestGrade_TypoNoRescatado_Incorrect(t *testing.T) {
	prov := &mockProvider{} // el par devuelve incorrect por defecto
	res, err := Grade(context.Background(), prov, grandColombia("ecuador, benezuela y colombia"))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictIncorrect || res.Score != 0.0 {
		t.Fatalf("esperaba incorrect/0.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 1 {
		t.Fatalf("esperaba 1 llamada de par (benezuela candidato), hubo %d", prov.pairCalls)
	}
}

func TestGrade_FaltaItem_SinLlamada(t *testing.T) {
	prov := &mockProvider{}
	res, err := Grade(context.Background(), prov, grandColombia("ecuador y colombia"))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictIncorrect || res.Score != 0.0 {
		t.Fatalf("esperaba incorrect/0.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	// Sin fragmento sobrante para venezuela ⇒ NO se gasta llamada (no se adivina).
	if prov.pairCalls != 0 {
		t.Fatalf("esperaba 0 llamadas de par (sin candidato), hubo %d", prov.pairCalls)
	}
}

func TestGrade_ErrorProviderSePropaga(t *testing.T) {
	prov := &mockProvider{pairErr: context.DeadlineExceeded}
	_, err := Grade(context.Background(), prov, grandColombia("ecuador, benezuela y colombia"))
	if err == nil {
		t.Fatal("esperaba que el error del provider se propagara")
	}
}
