package reduce

import (
	"context"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

// --- fakes / helpers propios de la selección ---

// fakeIdeas es un jobIdeasResolver determinista: devuelve una lista fija (o error) y
// cuenta las llamadas. Lista vacía ejercita el fallback al agregado de source_ideas.
type fakeIdeas struct {
	ideas []string
	err   error
	calls int
}

func (f *fakeIdeas) GetJobIdeas(context.Context, string) ([]string, error) {
	f.calls++
	return f.ideas, f.err
}

func newSelPass(store candidateStore, ideas jobIdeasResolver) *SelectionPass {
	return NewSelectionPass(store, ideas, &nopLogger{})
}

// liveCand arma una candidata viva (status=candidate) con score persistido, reusando el
// candRecord de dedupe_test.go (payload válido contra el contrato import).
func liveCand(id string, seq int, qType, qText string, score float64, correct any, options, sourceIdeas []string) m2m.CandidateRecord {
	r := candRecord(id, seq, qType, qText, correct, options, sourceIdeas)
	r.Score = &score
	return r
}

// liveOpen es el atajo para una candidata open_ended (sin opciones ni correct_answer).
func liveOpen(id string, seq int, qText string, score float64, sourceIdeas []string) m2m.CandidateRecord {
	return liveCand(id, seq, "open_ended", qText, score, nil, nil, sourceIdeas)
}

// liveShort es el atajo para una candidata short_answer (correct_answer libre, sin opciones).
func liveShort(id string, seq int, qText string, score float64, sourceIdeas []string) m2m.CandidateRecord {
	return liveCand(id, seq, "short_answer", qText, score, "resp", nil, sourceIdeas)
}

// --- tests ---

// La cobertura de ideas manda sobre el score puro: una candidata de score alto que
// duplica una idea ya cubierta queda fuera para dar cupo a la única que cubre otra idea,
// aunque su score sea menor.
func TestSelection_CoverageBeatsPureScore(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("c1", 0, "cubre A alto", 0.90, []string{"idea A"}),
		liveOpen("c2", 1, "cubre A medio", 0.80, []string{"idea A"}),
		liveOpen("c3", 2, "cubre B bajo", 0.50, []string{"idea B"}),
	}}
	ideas := &fakeIdeas{ideas: []string{"idea A", "idea B"}}

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	st := statusByID(store)
	if st["c1"] != statusSelected || st["c3"] != statusSelected {
		t.Fatalf("debían entrar c1 (cubre A) y c3 (única que cubre B): %v", st)
	}
	if st["c2"] != statusCandidate {
		t.Fatalf("c2 (score alto pero idea ya cubierta) NO debía entrar: %v", st)
	}
	if rep.Selected != 2 || rep.IdeasCubiertas != 2 || len(rep.IdeasSinCubrir) != 0 {
		t.Fatalf("reporte inesperado: %+v", rep)
	}
}

// Mix de tipos en el relleno: cubierta la única idea con la de mayor score, el relleno
// prefiere el tipo menos representado (short_answer) sobre una open_ended de mayor score.
func TestSelection_TypeMixInFill(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("c4", 0, "open cubre A top", 0.95, []string{"idea A"}),
		liveOpen("c1", 1, "open alto", 0.90, []string{"idea A"}),
		liveOpen("c2", 2, "open medio", 0.85, []string{"idea A"}),
		liveShort("c3", 3, "short bajo", 0.70, []string{"idea A"}),
	}}
	ideas := &fakeIdeas{ideas: []string{"idea A"}}

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 3)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	st := statusByID(store)
	// Cobertura: c4 (mayor score que cubre A). Relleno: c3 (short, tipo menos representado)
	// gana a c2 pese a menor score; luego c1 (mayor score entre las open restantes).
	if st["c4"] != statusSelected || st["c3"] != statusSelected || st["c1"] != statusSelected {
		t.Fatalf("debían entrar c4, c3 (mix) y c1: %v", st)
	}
	if st["c2"] != statusCandidate {
		t.Fatalf("c2 (open, score alto) NO debía entrar: el mix metió a c3: %v", st)
	}
	if rep.PorTipo["open_ended"] != 2 || rep.PorTipo["short_answer"] != 1 {
		t.Fatalf("mix por tipo inesperado: %+v", rep.PorTipo)
	}
}

// Ideas sin cubrir: una idea que ninguna candidata cubre se reporta en IdeasSinCubrir
// (advertencia), no aborta la selección.
func TestSelection_UncoveredIdeasReported(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("c1", 0, "cubre A", 0.90, []string{"idea A"}),
		liveOpen("c2", 1, "cubre otra", 0.50, []string{"idea distinta"}),
	}}
	ideas := &fakeIdeas{ideas: []string{"idea A", "idea huerfana sin candidata"}}

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.IdeasCubiertas != 1 || len(rep.IdeasSinCubrir) != 1 {
		t.Fatalf("debía quedar 1 idea sin cubrir: %+v", rep)
	}
	if rep.IdeasSinCubrir[0] != "idea huerfana sin candidata" {
		t.Fatalf("idea sin cubrir inesperada: %q", rep.IdeasSinCubrir[0])
	}
}

