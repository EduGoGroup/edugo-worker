package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

func strPtr(s string) *string { return &s }

func newChunkedStore(t *testing.T, n int) *memoryStore {
	t.Helper()
	s := newMemoryStore("job-1", "")
	chunks := make([]m2m.ChunkInput, n)
	for i := range chunks {
		chunks[i] = m2m.ChunkInput{Seq: i, ChunkText: "texto del trozo"}
	}
	if err := s.SaveChunks(context.Background(), "job-1", chunks); err != nil {
		t.Fatalf("SaveChunks: %v", err)
	}
	return s
}

func TestMemoryStore_NextPendingChunkChainsPrevSummary(t *testing.T) {
	ctx := context.Background()
	s := newChunkedStore(t, 3)

	first, err := s.GetNextPendingChunk(ctx, "job-1")
	if err != nil || first == nil || first.Seq != 0 || first.PrevSummary != nil {
		t.Fatalf("primer trozo: %+v, %v (se espera seq 0 sin resumen previo)", first, err)
	}
	if err := s.SaveChunkArtifacts(ctx, first.ChunkID, strPtr("resumen del 0"), json.RawMessage(`{}`), nil); err != nil {
		t.Fatalf("SaveChunkArtifacts: %v", err)
	}
	second, _ := s.GetNextPendingChunk(ctx, "job-1")
	if second == nil || second.Seq != 1 || second.PrevSummary == nil || *second.PrevSummary != "resumen del 0" {
		t.Fatalf("segundo trozo: %+v (se espera el resumen del 0)", second)
	}
	// Un trozo aislado no deja resumen: el siguiente hereda el del más cercano que lo tenga.
	if err := s.MarkChunkFailed(ctx, second.ChunkID, "sin calidad"); err != nil {
		t.Fatalf("MarkChunkFailed: %v", err)
	}
	third, _ := s.GetNextPendingChunk(ctx, "job-1")
	if third == nil || third.Seq != 2 || third.PrevSummary == nil || *third.PrevSummary != "resumen del 0" {
		t.Fatalf("tercer trozo: %+v (se espera el resumen del 0, saltando el fallido)", third)
	}
	if err := s.SaveChunkArtifacts(ctx, third.ChunkID, strPtr("r2"), json.RawMessage(`{}`), nil); err != nil {
		t.Fatalf("SaveChunkArtifacts: %v", err)
	}
	if next, _ := s.GetNextPendingChunk(ctx, "job-1"); next != nil {
		t.Fatalf("no deberían quedar pendientes: %+v", next)
	}
	job, _ := s.GetJob(ctx, "job-1")
	if job.ChunkCounts["done"] != 2 || job.ChunkCounts["failed"] != 1 {
		t.Fatalf("conteos de trozos: %v", job.ChunkCounts)
	}
	// Un trozo cerrado no se vuelve a cerrar ni se reemplaza el porcionado.
	if err := s.SaveChunkArtifacts(ctx, third.ChunkID, nil, nil, nil); !errors.Is(err, m2m.ErrPipelineConflict) {
		t.Fatalf("cerrar dos veces: %v (se espera conflicto)", err)
	}
	if err := s.SaveChunks(ctx, "job-1", nil); !errors.Is(err, m2m.ErrPipelineConflict) {
		t.Fatalf("reporcionar con trozos cerrados: %v (se espera conflicto)", err)
	}
}

func TestMemoryStore_ListCandidatesOrder(t *testing.T) {
	ctx := context.Background()
	s := newChunkedStore(t, 2)
	// Se cierra antes el trozo 1 que el 0: el orden de la lista no es el de inserción.
	payload := []m2m.CandidatePayload{{Payload: json.RawMessage(`{"n":1}`)}, {Payload: json.RawMessage(`{"n":2}`)}}
	if err := s.SaveChunkArtifacts(ctx, "chunk-001", strPtr("r"), json.RawMessage(`{}`), payload); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveChunkArtifacts(ctx, "chunk-000", strPtr("r"), json.RawMessage(`{}`), payload); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListCandidates(ctx, "job-1")
	if err != nil || len(got) != 4 {
		t.Fatalf("ListCandidates: %d, %v", len(got), err)
	}
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.ChunkSequence > b.ChunkSequence || (a.ChunkSequence == b.ChunkSequence && a.ID >= b.ID) {
			t.Fatalf("orden roto en %d: %s(trozo %d) antes de %s(trozo %d)", i, a.ID, a.ChunkSequence, b.ID, b.ChunkSequence)
		}
	}
	if got[0].ChunkSequence != 0 || got[0].Status != candStatusCandidate {
		t.Fatalf("primera candidata: %+v", got[0])
	}
}

