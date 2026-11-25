package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

const (
	testRailPDF     = "testdata/material/ciclo_del_agua.pdf"
	testRailLongPDF = "testdata/material/fotosintesis.pdf" // da más de un trozo
)

// oneHotEmbedder da a cada texto un vector ortogonal a los demás: el coseno es 0 y
// el escalón de significado decide «no duplicadas» sin llamar al juez.
type oneHotEmbedder struct{ next int }

func (e *oneHotEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 256)
		v[e.next%256] = 1
		e.next++
		out[i] = v
	}
	return out, nil
}

// Enunciados sin letras en común entre sí, para que el escalón de letras no los junte.
var fakeQuestions = []string{
	"¿Qué proceso convierte el agua líquida en vapor?",
	"Nombra dos lugares donde se acumula nieve durante años.",
	"¿Por qué llueve cuando las nubes se enfrían?",
	"Explica cómo vuelve el río al océano.",
	"¿Cuánta energía solar recibe un lago grande?",
	"Describe la infiltración bajo tierra firme.",
}

// workingModel es un proveedor de mentira que hace bien su trabajo: lee cada trozo y
// propone tres candidatas válidas de tipos distintos.
func workingModel() *fakeModel {
	var n atomic.Int64
	return &fakeModel{
		digest: func(llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
			return &llm.DigestChunkResult{
				Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: []string{"idea uno", "idea dos"}, ChunkTopic: "el agua"},
				Summary:   "Resumen corto del trozo.",
			}, nil
		},
		propose: func(in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
			q := func() string { return fakeQuestions[int(n.Add(1)-1)%len(fakeQuestions)] }
			return []materialpipeline.CandidatePayloadV1{
				{Version: 1, QuestionType: "short_answer", QuestionText: q(), CorrectAnswer: json.RawMessage(`"evaporación"`), SourceIdeas: in.Artifacts.MainIdeas[:1]},
				{Version: 1, QuestionType: "true_false", QuestionText: q(), CorrectAnswer: json.RawMessage(`"true"`), SourceIdeas: in.Artifacts.MainIdeas[1:]},
				{Version: 1, QuestionType: "open_ended", QuestionText: q(), SourceIdeas: in.Artifacts.MainIdeas},
			}, nil
		},
	}
}

func testDeps(model railModel) railDeps {
	return railDeps{
		model:    model,
		embedder: &oneHotEmbedder{},
		cfg:      (&config.Config{}).GetMaterialPipelineConfigWithDefaults(),
		log:      quietLog(),
	}
}

func prepareTestCase(t *testing.T) preparedCase { return prepareTestPDF(t, testRailPDF) }

func prepareTestPDF(t *testing.T, pdfPath string) preparedCase {
	t.Helper()
	cfg := (&config.Config{}).GetMaterialPipelineConfigWithDefaults()
	cases, err := prepareMaterialCases([]materialCase{{ID: "caso", PDF: pdfPath}}, chunkConfigFrom(cfg), quietLog())
	if err != nil {
		t.Fatalf("preparando el caso: %v", err)
	}
	return cases[0]
}

