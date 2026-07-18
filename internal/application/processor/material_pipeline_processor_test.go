package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// --- mocks del carril de materiales ---

type updateRecord struct {
	status    string
	phase     int16
	lastError *string
}

// mockMaterialPipeline implementa MaterialPipelineClient y registra el orden de las
// llamadas (opcionalmente en un log compartido con el provider para verificar el
// interleaving fase0/fase1).
type mockMaterialPipeline struct {
	log *[]string

	job         *m2m.PipelineJob
	getJobErr   error
	getJobCalls int

	pending    []*m2m.NextChunk
	nextIdx    int
	pendingErr error

	saveArtifactsErr error
	saveCalls        int
	savedSummaries   []string
	savedCandidates  [][]m2m.CandidatePayload

	updateErr   error
	updateCalls []updateRecord
}

func (m *mockMaterialPipeline) record(name string) {
	if m.log != nil {
		*m.log = append(*m.log, name)
	}
}

func (m *mockMaterialPipeline) GetJob(_ context.Context, _ string) (*m2m.PipelineJob, error) {
	m.record("GetJob")
	m.getJobCalls++
	if m.getJobErr != nil {
		return nil, m.getJobErr
	}
	return m.job, nil
}

func (m *mockMaterialPipeline) GetFileURL(_ context.Context, _ string) (*m2m.PresignedFile, error) {
	m.record("GetFileURL")
	return &m2m.PresignedFile{URL: "https://signed/pdf"}, nil
}

func (m *mockMaterialPipeline) SaveChunks(_ context.Context, _ string, _ []m2m.ChunkInput) error {
	m.record("SaveChunks")
	return nil
}

func (m *mockMaterialPipeline) GetNextPendingChunk(_ context.Context, _ string) (*m2m.NextChunk, error) {
	m.record("GetNextPendingChunk")
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	if m.nextIdx >= len(m.pending) {
		return nil, nil
	}
	c := m.pending[m.nextIdx]
	m.nextIdx++
	return c, nil
}

func (m *mockMaterialPipeline) SaveChunkArtifacts(_ context.Context, _ string, summary *string, _ json.RawMessage, candidates []m2m.CandidatePayload) error {
	m.record("SaveChunkArtifacts")
	m.saveCalls++
	m.savedSummaries = append(m.savedSummaries, deref(summary))
	m.savedCandidates = append(m.savedCandidates, candidates)
	return m.saveArtifactsErr
}

func (m *mockMaterialPipeline) UpdateJobStatus(_ context.Context, _, status string, phase int16, lastError *string) error {
	m.record("UpdateJobStatus:" + status)
	m.updateCalls = append(m.updateCalls, updateRecord{status: status, phase: phase, lastError: lastError})
	return m.updateErr
}

// mockMaterialProvider implementa MaterialLLMProvider con salidas fijas.
type mockMaterialProvider struct {
	log        *[]string
	digest     *llm.DigestChunkResult
	digestErr  error
	candidates []materialpipeline.CandidatePayloadV1
	proposeErr error
}

func (m *mockMaterialProvider) DigestChunk(_ context.Context, _ llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	if m.log != nil {
		*m.log = append(*m.log, "DigestChunk")
	}
	if m.digestErr != nil {
		return nil, m.digestErr
	}
	return m.digest, nil
}

func (m *mockMaterialProvider) ProposeCandidates(_ context.Context, _ llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	if m.log != nil {
		*m.log = append(*m.log, "ProposeCandidates")
	}
	if m.proposeErr != nil {
		return nil, m.proposeErr
	}
	return m.candidates, nil
}

func (m *mockMaterialProvider) Name() string { return "mock-local" }

// --- helpers ---

