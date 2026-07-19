package reduce

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// --- fakes ---

type nopLogger struct{}

func (l *nopLogger) Debug(string, ...any)      {}
func (l *nopLogger) Info(string, ...any)       {}
func (l *nopLogger) Warn(string, ...any)       {}
func (l *nopLogger) Error(string, ...any)      {}
func (l *nopLogger) Fatal(string, ...any)      {}
func (l *nopLogger) Sync() error               { return nil }
func (l *nopLogger) With(...any) logger.Logger { return l }

// fakeStore es un candidateStore en memoria: aplica los updates a sus registros para
// que un segundo Run vea los cambios (idempotencia) y guarda los lotes recibidos.
type fakeStore struct {
	records       []m2m.CandidateRecord
	updateBatches [][]m2m.CandidateUpdate
}

func (s *fakeStore) ListCandidates(context.Context, string) ([]m2m.CandidateRecord, error) {
	out := make([]m2m.CandidateRecord, len(s.records))
	copy(out, s.records)
	return out, nil
}

func (s *fakeStore) UpdateCandidates(_ context.Context, updates []m2m.CandidateUpdate) (int, error) {
	s.updateBatches = append(s.updateBatches, updates)
	for _, u := range updates {
		for i := range s.records {
			if s.records[i].ID != u.ID {
				continue
			}
			if u.Status != nil {
				s.records[i].Status = *u.Status
			}
			if u.DedupeGroup != nil {
				s.records[i].DedupeGroup = u.DedupeGroup
			}
			if u.Score != nil {
				s.records[i].Score = u.Score
			}
			if len(u.Embedding) > 0 {
				s.records[i].Embedding = u.Embedding
			}
		}
	}
	return len(updates), nil
}

// fakeEmbedder devuelve un vector por texto desde un mapa; texto sin vector es un error
// (obliga a que el test declare cada question_text). Cuenta las llamadas.
type fakeEmbedder struct {
	vecs  map[string][]float32
	calls int
}

func (e *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	e.calls++
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, ok := e.vecs[t]
		if !ok {
			return nil, fmt.Errorf("fakeEmbedder: sin vector para %q", t)
		}
		out[i] = v
	}
	return out, nil
}

// fakeJudge decide equivalencia por un predicado sobre (expected, candidate). Cuenta
// llamadas para verificar que el LLM solo se toca en la zona gris.
type fakeJudge struct {
	equal func(expected, candidate string) bool
	calls int
}

func (j *fakeJudge) JudgePairEquivalence(_ context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	j.calls++
	if j.equal(req.Expected, req.Candidate) {
		return llm.ReviewResult{Verdict: llm.VerdictCorrect, Score: 1}, nil
	}
	return llm.ReviewResult{Verdict: llm.VerdictIncorrect, Score: 0}, nil
}

// --- helpers ---

func candRecord(id string, seq int, qType, qText string, correct any, options, sourceIdeas []string) m2m.CandidateRecord {
	p := materialpipeline.CandidatePayloadV1{
		Version:      1,
		QuestionType: qType,
		QuestionText: qText,
		Options:      options,
		SourceIdeas:  sourceIdeas,
	}
	if correct != nil {
		raw, err := json.Marshal(correct)
		if err != nil {
			panic(err)
		}
		p.CorrectAnswer = raw
	}
	payload, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return m2m.CandidateRecord{
		ID:            id,
		ChunkID:       "chunk-" + id,
		ChunkSequence: seq,
		Payload:       payload,
		Status:        statusCandidate,
	}
}

// statusByID indexa el estado final de cada candidata tras aplicar los updates al store.
func statusByID(s *fakeStore) map[string]string {
	m := make(map[string]string, len(s.records))
	for _, r := range s.records {
		m[r.ID] = r.Status
	}
	return m
}

func groupByID(s *fakeStore) map[string]*string {
	m := make(map[string]*string, len(s.records))
	for _, r := range s.records {
		m[r.ID] = r.DedupeGroup
	}
	return m
}

func newPass(store candidateStore, emb llm.Embedder, judge pairJudge) *DedupePass {
	return NewDedupePass(store, emb, judge, Config{}, &nopLogger{})
}

func neverEqual(string, string) bool { return false }

// --- tests ---