// saveCaseRun guarda la corrida como lo hace el modo y comprueba que el fichero queda.
func saveCaseRun(t *testing.T, run materialCaseRun) {
	t.Helper()
	path, err := writeRun(t.TempDir(), runInfo{Mode: "material", Model: "fake"}, materialRunData{Cases: []materialCaseRun{run}})
	if err != nil {
		t.Fatalf("la corrida no se pudo guardar: %v", err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("la corrida guardada no está o está vacía: %v", err)
	}
}

func TestRail_ProcessOverMemoryStore(t *testing.T) {
	c := prepareTestCase(t)
	run := runMaterialCase(context.Background(), testDeps(workingModel()), c)

	if run.Outcome != outcomeComplete || len(run.Attempts) != 1 || run.Attempts[0].Error != "" {
		t.Fatalf("desenlace %q con entregas %+v (se espera completo a la primera)", run.Outcome, run.Attempts)
	}
	if run.JobStatus != storeStatusDone || run.JobPhase != 2 {
		t.Fatalf("job en %s fase %d (se espera done fase 2)", run.JobStatus, run.JobPhase)
	}
	g := run.Metrics.Generation
	if g.Chunks == 0 || g.Chunks != c.ExpectedChunks || len(run.Chunks) != g.Chunks {
		t.Fatalf("trozos: %d (esperados %d)", g.Chunks, c.ExpectedChunks)
	}
	if g.ProposedTotal != 3*g.Chunks || len(run.Candidates) != g.ProposedTotal || g.ChunksWithoutOpenEnded != 0 {
		t.Fatalf("candidatas: propuestas %d, guardadas %d, trozos sin abierta %d", g.ProposedTotal, len(run.Candidates), g.ChunksWithoutOpenEnded)
	}
	p := run.Passes
	if len(p.Dedupe) != 1 || len(p.Relevance) != 1 || len(p.Quality) != 1 || len(p.Selection) != 1 {
		t.Fatalf("informes de las pasadas: %d/%d/%d/%d (se espera uno de cada una)", len(p.Dedupe), len(p.Relevance), len(p.Quality), len(p.Selection))
	}
	d := run.Metrics.Draft
	if d.Selected == 0 || d.Selected != len(d.Questions) || d.Target != 20 || !d.SelectionRan {
		t.Fatalf("borrador: %+v", d)
	}
	cost := run.Metrics.Cost
	if cost.Digest.Calls != g.Chunks || cost.Propose.Calls != g.Chunks || cost.Relevance.Calls != g.ProposedTotal || cost.Redeliveries != 0 {
		t.Fatalf("coste: %+v", cost)
	}
	if run.Metrics.Support.CandidatesAllTrue != len(run.Candidates) {
		t.Fatalf("source_ideas: %+v (todas eran ideas dadas)", run.Metrics.Support)
	}
	saveCaseRun(t, run)
}

func TestRail_InvalidModelOutputIsData(t *testing.T) {
	model := workingModel()
	model.propose = func(llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
		return nil, fmt.Errorf("%w: JSON sin cierre", llm.ErrLLMQuality)
	}
	run := runMaterialCase(context.Background(), testDeps(model), prepareTestCase(t))

	// Una salida rota no es un fallo del arnés: el caso termina, sin borrador, y se guarda.
	if run.Outcome != outcomeNoDraft || len(run.Attempts) != 1 {
		t.Fatalf("desenlace %q con %d entregas (se espera sin borrador, sin reentrega)", run.Outcome, len(run.Attempts))
	}
	cost := run.Metrics.Cost
	chunks := run.Metrics.Generation.Chunks
	// El procesador reintenta una vez por calidad en cada trozo: dos llamadas, las dos inválidas.
	if cost.Propose.Calls != 2*chunks || cost.Propose.InvalidOutputs != 2*chunks || cost.Propose.QualityRetries != chunks {
		t.Fatalf("cuentas de «preguntar»: %+v con %d trozos", cost.Propose, chunks)
	}
	if len(run.Candidates) != 0 || run.Metrics.Draft.Selected != 0 {
		t.Fatalf("no debería haber candidatas: %d", len(run.Candidates))
	}
	saveCaseRun(t, run)
}

func TestRail_TransientErrorRedeliversThenSavesPartial(t *testing.T) {
	// El proveedor cae en la segunda lectura y no vuelve: cada reentrega reanuda ahí.
	model := workingModel()
	okDigest := model.digest
	calls := 0
	model.digest = func(in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
		calls++
		if calls == 1 {
			return okDigest(in)
		}
		return nil, errors.New("ollama: context deadline exceeded")
	}
	c := prepareTestPDF(t, testRailLongPDF)
	if c.ExpectedChunks < 2 {
		t.Fatalf("el PDF de prueba da %d trozo: hace falta un segundo trozo que falle", c.ExpectedChunks)
	}
	run := runMaterialCase(context.Background(), testDeps(model), c)

	if run.Outcome != outcomeIncomplete || len(run.Attempts) != 1+maxRedeliveries {
		t.Fatalf("desenlace %q con %d entregas (se espera incompleto tras %d)", run.Outcome, len(run.Attempts), 1+maxRedeliveries)
	}
	for i, a := range run.Attempts {
		if a.ErrorKind != errKindProvider || a.Permanent {
			t.Fatalf("entrega %d: %+v (se espera error del proveedor, transitorio)", i, a)
		}
	}
	if run.Metrics.Cost.Redeliveries != maxRedeliveries || run.Metrics.Cost.Digest.ProviderErrors != 1+maxRedeliveries {
		t.Fatalf("coste: %+v", run.Metrics.Cost)
	}
	// Lo hecho antes del fallo se conserva: el primer trozo, cerrado, con sus candidatas.
	if run.Chunks[0].Status != storeStatusDone || len(run.Candidates) != 3 || run.JobStatus != storeStatusProcessing {
		t.Fatalf("lo parcial: trozo 0 %s, %d candidatas, job %s", run.Chunks[0].Status, len(run.Candidates), run.JobStatus)
	}
	saveCaseRun(t, run)

	// Si el proveedor vuelve, la reentrega termina el caso.
	recovered := workingModel()
	okDigest = recovered.digest
	failed := false
	recovered.digest = func(in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
		if !failed {
			failed = true
			return nil, errors.New("ollama: connection refused")
		}
		return okDigest(in)
	}
	run = runMaterialCase(context.Background(), testDeps(recovered), c)
	if run.Outcome != outcomeComplete || len(run.Attempts) != 2 || run.Metrics.Cost.Redeliveries != 1 {
		t.Fatalf("tras volver el proveedor: %q con %d entregas", run.Outcome, len(run.Attempts))
	}
}

func TestRail_NoSelectedIsDataNotRedelivery(t *testing.T) {
	model := workingModel()
	model.relevance = func(llm.RelevanceRequest) (llm.RelevanceResult, error) {
		return llm.RelevanceResult{Score: 0}, nil // nada es relevante: todo cae
	}
	run := runMaterialCase(context.Background(), testDeps(model), prepareTestCase(t))

	// Sin seleccionadas la entrega falla con un permanente: es contenido del modelo, no
	// se reentrega. Completo es «done con borrador», no «Process sin error».
	if run.Outcome != outcomeNoDraft || len(run.Attempts) != 1 || !run.Attempts[0].Permanent {
		t.Fatalf("desenlace %q con entregas %+v (se espera sin borrador, un permanente, sin reentrega)", run.Outcome, run.Attempts)
	}
	if run.Metrics.Draft.Selected != 0 || run.Metrics.Reduction.DroppedIrrelevant != len(run.Candidates) {
		t.Fatalf("reducción: %+v", run.Metrics.Reduction)
	}
	// Learning no deja pasar un job de done a failed: queda done, en fase 1 y sin borrador.
	if run.JobStatus != storeStatusDone || run.JobPhase != 1 {
		t.Fatalf("job en %s fase %d (se espera done fase 1)", run.JobStatus, run.JobPhase)
	}
	saveCaseRun(t, run)
}