func TestMemoryStore_TerminalStatusConflict(t *testing.T) {
	ctx := context.Background()
	s := newMemoryStore("job-1", "")
	s.seedCandidates(json.RawMessage(`{}`), json.RawMessage(`{}`))
	dropped, selected := candStatusDroppedDup, candStatusSelected
	score := 0.7

	if _, err := s.UpdateCandidates(ctx, []m2m.CandidateUpdate{{ID: "cand-0001", Status: &dropped}}); err != nil {
		t.Fatalf("a terminal: %v", err)
	}
	// El mismo estado es idempotente; otro distinto es un conflicto y no escribe NADA del lote.
	if _, err := s.UpdateCandidates(ctx, []m2m.CandidateUpdate{{ID: "cand-0001", Status: &dropped}}); err != nil {
		t.Fatalf("mismo estado terminal: %v", err)
	}
	_, err := s.UpdateCandidates(ctx, []m2m.CandidateUpdate{
		{ID: "cand-0002", Score: &score},
		{ID: "cand-0001", Status: &selected},
	})
	if !errors.Is(err, m2m.ErrPipelineConflict) {
		t.Fatalf("terminal a otro estado: %v (se espera conflicto)", err)
	}
	got, _ := s.ListCandidates(ctx, "job-1")
	if got[0].Status != candStatusDroppedDup || got[1].Score != nil {
		t.Fatalf("el lote en conflicto escribió algo: %+v", got)
	}
	if _, err := s.UpdateCandidates(ctx, []m2m.CandidateUpdate{{ID: "no-existe", Score: &score}}); !errors.Is(err, m2m.ErrLearningPermanent) {
		t.Fatalf("candidata inexistente: %v (se espera permanente)", err)
	}

	// El job: done solo desde processing, y failed no sale de done.
	j := newMemoryStore("job-2", "")
	if err := j.UpdateJobStatus(ctx, "job-2", storeStatusDone, 1, nil); !errors.Is(err, m2m.ErrPipelineConflict) {
		t.Fatalf("pending a done: %v (se espera conflicto)", err)
	}
	_ = j.UpdateJobStatus(ctx, "job-2", storeStatusProcessing, 0, nil)
	if err := j.UpdateJobStatus(ctx, "job-2", storeStatusDone, 1, nil); err != nil {
		t.Fatalf("processing a done: %v", err)
	}
	if err := j.UpdateJobStatus(ctx, "job-2", storeStatusFailed, 2, nil); !errors.Is(err, m2m.ErrPipelineConflict) {
		t.Fatalf("done a failed: %v (se espera conflicto)", err)
	}
	// Sin seleccionadas, la entrega es un permanente y el job no gana borrador.
	if _, _, err := j.DeliverJob(ctx, "job-2"); !errors.Is(err, m2m.ErrLearningPermanent) || j.delivered() {
		t.Fatalf("entrega sin seleccionadas: %v, delivered=%v", err, j.delivered())
	}
}

func TestMemoryStore_JobIdeasUnion(t *testing.T) {
	ctx := context.Background()
	s := newChunkedStore(t, 3)
	art := func(ideas ...string) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"version": 1, "main_ideas": ideas, "chunk_topic": "t"})
		return raw
	}
	_ = s.SaveChunkArtifacts(ctx, "chunk-000", strPtr("r"), art("idea A", "idea B"), nil)
	_ = s.SaveChunkArtifacts(ctx, "chunk-001", strPtr("r"), art("idea B", "idea C"), nil)
	// El tercer trozo queda pendiente: sus ideas no cuentan.
	got, err := s.GetJobIdeas(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"idea A", "idea B", "idea C"}
	if len(got) != len(want) {
		t.Fatalf("ideas del job: %v, se espera %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ideas del job: %v, se espera %v", got, want)
		}
	}
}