// Duplicado exacto: mismo texto y misma respuesta ⇒ se resuelve en el escalón de letras,
// sin tocar el LLM; una cae a dropped_dup y comparten dedupe_group.
func TestDedupe_ExactDuplicate_TextTier(t *testing.T) {
	q := "cual es la capital de francia"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "short_answer", q, "paris", nil, []string{"i1"}),
		candRecord("b", 1, "short_answer", q, "paris", nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{q: {1, 0, 0}}}
	judge := &fakeJudge{equal: neverEqual}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("el LLM no debía intervenir en un match textual, calls=%d", judge.calls)
	}
	if rep.Clusters != 1 || rep.DroppedDup != 1 || rep.PairsText != 1 {
		t.Fatalf("reporte inesperado: %+v", rep)
	}
	st := statusByID(store)
	if st["a"] != statusCandidate || st["b"] != statusDroppedDup {
		t.Fatalf("estados inesperados: %v", st)
	}
	g := groupByID(store)
	if g["a"] == nil || g["b"] == nil || *g["a"] != *g["b"] {
		t.Fatalf("dedupe_group no compartido: %v / %v", g["a"], g["b"])
	}
}

// Parafraseo: textos distintos que NO empatan por letras pero sí por coseno alto ⇒ dup
// resuelto en el escalón de embeddings, sin LLM.
func TestDedupe_Paraphrase_EmbedTier(t *testing.T) {
	q1 := "que paises formaron la gran colombia"
	q2 := "quienes integraban la gran colombia"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "open_ended", q1, nil, nil, []string{"i1", "i2"}),
		candRecord("b", 2, "open_ended", q2, nil, nil, []string{"i1"}),
	}}
	// Vectores casi paralelos → coseno ≈ 0.9997 ≥ 0.93.
	emb := &fakeEmbedder{vecs: map[string][]float32{
		q1: {1, 0.02, 0},
		q2: {1, 0, 0.02},
	}}
	judge := &fakeJudge{equal: neverEqual}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("coseno alto no debía llegar al LLM, calls=%d", judge.calls)
	}
	if rep.PairsEmbed != 1 || rep.DroppedDup != 1 || rep.EmbeddingsComputed != 2 {
		t.Fatalf("reporte inesperado: %+v", rep)
	}
	// Representante = más source_ideas (a tiene 2, b tiene 1).
	st := statusByID(store)
	if st["a"] != statusCandidate || st["b"] != statusDroppedDup {
		t.Fatalf("la representante debía ser 'a' (más source_ideas): %v", st)
	}
}

// Zona gris: coseno intermedio ⇒ el LLM decide. Si dice equivalente, se agrupa.
func TestDedupe_GrayZone_LLMDecides(t *testing.T) {
	q1 := "explica la fotosintesis"
	q2 := "describe como las plantas producen energia"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "open_ended", q1, nil, nil, []string{"i1"}),
		candRecord("b", 1, "open_ended", q2, nil, nil, []string{"i1"}),
	}}
	// Coseno ≈ 0.707 → dentro de [0.50, 0.93): zona gris.
	emb := &fakeEmbedder{vecs: map[string][]float32{
		q1: {1, 0, 0},
		q2: {1, 1, 0},
	}}
	judge := &fakeJudge{equal: func(_, _ string) bool { return true }}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 1 || rep.PairsLLM != 1 || rep.LLMCalls != 1 {
		t.Fatalf("la zona gris debía gastar exactamente 1 llamada LLM: calls=%d rep=%+v", judge.calls, rep)
	}
	if rep.DroppedDup != 1 || rep.Clusters != 1 {
		t.Fatalf("el LLM equivalente debía agrupar: %+v", rep)
	}
}

// Zona gris pero el LLM dice que NO ⇒ no se agrupan.
func TestDedupe_GrayZone_LLMSaysNo(t *testing.T) {
	q1 := "explica la fotosintesis"
	q2 := "describe como las plantas producen energia"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "open_ended", q1, nil, nil, []string{"i1"}),
		candRecord("b", 1, "open_ended", q2, nil, nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{
		q1: {1, 0, 0},
		q2: {1, 1, 0},
	}}
	judge := &fakeJudge{equal: neverEqual}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.DroppedDup != 0 || rep.Clusters != 0 {
		t.Fatalf("el LLM no-equivalente NO debía agrupar: %+v", rep)
	}
	if len(store.updateBatches) == 0 {
		t.Fatal("debía persistir al menos el lote de embeddings")
	}
}

// Mismo texto pero respuesta claramente distinta ⇒ NO son duplicados (desempate del
// escalón 1 por correct_answer).
func TestDedupe_SameTextDifferentAnswer_NotDup(t *testing.T) {
	q := "selecciona la opcion correcta"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "multiple_choice", q, "roma", []string{"roma", "paris"}, []string{"i1"}),
		candRecord("b", 1, "multiple_choice", q, "paris", []string{"roma", "paris"}, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{q: {1, 0, 0}}}
	judge := &fakeJudge{equal: func(_, _ string) bool { return true }} // no debería llamarse

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("el desempate por respuesta se resuelve en letras, no en LLM: calls=%d", judge.calls)
	}
	if rep.DroppedDup != 0 || rep.Clusters != 0 {
		t.Fatalf("respuestas distintas NO son duplicados: %+v", rep)
	}
	if rep.PairsText != 1 {
		t.Fatalf("el par debía resolverse en el escalón de letras: %+v", rep)
	}
}

