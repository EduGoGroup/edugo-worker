package main

// material_store.go — el almacén en memoria que hace de learning para el modo
// material (AR-IA.3, decisión 1). El procesador real habla con learning por M2M;
// aquí habla con este doble, que guarda un solo job y COPIA, a propósito y en
// pequeño, las reglas de learning de las que depende el riel:
//
//   - el siguiente trozo pendiente trae el resumen del anterior más cercano que lo
//     tenga (internal/infra/postgres/pipeline/chunk.go de learning);
//   - las candidatas se listan por trozo y luego por id;
//   - los conteos de trozos del job, por estado;
//   - la máquina de estados del job (HU-IA.5): a processing o a failed desde pending
//     o processing; done NO es un destino del PATCH (409) y un job terminal no cambia;
//     la fase nunca retrocede (queda la mayor); y la de la candidata (un estado
//     terminal no cambia a otro distinto): lo demás es un conflicto (409);
//   - las ideas del job son la unión de las main_ideas de sus trozos cerrados;
//   - la entrega es lo único que cierra el job en done: procede desde processing,
//     fase 1; falla sin seleccionadas (422, permanente, con el mismo error tipado que
//     el cliente) y sobre un job en otro estado o fase es un conflicto (409).
//
// Es una copia: si learning cambia una de estas reglas, hay que cambiarla aquí.
// Sus tests (material_store_test.go) la fijan.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

const (
	storeStatusPending    = "pending"
	storeStatusProcessing = "processing"
	storeStatusDone       = "done"
	storeStatusFailed     = "failed"

	candStatusCandidate         = "candidate"
	candStatusDroppedDup        = "dropped_dup"
	candStatusDroppedIrrelevant = "dropped_irrelevant"
	candStatusSelected          = "selected"
)

// memChunk es un trozo del job en el almacén.
type memChunk struct {
	id         string
	seq        int
	text       string
	status     string
	summary    *string
	artifacts  json.RawMessage
	failReason string
}

// memoryStore guarda UN job con sus trozos y sus candidatas. Satisface el cliente
// del riel del procesador y las interfaces de las cuatro pasadas del reduce.
type memoryStore struct {
	mu         sync.Mutex
	job        m2m.PipelineJob
	fileURL    string
	chunks     []*memChunk
	candidates []*m2m.CandidateRecord
	nextCand   int

	// Marcas de tiempo del riel, para los segundos por fase.
	chunkedAt   time.Time // fin de la fase 0 (trozos guardados)
	phase1Done  time.Time // fin de la fase 1 (primer PATCH processing con fase 1)
	deliveredAt time.Time
}

// newMemoryStore crea el almacén con un job pendiente cuyo fichero es fileURL (la
// ruta local del PDF: la «descarga» del arnés lee el disco).
func newMemoryStore(jobID, fileURL string) *memoryStore {
	return &memoryStore{
		job:     m2m.PipelineJob{JobID: jobID, MaterialID: "material-" + jobID, Status: storeStatusPending},
		fileURL: fileURL,
	}
}

func (s *memoryStore) notFound(what, id string) error {
	return fmt.Errorf("%w: %s %s no existe (404)", m2m.ErrLearningPermanent, what, id)
}

// GetJob devuelve una copia del job con los conteos de trozos por estado.
func (s *memoryStore) GetJob(_ context.Context, jobID string) (*m2m.PipelineJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return nil, s.notFound("job", jobID)
	}
	job := s.job
	job.ChunkCounts = map[string]int{}
	for _, c := range s.chunks {
		job.ChunkCounts[c.status]++
	}
	return &job, nil
}

// GetFileURL devuelve la ruta local del PDF como si fuera la URL firmada.
func (s *memoryStore) GetFileURL(_ context.Context, jobID string) (*m2m.PresignedFile, error) {
	if jobID != s.job.JobID {
		return nil, s.notFound("job", jobID)
	}
	return &m2m.PresignedFile{URL: s.fileURL}, nil
}

// SaveChunks reemplaza el porcionado; con algún trozo ya cerrado es un conflicto.
func (s *memoryStore) SaveChunks(_ context.Context, jobID string, chunks []m2m.ChunkInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return s.notFound("job", jobID)
	}
	for _, c := range s.chunks {
		if c.status == storeStatusDone {
			return m2m.ErrPipelineConflict
		}
	}
	s.chunks = s.chunks[:0]
	for _, in := range chunks {
		s.chunks = append(s.chunks, &memChunk{
			id:     fmt.Sprintf("chunk-%03d", in.Seq),
			seq:    in.Seq,
			text:   in.ChunkText,
			status: storeStatusPending,
		})
	}
	sort.Slice(s.chunks, func(i, j int) bool { return s.chunks[i].seq < s.chunks[j].seq })
	s.chunkedAt = time.Now()
	return nil
}

