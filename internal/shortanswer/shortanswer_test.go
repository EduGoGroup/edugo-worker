package shortanswer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// mockProvider implementa llm.LLMProvider; solo JudgePairEquivalence hace algo. Cuenta
// las llamadas de par (para verificar el modelo de costo del carril) y responde según
// la config: pairErr fuerza un error; alwaysCorrect devuelve siempre correct/1.0. Sin
// config, el par devuelve incorrect por defecto.
type mockProvider struct {
	alwaysCorrect bool
	pairCalls     int
	pairErr       error
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
	m.pairCalls++
	if m.pairErr != nil {
		return llm.ReviewResult{}, m.pairErr
	}
	if m.alwaysCorrect {
		return llm.ReviewResult{Verdict: llm.VerdictCorrect, Score: 1.0}, nil
	}
	return llm.ReviewResult{Verdict: llm.VerdictIncorrect, Score: 0.0}, nil
}
func (m *mockProvider) CheckCriterion(_ context.Context, _ llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, nil
}
func (m *mockProvider) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	return nil, nil
}
func (m *mockProvider) DigestChunk(_ context.Context, _ llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	return nil, nil
}

func (m *mockProvider) ProposeCandidates(_ context.Context, _ llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	return nil, nil
}

func (m *mockProvider) Name() string { return "mock" }

// TestGrade_Caso1_TodoFuzzy_SinLlamadas es el ASSERT ESTRELLA del plan 045 F3: el Caso 1
// del research (redes sociales con dos typos + relleno de cortesía) se resuelve por la
// cascada exacto→fuzzy, SIN una sola llamada al modelo. "whastapp"≈"whatsapp" es una
// transposición (OSA dist 1, sim 0.875) e "instalgram"≈"instagram" una inserción (dist
// 1, sim 0.9); ambos superan el umbral 0.85. "el famoso" sobra y Lenient lo ignora.
func TestGrade_Caso1_TodoFuzzy_SinLlamadas(t *testing.T) {
	prov := &mockProvider{}
	in := GradeInput{
		QuestionText:  "Nombra tres redes sociales",
		StudentAnswer: "whastapp instalgram y el famoso facebook",
		Items:         []string{"facebook", "instagram", "whatsapp"},
		ItemsVerbatim: []string{"Facebook", "Instagram", "WhatsApp"},
	}
	res, err := Grade(context.Background(), prov, in)
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 0 {
		t.Fatalf("esperaba 0 llamadas al provider (todo por fuzzy), hubo %d", prov.pairCalls)
	}
}

// TestGrade_TypoRescatadoPorFuzzy_SinLlamada documenta el cambio de comportamiento de la
// migración: antes "benezuela" (typo de "venezuela") escalaba a UNA llamada de par; con
// el escalón fuzzy nuevo (dist 1, sim 0.888 ≥ 0.85) se rescata determinista, sin LLM.
// Reemplaza al viejo TestGrade_TypoRescatadoPorUnPar (que esperaba 1 llamada).
func TestGrade_TypoRescatadoPorFuzzy_SinLlamada(t *testing.T) {
	prov := &mockProvider{}
	res, err := Grade(context.Background(), prov, grandColombia("ecuador, benezuela y colombia"))
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0 (fuzzy rescata el typo), hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 0 {
		t.Fatalf("esperaba 0 llamadas (fuzzy, no LLM), hubo %d", prov.pairCalls)
	}
}

// TestGrade_FaltanteReal_SinCandidato_Incorrect: falta un ítem y el alumno no dejó
// ningún fragmento sobrante que ofrecer ⇒ incorrect sin gastar una llamada (no adivina).
func TestGrade_FaltanteReal_SinCandidato_Incorrect(t *testing.T) {
	prov := &mockProvider{}
	in := GradeInput{
		QuestionText:  "Nombra dos países de Sudamérica",
		StudentAnswer: "brasil",
		Items:         []string{"brasil", "argentina"},
		ItemsVerbatim: []string{"Brasil", "Argentina"},
	}
	res, err := Grade(context.Background(), prov, in)
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictIncorrect || res.Score != 0.0 {
		t.Fatalf("esperaba incorrect/0.0, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 0 {
		t.Fatalf("esperaba 0 llamadas (sin candidato sobrante), hubo %d", prov.pairCalls)
	}
}

// TestGrade_Escalado_UnaLlamada_Cubre: un ítem que el fuzzy NO cubre pero hay un
// candidato sobrante ⇒ exactamente UNA llamada al provider para ese ítem; si el fake
// dice correct, el ítem queda cubierto y el veredicto es correct.
func TestGrade_Escalado_UnaLlamada_Cubre(t *testing.T) {
	prov := &mockProvider{alwaysCorrect: true}
	in := GradeInput{
		QuestionText:  "¿Cuál es el país de los aztecas?",
		StudentAnswer: "pais azteca",
		Items:         []string{"mexico"},
		ItemsVerbatim: []string{"México"},
	}
	res, err := Grade(context.Background(), prov, in)
	if err != nil {
		t.Fatalf("Grade error: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("esperaba correct/1.0 tras escalado, hubo %s/%.2f", res.Verdict, res.Score)
	}
	if prov.pairCalls != 1 {
		t.Fatalf("esperaba exactamente 1 llamada de par, hubo %d", prov.pairCalls)
	}
}

// TestGrade_ErrorProviderSePropaga: un error del provider en la fase de escalado se
// propaga (transitorio; el caller reintenta el intento).
func TestGrade_ErrorProviderSePropaga(t *testing.T) {
	prov := &mockProvider{pairErr: context.DeadlineExceeded}
	in := GradeInput{
		QuestionText:  "¿Cuál es el país de los aztecas?",
		StudentAnswer: "pais azteca",
		Items:         []string{"mexico"},
		ItemsVerbatim: []string{"México"},
	}
	_, err := Grade(context.Background(), prov, in)
	if err == nil {
		t.Fatal("esperaba que el error del provider se propagara")
	}
}

func grandColombia(student string) GradeInput {
	return GradeInput{
		QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
		StudentAnswer: student,
		Items:         []string{"ecuador", "venezuela", "colombia"},
		ItemsVerbatim: []string{"Ecuador", "Venezuela", "Colombia"},
	}
}

// TestGrade_TodoDeterminista_SinLlamadas: la respuesta calza exacta (separada solo por
// espacios y conjunción) ⇒ correct sin llamadas.
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