// Idempotencia: si el job ya tiene ≥target seleccionadas, no se re-selecciona (sin updates)
// y el reporte lo refleja (AlreadySelected + PorTipo de lo ya hecho).
func TestSelection_Idempotent_AlreadySelected(t *testing.T) {
	sel1 := liveOpen("s1", 0, "ya sel open", 0.90, []string{"idea A"})
	sel1.Status = statusSelected
	sel2 := liveShort("s2", 1, "ya sel short", 0.80, []string{"idea B"})
	sel2.Status = statusSelected
	store := &fakeStore{records: []m2m.CandidateRecord{
		sel1, sel2,
		liveOpen("c3", 2, "candidata viva", 0.95, []string{"idea A"}),
	}}
	ideas := &fakeIdeas{ideas: []string{"idea A", "idea B"}}

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.AlreadySelected || rep.Selected != 2 {
		t.Fatalf("debía cortar por idempotencia: %+v", rep)
	}
	if len(store.updateBatches) != 0 {
		t.Fatalf("no debía emitir updates: %d lotes", len(store.updateBatches))
	}
	if rep.PorTipo["open_ended"] != 1 || rep.PorTipo["short_answer"] != 1 {
		t.Fatalf("PorTipo debía reflejar lo ya seleccionado: %+v", rep.PorTipo)
	}
	if ideas.calls != 0 {
		t.Fatalf("el corte idempotente no debía consultar las ideas: calls=%d", ideas.calls)
	}
}

// Empates deterministas: mismo score cubriendo la misma idea → gana menor chunk_sequence
// y, si empata la secuencia, menor id.
func TestSelection_DeterministicTies(t *testing.T) {
	// Empate de score → menor chunk_sequence gana (b: seq 2 sobre a: seq 5).
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("a", 5, "empate seq alto", 0.80, []string{"idea A"}),
		liveOpen("b", 2, "empate seq bajo", 0.80, []string{"idea A"}),
	}}
	if _, err := newSelPass(store, &fakeIdeas{ideas: []string{"idea A"}}).Run(context.Background(), "job-1", 1); err != nil {
		t.Fatalf("Run: %v", err)
	}
	st := statusByID(store)
	if st["b"] != statusSelected || st["a"] != statusCandidate {
		t.Fatalf("en empate de score debía ganar 'b' (menor seq): %v", st)
	}

	// Empate de score y de seq → menor id gana ('a' sobre 'z').
	store2 := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("z", 3, "empate id z", 0.80, []string{"idea A"}),
		liveOpen("a", 3, "empate id a", 0.80, []string{"idea A"}),
	}}
	if _, err := newSelPass(store2, &fakeIdeas{ideas: []string{"idea A"}}).Run(context.Background(), "job-1", 1); err != nil {
		t.Fatalf("Run: %v", err)
	}
	st2 := statusByID(store2)
	if st2["a"] != statusSelected || st2["z"] != statusCandidate {
		t.Fatalf("en empate de seq debía ganar 'a' (menor id): %v", st2)
	}
}

// Target mayor que las vivas: se seleccionan TODAS las candidatas vivas (cobertura + relleno
// vacían la lista) sin error.
func TestSelection_TargetExceedsLive(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("c1", 0, "cubre A", 0.90, []string{"idea A"}),
		liveOpen("c2", 1, "no cubre nada del set", 0.50, []string{"idea suelta"}),
	}}
	ideas := &fakeIdeas{ideas: []string{"idea A"}}

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 5)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Selected != 2 {
		t.Fatalf("target > vivas debía seleccionar las 2 vivas: %+v", rep)
	}
	st := statusByID(store)
	if st["c1"] != statusSelected || st["c2"] != statusSelected {
		t.Fatalf("ambas vivas debían quedar selected: %v", st)
	}
}

// Fallback de ideas: si GetJobIdeas viene vacío, la cobertura se calcula sobre el agregado
// de source_ideas de las candidatas (IdeasFromSource) y la selección igual funciona.
func TestSelection_IdeasFallbackToSource(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		liveOpen("c1", 0, "cubre A", 0.90, []string{"idea A"}),
		liveOpen("c2", 1, "cubre B", 0.80, []string{"idea B"}),
	}}
	ideas := &fakeIdeas{ideas: nil} // vacío → fallback

	rep, err := newSelPass(store, ideas).Run(context.Background(), "job-1", 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.IdeasFromSource {
		t.Fatalf("debía marcar IdeasFromSource al caer al agregado: %+v", rep)
	}
	if rep.Selected != 2 || rep.IdeasCubiertas != 2 {
		t.Fatalf("el fallback debía cubrir A y B: %+v", rep)
	}
}
