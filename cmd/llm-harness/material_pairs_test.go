package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
)

func quietLog() logger.Logger {
	return logger.NewSlogAdapter(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// fakeModel es un proveedor de mentira: cada llamada se programa con una función.
type fakeModel struct {
	digest    func(in llm.DigestChunkInput) (*llm.DigestChunkResult, error)
	propose   func(in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error)
	judgeFn   func(req llm.PairEquivalenceRequest) (llm.ReviewResult, error)
	relevance func(req llm.RelevanceRequest) (llm.RelevanceResult, error)
	judged    int
}

func (f *fakeModel) Name() string { return "fake" }

func (f *fakeModel) DigestChunk(_ context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	return f.digest(in)
}

func (f *fakeModel) ProposeCandidates(_ context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	return f.propose(in)
}

func (f *fakeModel) JudgePairEquivalence(_ context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	f.judged++
	if f.judgeFn == nil {
		return llm.ReviewResult{Verdict: llm.VerdictIncorrect}, nil
	}
	return f.judgeFn(req)
}

func (f *fakeModel) ScoreRelevance(_ context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error) {
	if f.relevance == nil {
		return llm.RelevanceResult{Score: 0.9}, nil
	}
	return f.relevance(req)
}

const testPairsPath = "testdata/material/pairs.json"

func writePairs(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pairs.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPairs_LoadRejectsDuplicateIDsAndBadLabels(t *testing.T) {
	f, sum, err := loadMaterialPairs(testPairsPath)
	if err != nil {
		t.Fatalf("la batería congelada no carga: %v", err)
	}
	if len(f.Pairs) != 40 || sum == "" {
		t.Fatalf("la batería congelada trae %d parejas (se esperan 40)", len(f.Pairs))
	}
	perCategory := map[string]int{}
	for _, p := range f.Pairs {
		perCategory[p.Category]++
	}
	for _, c := range f.Categories {
		if perCategory[c] != 10 {
			t.Errorf("categoría %s: %d parejas (se esperan 10)", c, perCategory[c])
		}
	}

	member := `{"version":1,"question_type":"short_answer","question_text":"¿Cuándo?","correct_answer":"1810"}`
	pair := func(id, label, a string) string {
		return fmt.Sprintf(`{"id":%q,"category":"c","label":%q,"a":%s,"b":%s}`, id, label, a, member)
	}
	file := func(pairs ...string) string {
		return `{"categories":["c"],"pairs":[` + strings.Join(pairs, ",") + `]}`
	}
	bad := map[string]string{
		"ids repetidos":          file(pair("p1", "dup", member), pair("p1", "no_dup", member)),
		"etiqueta desconocida":   file(pair("p1", "igual", member)),
		"categoría sin declarar": `{"categories":["otra"],"pairs":[` + pair("p1", "dup", member) + `]}`,
		"miembro fuera de contrato": file(pair("p1", "dup",
			`{"version":1,"question_type":"multiple_choice","question_text":"¿Cuál?","options":["a"],"correct_answer":"z"}`)),
		"sin parejas": file(),
	}
	for name, body := range bad {
		if _, _, err := loadMaterialPairs(writePairs(t, body)); err == nil {
			t.Errorf("%s: debería rechazarse", name)
		}
	}
	if _, _, err := loadMaterialPairs(writePairs(t, file(pair("p1", "dup", member), pair("p2", "no_dup", member)))); err != nil {
		t.Errorf("una batería válida no carga: %v", err)
	}
}

func TestPairs_GreyZoneEmbedderForcesJudge(t *testing.T) {
	f, _, err := loadMaterialPairs(testPairsPath)
	if err != nil {
		t.Fatal(err)
	}
	judge := &fakeModel{}
	cfg := (&config.Config{}).GetMaterialPipelineConfigWithDefaults()
	dedupeCfg := reduce.Config{DupHigh: cfg.DedupeHigh, DupLow: cfg.DedupeLow}
	byText := 0
	for _, p := range f.Pairs {
		before := judge.judged
		m, err := measurePair(context.Background(), p, greyZoneEmbedder{}, judge, dedupeCfg, quietLog())
		if err != nil {
			t.Fatalf("pareja %s: %v", p.ID, err)
		}
		switch m.Tier {
		case pairTierText:
			byText++
			if judge.judged != before {
				t.Errorf("pareja %s: la decidieron las letras y aun así se llamó al juez", p.ID)
			}
		case pairTierLLM:
			if judge.judged != before+1 {
				t.Errorf("pareja %s: se esperaba una llamada al juez, hubo %d", p.ID, judge.judged-before)
			}
		default:
			t.Errorf("pareja %s: decidió el escalón %q; con el embedder fijo solo caben letras o juez", p.ID, m.Tier)
		}
	}
	if byText+judge.judged != len(f.Pairs) {
		t.Fatalf("letras %d + juez %d no suman las %d parejas", byText, judge.judged, len(f.Pairs))
	}
}

func TestPairs_CountsByCategory(t *testing.T) {
	member := func(text string) []byte {
		return []byte(fmt.Sprintf(`{"version":1,"question_type":"short_answer","question_text":%q,"correct_answer":"x"}`, text))
	}
	b := member("Una pregunta completamente distinta sobre otro asunto del temario")
	f := pairsFile{Categories: []string{"c"}, Pairs: []materialPair{
		{ID: "hit", Category: "c", Label: pairLabelDup, A: member("alfa dice que sí"), B: b},
		{ID: "missed", Category: "c", Label: pairLabelDup, A: member("beta dice que no"), B: b},
		{ID: "false", Category: "c", Label: pairLabelNoDup, A: member("gamma dice que sí"), B: b},
		{ID: "invalid", Category: "c", Label: pairLabelNoDup, A: member("delta rompe la salida"), B: b},
	}}
	judge := &fakeModel{judgeFn: func(req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
		switch {
		case strings.Contains(req.Expected, "rompe"):
			return llm.ReviewResult{}, fmt.Errorf("%w: no parsea", llm.ErrLLMQuality)
		case strings.Contains(req.Expected, "sí"):
			return llm.ReviewResult{Verdict: llm.VerdictCorrect}, nil
		default:
			return llm.ReviewResult{Verdict: llm.VerdictIncorrect}, nil
		}
	}}
	cfg := (&config.Config{}).GetMaterialPipelineConfigWithDefaults()
	runs, stats, _, err := runPairs(context.Background(), f, judge, greyZoneEmbedder{}, cfg, quietLog(), nil)
	if err != nil {
		t.Fatalf("runPairs: %v", err)
	}
	rows := pairsTable(f.Categories, runs)
	if len(rows) != 1 {
		t.Fatalf("filas: %d", len(rows))
	}
	for name, c := range map[string]pairCounts{pairModeReal: rows[0].Real, pairModeJudgeOnly: rows[0].JudgeOnly} {
		if c.Pairs != 4 || c.TierLLM != 4 || c.Hits != 1 || c.MissedDups != 1 || c.FalseDups != 1 || c.InvalidOutputs != 1 || c.ProviderErrors != 0 {
			t.Errorf("%s: %+v (se espera 4 parejas por el juez: 1 igual, 1 escapada, 1 falsa, 1 inválida)", name, c)
		}
	}
	// Dos medidas por pareja, una llamada al juez cada una, y la inválida cuenta.
	if stats.Calls != 8 || stats.InvalidOutputs != 2 {
		t.Errorf("cuentas del juez: %+v (se esperan 8 llamadas y 2 inválidas)", stats)
	}
}