func newMaterialProcessor(settings SchoolSettingsReader, pipe MaterialPipelineClient, prov MaterialLLMProvider) *MaterialPipelineProcessor {
	// El extractor y la descarga NO deben usarse en estos tests (el job entra ya
	// porcionado → la fase 0 se salta sola). Si alguno se invoca, falla claro.
	ex := &mockPhase0Extractor{err: errors.New("extractor no debe invocarse en estos tests")}
	dl := func(_ context.Context, _ string, _ int64) ([]byte, error) {
		return nil, errors.New("descarga no debe invocarse en estos tests")
	}
	return NewMaterialPipelineProcessor(settings, pipe, prov, ex, dl, chunking.DefaultConfig(), 1024, newTestLogger())
}

func materialEventJSON(jobID, materialID, schoolID string) []byte {
	evt := events.MaterialAssessmentRequestedEvent{
		EventType: events.EventTypeMaterialAssessmentRequested,
		Payload: events.MaterialAssessmentRequestedPayload{
			JobID:      jobID,
			MaterialID: materialID,
			SchoolID:   schoolID,
		},
	}
	b, _ := json.Marshal(evt)
	return b
}

// processingJob devuelve un job en `processing` YA porcionado (chunk_counts pending>0)
// para que la fase 0 se salte sola (reanudación) y el test se enfoque en la fase 1.
func processingJob() *m2m.PipelineJob {
	return &m2m.PipelineJob{JobID: "job-1", Status: jobStatusProcessing, ChunkCounts: map[string]int{"pending": 1}}
}

func pendingChunk(id string) *m2m.NextChunk {
	return &m2m.NextChunk{ChunkID: id, JobID: "job-1", Seq: 0, ChunkText: "texto del trozo", Status: "pending"}
}

func validDigest() *llm.DigestChunkResult {
	return &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{
			Version:    1,
			MainIdeas:  []string{"La fotosíntesis convierte luz en energía química."},
			ChunkTopic: "fotosíntesis",
		},
		Summary: "La fotosíntesis convierte luz solar en energía química en las plantas.",
	}
}

func validCandidate() materialpipeline.CandidatePayloadV1 {
	return materialpipeline.CandidatePayloadV1{
		Version:       1,
		QuestionType:  "true_false",
		QuestionText:  "¿La fotosíntesis usa luz solar?",
		CorrectAnswer: json.RawMessage(`"true"`),
	}
}

func invalidCandidate() materialpipeline.CandidatePayloadV1 {
	return materialpipeline.CandidatePayloadV1{
		Version:      1,
		QuestionType: "tipo_inexistente",
		QuestionText: "pregunta con tipo desconocido",
	}
}

func onSettings() *mockSettingsReader {
	return &mockSettingsReader{settings: settingsWith(settingKeyPipelineMode, pipelineModeOn)}
}

// --- casos ---

func TestMaterialProcess_PolicyOff_Acks(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyPipelineMode, pipelineModeOff)}
	pipe := &mockMaterialPipeline{job: processingJob()}
	prov := &mockMaterialProvider{}

	if err := newMaterialProcessor(settings, pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("política off debería ACKear sin error, got %v", err)
	}
	if pipe.getJobCalls != 0 {
		t.Fatalf("política off NO debe tocar el pipeline; GetJob se llamó %d veces", pipe.getJobCalls)
	}
}

func TestMaterialProcess_PolicyAbsent_Acks(t *testing.T) {
	// Settings sin la llave: settingValueOr cae al default off.
	settings := &mockSettingsReader{settings: m2m.SchoolSettings{SchoolID: "school-1"}}
	pipe := &mockMaterialPipeline{job: processingJob()}

	if err := newMaterialProcessor(settings, pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("política ausente debería ACKear, got %v", err)
	}
	if pipe.getJobCalls != 0 {
		t.Fatalf("política ausente NO debe tocar el pipeline")
	}
}

func TestMaterialProcess_MalformedEvent_Permanent(t *testing.T) {
	settings := onSettings()
	pipe := &mockMaterialPipeline{job: processingJob()}

	// job_id vacío → evento inválido.
	err := newMaterialProcessor(settings, pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("", "mat-1", "school-1"))
	if err == nil {
		t.Fatal("evento sin job_id debería fallar")
	}
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("error debería envolver ErrMalformedEvent, got %v", err)
	}
	if classifyError(err) != ErrorTypePermanent {
		t.Fatalf("evento malformado debería ser permanente")
	}
	if settings.calls != 0 || pipe.getJobCalls != 0 {
		t.Fatalf("evento malformado no debe leer settings ni pipeline")
	}
}

