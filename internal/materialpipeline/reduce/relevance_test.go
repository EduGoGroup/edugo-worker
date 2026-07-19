package reduce

import (
	"context"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// --- fakes de la pasada 2 ---

// fakeRelevanceJudge puntúa según una función inyectada; cuenta llamadas para verificar
// idempotencia y reintentos.
type fakeRelevanceJudge struct {
	fn    func(req llm.RelevanceRequest) (llm.RelevanceResult, error)
	calls int
}

func (j *fakeRelevanceJudge) ScoreRelevance(_ context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error) {
	j.calls++
	return j.fn(req)
}

func constScore(s float64) func(llm.RelevanceRequest) (llm.RelevanceResult, error) {
	return func(llm.RelevanceRequest) (llm.RelevanceResult, error) {
		return llm.RelevanceResult{Score: s}, nil
	}
}

// fakeChunkResolver resuelve chunk_text desde un mapa (chunkID→texto); err fuerza fallo.
type fakeChunkResolver struct {
	texts map[string]string
	err   error
	calls int
}

func (r *fakeChunkResolver) ChunkText(_ context.Context, chunkID string) (string, error) {
	r.calls++
	if r.err != nil {
		return "", r.err
	}
	return r.texts[chunkID], nil
}

func scoreByID(s *fakeStore) map[string]*float64 {
	m := make(map[string]*float64, len(s.records))
	for _, r := range s.records {
		m[r.ID] = r.Score
	}
	return m
}

// --- tests de relevancia ---

// Central (score alto) sobrevive con score; periférica bajo umbral se descarta; no-responde
// se descarta. Una candidata por llamada.
func TestRelevance_CentralPeriphericaNoResponde(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("central", 0, "short_answer", "pregunta central", "r", nil, []string{"idea uno"}),
		candRecord("periferica", 1, "short_answer", "pregunta periferica", "r", nil, []string{"idea dos"}),
		candRecord("fuera", 2, "short_answer", "pregunta fuera", "r", nil, []string{"idea tres"}),
	}}
	judge := &fakeRelevanceJudge{fn: func(req llm.RelevanceRequest) (llm.RelevanceResult, error) {
		switch req.QuestionText {
		case "pregunta central":
			return llm.RelevanceResult{Score: 0.9}, nil
		case "pregunta periferica":
			return llm.RelevanceResult{Score: 0.2}, nil // < 0.4 → descartada
		default:
			return llm.RelevanceResult{Score: 0.0}, nil
		}
	}}

	pass := NewRelevancePass(store, judge, nil, nil, RelevanceConfig{}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Processed != 3 || rep.Scored != 3 {
		t.Fatalf("Processed/Scored = %d/%d, quiero 3/3", rep.Processed, rep.Scored)
	}
	if rep.DroppedIrrelevant != 2 {
		t.Fatalf("DroppedIrrelevant = %d, quiero 2", rep.DroppedIrrelevant)
	}
	st := statusByID(store)
	if st["central"] != statusCandidate {
		t.Fatalf("la central debe seguir en candidate, está en %q", st["central"])
	}
	if st["periferica"] != statusDroppedIrrelevant || st["fuera"] != statusDroppedIrrelevant {
		t.Fatalf("periferica/fuera deben caer a dropped_irrelevant: %q/%q", st["periferica"], st["fuera"])
	}
	sc := scoreByID(store)
	if sc["central"] == nil || *sc["central"] != 0.9 {
		t.Fatalf("score de central debe ser 0.9, es %v", sc["central"])
	}
	if !rep.IdeasFromSource {
		t.Fatalf("IdeasFromSource debe reportar la desviación (main_ideas de source_ideas)")
	}
}

// Idempotencia: una candidata que YA trae score no se re-llama.
func TestRelevance_Idempotent_ScorePresente(t *testing.T) {
	rec := candRecord("a", 0, "short_answer", "q", "r", nil, []string{"idea"})
	existing := 0.8
	rec.Score = &existing
	store := &fakeStore{records: []m2m.CandidateRecord{rec}}
	judge := &fakeRelevanceJudge{fn: constScore(0.1)}

	pass := NewRelevancePass(store, judge, nil, nil, RelevanceConfig{}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 {
		t.Fatalf("no debe re-llamar al modelo si ya hay score, llamó %d veces", judge.calls)
	}
	if rep.SkippedScored != 1 || rep.Processed != 0 {
		t.Fatalf("SkippedScored/Processed = %d/%d, quiero 1/0", rep.SkippedScored, rep.Processed)
	}
	if statusByID(store)["a"] != statusCandidate {
		t.Fatalf("la candidata ya puntuada no debe cambiar de estado")
	}
}

// Las terminales se saltan (idempotencia por status).
func TestRelevance_SkipsTerminal(t *testing.T) {
	rec := candRecord("d", 0, "short_answer", "q", "r", nil, []string{"idea"})
	rec.Status = statusDroppedDup
	store := &fakeStore{records: []m2m.CandidateRecord{rec}}
	judge := &fakeRelevanceJudge{fn: constScore(0.9)}

	pass := NewRelevancePass(store, judge, nil, nil, RelevanceConfig{}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 0 || rep.Processed != 0 {
		t.Fatalf("una terminal no debe procesarse (calls=%d, processed=%d)", judge.calls, rep.Processed)
	}
}

// Fallo del LLM dos veces → score nil, NO se descarta (conservador). Reintenta una vez.
func TestRelevance_LLMFallaDosVeces_NoDescarta(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "short_answer", "q", "r", nil, []string{"idea"}),
	}}
	judge := &fakeRelevanceJudge{fn: func(llm.RelevanceRequest) (llm.RelevanceResult, error) {
		return llm.RelevanceResult{}, errors.New("salida malformada")
	}}

	pass := NewRelevancePass(store, judge, nil, nil, RelevanceConfig{}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if judge.calls != 2 {
		t.Fatalf("debe reintentar una vez (2 llamadas), hizo %d", judge.calls)
	}
	if rep.Unscored != 1 || rep.Scored != 0 || rep.DroppedIrrelevant != 0 {
		t.Fatalf("Unscored/Scored/Dropped = %d/%d/%d, quiero 1/0/0", rep.Unscored, rep.Scored, rep.DroppedIrrelevant)
	}
	if statusByID(store)["a"] != statusCandidate {
		t.Fatalf("un fallo de infra NO debe descartar: debe seguir en candidate")
	}
	if scoreByID(store)["a"] != nil {
		t.Fatalf("score debe quedar nil ante fallo del modelo")
	}
}