// Idempotencia: un segundo Run no reprocesa terminales ni reagrupa; no calcula embeddings
// de nuevo (ya persistidos) y no emite updates.
func TestDedupe_Idempotent_ReRun(t *testing.T) {
	q := "cual es la capital de francia"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "short_answer", q, "paris", nil, []string{"i1"}),
		candRecord("b", 1, "short_answer", q, "paris", nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{q: {1, 0, 0}}}
	judge := &fakeJudge{equal: neverEqual}
	pass := newPass(store, emb, judge)

	if _, err := pass.Run(context.Background(), "job-1"); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	batchesAfter1 := len(store.updateBatches)
	callsAfter1 := emb.calls

	rep2, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	// En el 2º Run solo 'a' sigue en candidate → 1 sola candidata viva → sin pares.
	if rep2.Processed != 1 {
		t.Fatalf("el 2º Run debía ver 1 sola candidata viva: %+v", rep2)
	}
	if rep2.DroppedDup != 0 || rep2.Clusters != 0 {
		t.Fatalf("el 2º Run no debía reagrupar: %+v", rep2)
	}
	if len(store.updateBatches) != batchesAfter1 {
		t.Fatalf("el 2º Run no debía emitir updates (batches %d→%d)", batchesAfter1, len(store.updateBatches))
	}
	if emb.calls != callsAfter1 {
		t.Fatalf("el 2º Run no debía recalcular embeddings (calls %d→%d)", callsAfter1, emb.calls)
	}
}

// Representante determinista: empate en source_ideas ⇒ gana el de menor chunk_sequence.
func TestDedupe_Representative_TieByChunkSequence(t *testing.T) {
	q := "cual es la capital de francia"
	// 'b' (seq 1) y 'a' (seq 3): mismo nº de source_ideas → gana 'b' (menor seq).
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 3, "short_answer", q, "paris", nil, []string{"i1"}),
		candRecord("b", 1, "short_answer", q, "paris", nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{q: {1, 0, 0}}}
	judge := &fakeJudge{equal: neverEqual}

	if _, err := newPass(store, emb, judge).Run(context.Background(), "job-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	st := statusByID(store)
	if st["b"] != statusCandidate || st["a"] != statusDroppedDup {
		t.Fatalf("la representante en empate debía ser 'b' (menor chunk_sequence): %v", st)
	}
}

// Cadena transitiva: A~B por texto y B~C por coseno ⇒ un solo grupo {A,B,C}.
func TestDedupe_TransitiveCluster(t *testing.T) {
	qAB := "que es la mitosis"
	qC := "describe el proceso de division celular mitosis"
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "open_ended", qAB, nil, nil, []string{"i1", "i2", "i3"}),
		candRecord("b", 1, "open_ended", qAB, nil, nil, []string{"i1"}),
		candRecord("c", 2, "open_ended", qC, nil, nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{
		qAB: {1, 0.02, 0},
		qC:  {1, 0, 0.02},
	}}
	judge := &fakeJudge{equal: neverEqual}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Clusters != 1 || rep.DroppedDup != 2 {
		t.Fatalf("debía formarse un único grupo de 3: %+v", rep)
	}
	st := statusByID(store)
	if st["a"] != statusCandidate {
		t.Fatalf("la representante debía ser 'a' (más source_ideas): %v", st)
	}
	g := groupByID(store)
	if g["a"] == nil || g["b"] == nil || g["c"] == nil || *g["a"] != *g["b"] || *g["b"] != *g["c"] {
		t.Fatalf("los 3 debían compartir dedupe_group: %v", g)
	}
}

// Terminales preexistentes se saltan: una candidata ya dropped_dup no entra al agrupado.
func TestDedupe_SkipsTerminalStatuses(t *testing.T) {
	q := "cual es la capital de francia"
	rec := candRecord("done", 0, "short_answer", q, "paris", nil, []string{"i1"})
	rec.Status = statusDroppedDup
	store := &fakeStore{records: []m2m.CandidateRecord{
		rec,
		candRecord("live", 1, "short_answer", "otra pregunta distinta totalmente", "x", nil, []string{"i1"}),
	}}
	emb := &fakeEmbedder{vecs: map[string][]float32{"otra pregunta distinta totalmente": {1, 0, 0}}}
	judge := &fakeJudge{equal: neverEqual}

	rep, err := newPass(store, emb, judge).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Processed != 1 {
		t.Fatalf("solo la candidata viva debía procesarse: %+v", rep)
	}
	if emb.calls != 0 {
		t.Fatalf("con 1 sola candidata viva no hay pares → no se embebe: calls=%d", emb.calls)
	}
}