func TestMaterialProcess_DecodeError_Permanent(t *testing.T) {
	err := newMaterialProcessor(onSettings(), &mockMaterialPipeline{}, &mockMaterialProvider{}).Process(context.Background(), []byte("{no es json"))
	if err == nil || classifyError(err) != ErrorTypePermanent {
		t.Fatalf("payload indecodificable debería ser permanente, got %v", err)
	}
}

func TestMaterialProcess_FullFlow_Order(t *testing.T) {
	var seq []string
	pipe := &mockMaterialPipeline{log: &seq, job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{log: &seq, digest: validDigest(), candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("flujo completo devolvió error: %v", err)
	}

	// Orden esperado: el processor GET job (guarda 404/terminal), la fase 0 GET job de
	// nuevo (reanudación → no-op), y luego el loop de fase 1.
	want := []string{"GetJob", "GetJob", "GetNextPendingChunk", "DigestChunk", "ProposeCandidates", "SaveChunkArtifacts", "GetNextPendingChunk", "UpdateJobStatus:done"}
	if len(seq) != len(want) {
		t.Fatalf("secuencia = %v, se esperaba %v", seq, want)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("secuencia[%d] = %q, se esperaba %q (full: %v)", i, seq[i], want[i], seq)
		}
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 1 {
		t.Fatalf("se esperaba 1 PUT con 1 candidata, got saveCalls=%d cand=%v", pipe.saveCalls, pipe.savedCandidates)
	}
	if pipe.savedSummaries[0] != validDigest().Summary {
		t.Fatalf("summary persistido = %q, se esperaba %q", pipe.savedSummaries[0], validDigest().Summary)
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone || last.phase != 1 || last.lastError != nil {
		t.Fatalf("cierre esperado done/phase1/nil, got %+v", last)
	}
}

func TestMaterialProcess_ArtifactsNotValidable_Transient(t *testing.T) {
	var seq []string
	badDigest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: nil, ChunkTopic: ""}, // sin ideas ni tema
		Summary:   "resumen",
	}
	pipe := &mockMaterialPipeline{log: &seq, job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{log: &seq, digest: badDigest, candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || !errors.Is(err, ErrInvalidChunkArtifacts) {
		t.Fatalf("artefactos inválidos deberían fallar con ErrInvalidChunkArtifacts, got %v", err)
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("artefactos inválidos deben ser transitorios (nunca DLQ), got permanente")
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("no debe persistirse nada con artefactos inválidos")
	}
	for _, c := range seq {
		if c == "ProposeCandidates" {
			t.Fatalf("B no debe llamarse si A no valida; seq=%v", seq)
		}
		if c == "UpdateJobStatus:failed" {
			t.Fatalf("un transitorio NO debe marcar el job failed")
		}
	}
}

func TestMaterialProcess_SummaryTooLong_Transient(t *testing.T) {
	long := ""
	for i := 0; i < 130; i++ {
		long += "palabra "
	}
	digest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: []string{"idea"}, ChunkTopic: "tema"},
		Summary:   long,
	}
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{digest: digest, candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || !errors.Is(err, ErrInvalidChunkArtifacts) || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("summary >120 palabras debería ser transitorio ErrInvalidChunkArtifacts, got %v", err)
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("no debe persistirse con summary inválido")
	}
}

func TestMaterialProcess_CandidateFiltering(t *testing.T) {
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{
		digest:     validDigest(),
		candidates: []materialpipeline.CandidatePayloadV1{validCandidate(), invalidCandidate(), validCandidate()},
	}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("filtrado devolvió error: %v", err)
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 2 {
		t.Fatalf("se esperaba 1 PUT con 2 candidatas válidas (1 descartada de 3), got saveCalls=%d cand=%d", pipe.saveCalls, len(pipe.savedCandidates[0]))
	}
}