// Fallo la primera, éxito la segunda: el reintento rescata el puntaje.
func TestRelevance_ReintentoRescata(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "short_answer", "q", "r", nil, []string{"idea"}),
	}}
	attempt := 0
	judge := &fakeRelevanceJudge{fn: func(llm.RelevanceRequest) (llm.RelevanceResult, error) {
		attempt++
		if attempt == 1 {
			return llm.RelevanceResult{}, errors.New("primer intento malformado")
		}
		return llm.RelevanceResult{Score: 0.7}, nil
	}}

	pass := NewRelevancePass(store, judge, nil, nil, RelevanceConfig{}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Scored != 1 || rep.Unscored != 0 {
		t.Fatalf("el reintento debe rescatar el score (Scored=%d, Unscored=%d)", rep.Scored, rep.Unscored)
	}
	if sc := scoreByID(store)["a"]; sc == nil || *sc != 0.7 {
		t.Fatalf("score debe ser 0.7 tras el reintento, es %v", sc)
	}
}

// Modo "api": una candidata local_only (cita verbatim del chunk) usa SIEMPRE el provider
// local; una candidata sin cita usa el de API.
func TestRelevance_LocalOnly_FuerzaLocalEnModoAPI(t *testing.T) {
	verbatim := words(30) // 30 palabras que estarán íntegras en el chunk
	recVerbatim := candRecord("verb", 0, "short_answer", verbatim, "r", nil, []string{"idea"})
	recVerbatim.ChunkID = "chunk-verb"
	recNormal := candRecord("norm", 1, "short_answer", "pregunta corta y original", "r", nil, []string{"idea"})
	recNormal.ChunkID = "chunk-norm"
	store := &fakeStore{records: []m2m.CandidateRecord{recVerbatim, recNormal}}

	localJudge := &fakeRelevanceJudge{fn: constScore(0.9)}
	apiJudge := &fakeRelevanceJudge{fn: constScore(0.9)}
	resolver := &fakeChunkResolver{texts: map[string]string{
		"chunk-verb": "contexto previo " + words(30) + " contexto posterior",
		"chunk-norm": "un chunk breve sin relacion literal",
	}}

	pass := NewRelevancePass(store, localJudge, apiJudge, resolver,
		RelevanceConfig{Mode: relevanceModeAPI}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// La verbatim va por local; la normal por API.
	if localJudge.calls != 1 {
		t.Fatalf("la candidata local_only debe usar el provider local (localJudge.calls=%d)", localJudge.calls)
	}
	if apiJudge.calls != 1 {
		t.Fatalf("la candidata sin cita debe usar el provider API (apiJudge.calls=%d)", apiJudge.calls)
	}
	if rep.LocalForced != 1 {
		t.Fatalf("LocalForced = %d, quiero 1 (solo la verbatim)", rep.LocalForced)
	}
}

// Modo "api" sin resolver de chunk_text: no se puede verificar la cita → conservador, todo
// va por local (nunca se arriesga una fuga por API).
func TestRelevance_ModoAPI_SinResolver_CaeALocal(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("a", 0, "short_answer", "q", "r", nil, []string{"idea"}),
	}}
	localJudge := &fakeRelevanceJudge{fn: constScore(0.9)}
	apiJudge := &fakeRelevanceJudge{fn: constScore(0.9)}

	pass := NewRelevancePass(store, localJudge, apiJudge, nil,
		RelevanceConfig{Mode: relevanceModeAPI}, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if apiJudge.calls != 0 || localJudge.calls != 1 {
		t.Fatalf("sin resolver todo debe ir por local (local=%d, api=%d)", localJudge.calls, apiJudge.calls)
	}
	if rep.LocalForced != 1 {
		t.Fatalf("LocalForced = %d, quiero 1", rep.LocalForced)
	}
}