// GetNextPendingChunk devuelve el primer trozo pendiente por seq y, a su lado, el
// resumen del trozo anterior más cercano que tenga uno no vacío.
func (s *memoryStore) GetNextPendingChunk(_ context.Context, jobID string) (*m2m.NextChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return nil, s.notFound("job", jobID)
	}
	for i, c := range s.chunks {
		if c.status != storeStatusPending {
			continue
		}
		next := &m2m.NextChunk{ChunkID: c.id, JobID: jobID, Seq: c.seq, ChunkText: c.text, Status: c.status}
		for j := i - 1; j >= 0; j-- {
			if prev := s.chunks[j].summary; prev != nil && *prev != "" {
				summary := *prev
				next.PrevSummary = &summary
				break
			}
		}
		return next, nil
	}
	return nil, nil
}

func (s *memoryStore) chunkByID(chunkID string) *memChunk {
	for _, c := range s.chunks {
		if c.id == chunkID {
			return c
		}
	}
	return nil
}

// SaveChunkArtifacts cierra el trozo con su resumen y sus artefactos e inserta sus
// candidatas. Un trozo ya cerrado es un conflicto.
func (s *memoryStore) SaveChunkArtifacts(_ context.Context, chunkID string, summary *string, artifacts json.RawMessage, candidates []m2m.CandidatePayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.chunkByID(chunkID)
	if c == nil {
		return s.notFound("chunk", chunkID)
	}
	if c.status == storeStatusDone {
		return m2m.ErrPipelineConflict
	}
	c.status = storeStatusDone
	c.summary = summary
	c.artifacts = artifacts
	for _, cand := range candidates {
		s.insertCandidate(c, cand.Payload)
	}
	return nil
}

func (s *memoryStore) insertCandidate(c *memChunk, payload json.RawMessage) *m2m.CandidateRecord {
	s.nextCand++
	rec := &m2m.CandidateRecord{
		ID:            fmt.Sprintf("cand-%04d", s.nextCand),
		ChunkID:       c.id,
		ChunkSequence: c.seq,
		Payload:       payload,
		Status:        candStatusCandidate,
	}
	s.candidates = append(s.candidates, rec)
	return rec
}

// MarkChunkFailed aísla un trozo; si ya está cerrado es un conflicto.
func (s *memoryStore) MarkChunkFailed(_ context.Context, chunkID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.chunkByID(chunkID)
	if c == nil {
		return s.notFound("chunk", chunkID)
	}
	if c.status == storeStatusDone {
		return m2m.ErrPipelineConflict
	}
	c.status = storeStatusFailed
	c.failReason = reason
	return nil
}

// UpdateJobStatus aplica la máquina de estados del job de learning: a processing o
// a failed desde pending o processing; done no es un destino (lo escribe la entrega);
// la fase nunca retrocede.
func (s *memoryStore) UpdateJobStatus(_ context.Context, jobID, status string, phase int16, lastError *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return s.notFound("job", jobID)
	}
	from := s.job.Status
	allowed := false
	switch status {
	case storeStatusProcessing, storeStatusFailed:
		allowed = from == storeStatusPending || from == storeStatusProcessing
	}
	if !allowed {
		return m2m.ErrPipelineConflict
	}
	s.job.Status = status
	if phase > s.job.Phase {
		s.job.Phase = phase
	}
	s.job.LastError = lastError
	// El ancla de la fase 1 se fija una sola vez: los latidos y las reentregas
	// vuelven a pedir processing y no deben moverla.
	if status == storeStatusProcessing && s.job.Phase >= 1 && s.phase1Done.IsZero() {
		s.phase1Done = time.Now()
	}
	return nil
}