func TestMaterialProcess_ZeroValidCandidates_Transient(t *testing.T) {
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{
		digest:     validDigest(),
		candidates: []materialpipeline.CandidatePayloadV1{invalidCandidate(), invalidCandidate()},
	}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || !errors.Is(err, ErrNoValidCandidates) || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("cero candidatas válidas debería ser transitorio ErrNoValidCandidates, got %v", err)
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("no debe persistirse sin candidatas válidas")
	}
}

func TestMaterialProcess_SaveConflict_Continues(t *testing.T) {
	pipe := &mockMaterialPipeline{
		job:              processingJob(),
		pending:          []*m2m.NextChunk{pendingChunk("c1")},
		saveArtifactsErr: m2m.ErrPipelineConflict,
	}
	prov := &mockMaterialProvider{digest: validDigest(), candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("un 409 en el PUT NO es fallo: debe continuar y cerrar, got %v", err)
	}
	if pipe.saveCalls != 1 {
		t.Fatalf("se esperaba 1 intento de PUT")
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("tras el 409 debe continuar al siguiente y cerrar done, got %+v", last)
	}
}

func TestMaterialProcess_EndOfPending_Done(t *testing.T) {
	pipe := &mockMaterialPipeline{job: processingJob()} // sin pendientes
	if err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("sin pendientes debería cerrar done sin error, got %v", err)
	}
	if len(pipe.updateCalls) != 1 || pipe.updateCalls[0].status != jobStatusDone || pipe.updateCalls[0].phase != 1 {
		t.Fatalf("se esperaba un único PATCH done/phase1, got %+v", pipe.updateCalls)
	}
}

func TestMaterialProcess_PermanentError_MarksFailedBestEffort(t *testing.T) {
	pipe := &mockMaterialPipeline{
		job:        processingJob(),
		pendingErr: m2m.ErrLearningPermanent, // 4xx permanente al pedir el siguiente chunk
	}
	err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypePermanent {
		t.Fatalf("un 4xx permanente debería subir como permanente, got %v", err)
	}
	var failed *updateRecord
	for i := range pipe.updateCalls {
		if pipe.updateCalls[i].status == jobStatusFailed {
			failed = &pipe.updateCalls[i]
		}
	}
	if failed == nil || failed.phase != 1 || failed.lastError == nil {
		t.Fatalf("un permanente debe marcar el job failed (best-effort) con fase y last_error, got %+v", pipe.updateCalls)
	}
}

func TestMaterialProcess_SettingsError_Transient(t *testing.T) {
	settings := &mockSettingsReader{err: errors.New("academic caído")}
	pipe := &mockMaterialPipeline{job: processingJob()}
	err := newMaterialProcessor(settings, pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un fallo de settings debería ser transitorio, got %v", err)
	}
	if pipe.getJobCalls != 0 {
		t.Fatalf("no debe tocar el pipeline si settings falla")
	}
}

func TestMaterialProcess_TerminalJob_Acks(t *testing.T) {
	pipe := &mockMaterialPipeline{job: &m2m.PipelineJob{JobID: "job-1", Status: jobStatusDone, ChunkCounts: map[string]int{}}}
	if err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("job terminal debería ACKear, got %v", err)
	}
	if pipe.nextIdx != 0 || len(pipe.updateCalls) != 0 {
		t.Fatalf("job terminal no debe entrar al loop ni actualizar estado")
	}
}

func TestMaterialProcess_JobDeleted_Permanent(t *testing.T) {
	pipe := &mockMaterialPipeline{getJobErr: m2m.ErrLearningPermanent}
	err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypePermanent {
		t.Fatalf("job borrado (404) debería ser permanente, got %v", err)
	}
	if len(pipe.updateCalls) != 0 {
		t.Fatalf("no hay job que marcar failed si GET job dio 404")
	}
}