// DeliverJob arma el borrador con las seleccionadas y cierra el job en done, fase 2,
// en el orden de learning: una reentrega sobre un job ya entregado devuelve el mismo
// borrador; sin seleccionadas falla con el permanente tipado (el 422 de learning); y
// solo procede desde processing, fase 1 (lo demás es un conflicto, 409).
func (s *memoryStore) DeliverJob(_ context.Context, jobID string) (string, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return "", 0, s.notFound("job", jobID)
	}
	selected := 0
	for _, c := range s.candidates {
		if c.Status == candStatusSelected {
			selected++
		}
	}
	if s.job.AssessmentID != nil {
		return *s.job.AssessmentID, selected, nil
	}
	if selected == 0 {
		return "", 0, fmt.Errorf("%w: job %s (422)", m2m.ErrNoSelectedCandidates, jobID)
	}
	if s.job.Status != storeStatusProcessing || s.job.Phase != 1 {
		return "", 0, m2m.ErrPipelineConflict
	}
	id := "draft-" + jobID
	s.job.AssessmentID = &id
	s.job.Status = storeStatusDone
	s.job.Phase = 2
	s.deliveredAt = time.Now()
	return id, selected, nil
}

// ListCandidates devuelve copias de las candidatas, por trozo y luego por id.
func (s *memoryStore) ListCandidates(_ context.Context, jobID string) ([]m2m.CandidateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return nil, s.notFound("job", jobID)
	}
	out := make([]m2m.CandidateRecord, 0, len(s.candidates))
	for _, c := range s.candidates {
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ChunkSequence != out[j].ChunkSequence {
			return out[i].ChunkSequence < out[j].ChunkSequence
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func isTerminalCandidateStatus(status string) bool {
	return status == candStatusDroppedDup || status == candStatusDroppedIrrelevant || status == candStatusSelected
}

// UpdateCandidates aplica un lote de cambios parciales, entero o nada: si una
// candidata no existe (404) o una terminal cambiaría a otro estado (409), no se
// escribe ninguno.
func (s *memoryStore) UpdateCandidates(_ context.Context, updates []m2m.CandidateUpdate) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byID := make(map[string]*m2m.CandidateRecord, len(s.candidates))
	for _, c := range s.candidates {
		byID[c.ID] = c
	}
	for _, u := range updates {
		cand, ok := byID[u.ID]
		if !ok {
			return 0, s.notFound("candidata", u.ID)
		}
		if u.Status != nil && isTerminalCandidateStatus(cand.Status) && *u.Status != cand.Status {
			return 0, m2m.ErrPipelineConflict
		}
	}
	for _, u := range updates {
		cand := byID[u.ID]
		if u.Status != nil {
			cand.Status = *u.Status
		}
		if u.DedupeGroup != nil {
			group := *u.DedupeGroup
			cand.DedupeGroup = &group
		}
		if u.Score != nil {
			score := *u.Score
			cand.Score = &score
		}
		if len(u.Embedding) > 0 {
			cand.Embedding = u.Embedding
		}
	}
	return len(updates), nil
}

// GetJobIdeas devuelve la unión de las main_ideas de los trozos cerrados con
// artefactos, en orden de trozo y sin repetir.
func (s *memoryStore) GetJobIdeas(_ context.Context, jobID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != s.job.JobID {
		return nil, s.notFound("job", jobID)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, c := range s.chunks {
		if c.status != storeStatusDone || len(c.artifacts) == 0 {
			continue
		}
		var art materialpipeline.ChunkArtifactsV1
		if err := json.Unmarshal(c.artifacts, &art); err != nil {
			continue
		}
		for _, idea := range art.MainIdeas {
			idea = strings.TrimSpace(idea)
			if idea == "" {
				continue
			}
			if _, dup := seen[idea]; dup {
				continue
			}
			seen[idea] = struct{}{}
			out = append(out, idea)
		}
	}
	return out, nil
}

// ChunkText devuelve el texto de un trozo (lo usa el candado de la relevancia).
func (s *memoryStore) ChunkText(_ context.Context, chunkID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.chunkByID(chunkID)
	if c == nil {
		return "", s.notFound("chunk", chunkID)
	}
	return c.text, nil
}

// delivered dice si el job quedó done con borrador: el caso está completo.
func (s *memoryStore) delivered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.job.Status == storeStatusDone && s.job.AssessmentID != nil && *s.job.AssessmentID != ""
}

// seedCandidates deja el almacén con el job en processing, fase 1, y un trozo por
// candidata dada, cada una viva. Lo usa el modo de parejas, que entra directo a la
// pasada de duplicadas sin pasar por las fases 0 y 1.
func (s *memoryStore) seedCandidates(payloads ...json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.job.Status = storeStatusProcessing
	s.job.Phase = 1
	for i, p := range payloads {
		c := &memChunk{id: fmt.Sprintf("chunk-%03d", i), seq: i, status: storeStatusDone}
		s.chunks = append(s.chunks, c)
		s.insertCandidate(c, p)
	}
}
