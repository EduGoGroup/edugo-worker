package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
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

	markFailedErr     error
	markFailedChunks  []string
	markFailedReasons []string

	updateErr   error
	updateCalls []updateRecord

	deliverAssessmentID string
	deliverQuestions    int
	deliverErr          error
	deliverCalls        int
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

func (m *mockMaterialPipeline) MarkChunkFailed(_ context.Context, chunkID, reason string) error {
	m.record("MarkChunkFailed")
	m.markFailedChunks = append(m.markFailedChunks, chunkID)
	m.markFailedReasons = append(m.markFailedReasons, reason)
	return m.markFailedErr
}

func (m *mockMaterialPipeline) UpdateJobStatus(_ context.Context, _, status string, phase int16, lastError *string) error {
	m.record("UpdateJobStatus:" + status)
	m.updateCalls = append(m.updateCalls, updateRecord{status: status, phase: phase, lastError: lastError})
	return m.updateErr
}

func (m *mockMaterialPipeline) DeliverJob(_ context.Context, _ string) (string, int, error) {
	m.record("DeliverJob")
	m.deliverCalls++
	if m.deliverErr != nil {
		return "", 0, m.deliverErr
	}
	return m.deliverAssessmentID, m.deliverQuestions, nil
}

// --- fakes de las pasadas del reduce (fase 2) ---
//
// Cada fake registra su nº de llamadas y opcionalmente devuelve un error, para probar el
// encadenamiento y la clasificación sin ejercer embeddings/LLM/learning reales. Los
// reportes van en cero (el processor solo los loguea).

type fakeDedupe struct {
	calls int
	err   error
}

func (f *fakeDedupe) Run(_ context.Context, _ string) (reduce.DedupeReport, error) {
	f.calls++
	return reduce.DedupeReport{}, f.err
}

type fakeRelevance struct {
	calls int
	err   error
}

func (f *fakeRelevance) Run(_ context.Context, _ string) (reduce.RelevanceReport, error) {
	f.calls++
	return reduce.RelevanceReport{}, f.err
}

type fakeQuality struct {
	calls int
	err   error
}

func (f *fakeQuality) Run(_ context.Context, _ string) (reduce.QualityReport, error) {
	f.calls++
	return reduce.QualityReport{}, f.err
}

type fakeSelection struct {
	calls     int
	gotTarget int
	err       error
}

func (f *fakeSelection) Run(_ context.Context, _ string, targetQuestions int) (reduce.SelectionReport, error) {
	f.calls++
	f.gotTarget = targetQuestions
	return reduce.SelectionReport{}, f.err
}

// digestOutcome es la salida de UNA llamada a DigestChunk (para probar el reintento por
// calidad: distinta respuesta por número de llamada).
type digestOutcome struct {
	result *llm.DigestChunkResult
	err    error
}

// proposeOutcome es la salida de UNA llamada a ProposeCandidates (para probar el reintento
// por calidad de la fase B).
type proposeOutcome struct {
	candidates []materialpipeline.CandidatePayloadV1
	err        error
}

// mockMaterialProvider implementa MaterialLLMProvider con salidas fijas. Si
// digestOutcomes no está vacío, define la salida por número de llamada (tiene prioridad
// sobre digest/digestErr); si se agotan, repite la última. Registra la temperatura
// recibida en cada llamada (digestTemps) para verificar el jitter del reintento.
type mockMaterialProvider struct {
	log            *[]string
	digest         *llm.DigestChunkResult
	digestErr      error
	digestOutcomes []digestOutcome
	digestCalls    int
	digestTemps    []*float64

	candidates      []materialpipeline.CandidatePayloadV1
	proposeErr      error
	proposeOutcomes []proposeOutcome
	proposeCalls    int
	proposeTemps    []*float64
}

func (m *mockMaterialProvider) DigestChunk(_ context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	if m.log != nil {
		*m.log = append(*m.log, "DigestChunk")
	}
	m.digestTemps = append(m.digestTemps, in.Temperature)
	idx := m.digestCalls
	m.digestCalls++
	if len(m.digestOutcomes) > 0 {
		if idx >= len(m.digestOutcomes) {
			idx = len(m.digestOutcomes) - 1
		}
		o := m.digestOutcomes[idx]
		return o.result, o.err
	}
	if m.digestErr != nil {
		return nil, m.digestErr
	}
	return m.digest, nil
}

func (m *mockMaterialProvider) ProposeCandidates(_ context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	if m.log != nil {
		*m.log = append(*m.log, "ProposeCandidates")
	}
	m.proposeTemps = append(m.proposeTemps, in.Temperature)
	idx := m.proposeCalls
	m.proposeCalls++
	if len(m.proposeOutcomes) > 0 {
		if idx >= len(m.proposeOutcomes) {
			idx = len(m.proposeOutcomes) - 1
		}
		o := m.proposeOutcomes[idx]
		return o.candidates, o.err
	}
	if m.proposeErr != nil {
		return nil, m.proposeErr
	}
	return m.candidates, nil
}

func (m *mockMaterialProvider) Name() string { return "mock-local" }

// --- helpers ---

// testTargetQuestions es el cupo por defecto que newMaterialProcessor inyecta en la
// selección (fase 2) para los tests que no lo controlan.
const testTargetQuestions = 20

// defaultReduceDeps arma unas dependencias del reduce con pasadas fake que SIEMPRE
// suceden (encadenamiento feliz por defecto). Los tests que quieren inspeccionar o
// forzar un error de una pasada construyen las suyas con newMaterialProcessorWithReduce.
func defaultReduceDeps() ReduceDeps {
	return ReduceDeps{
		Dedupe:                 &fakeDedupe{},
		Relevance:              &fakeRelevance{},
		Quality:                &fakeQuality{},
		Selection:              &fakeSelection{},
		TargetQuestionsDefault: testTargetQuestions,
	}
}

func newMaterialProcessor(settings SchoolSettingsReader, pipe MaterialPipelineClient, prov MaterialLLMProvider) *MaterialPipelineProcessor {
	return newMaterialProcessorWithReduce(settings, pipe, prov, defaultReduceDeps())
}

func newMaterialProcessorWithReduce(settings SchoolSettingsReader, pipe MaterialPipelineClient, prov MaterialLLMProvider, reduceDeps ReduceDeps) *MaterialPipelineProcessor {
	// El extractor y la descarga NO deben usarse en estos tests (el job entra ya
	// porcionado → la fase 0 se salta sola). Si alguno se invoca, falla claro.
	ex := &mockPhase0Extractor{err: errors.New("extractor no debe invocarse en estos tests")}
	dl := func(_ context.Context, _ string, _ int64) ([]byte, error) {
		return nil, errors.New("descarga no debe invocarse en estos tests")
	}
	return NewMaterialPipelineProcessor(settings, pipe, prov, ex, dl, chunking.DefaultConfig(), 1024, reduceDeps, newTestLogger())
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
	// nuevo (reanudación → no-op), el loop de fase 1 y, encadenada, la entrega de la fase 2
	// (las pasadas del reduce son fakes que no registran; solo el DeliverJob del cliente).
	want := []string{"GetJob", "GetJob", "GetNextPendingChunk", "DigestChunk", "ProposeCandidates", "SaveChunkArtifacts", "GetNextPendingChunk", "UpdateJobStatus:done", "DeliverJob"}
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

func TestMaterialProcess_ArtifactsNotValidable_IsolatesChunk(t *testing.T) {
	var seq []string
	badDigest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: nil, ChunkTopic: ""}, // sin ideas ni tema
		Summary:   "resumen",
	}
	pipe := &mockMaterialPipeline{log: &seq, job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{log: &seq, digest: badDigest, candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}}

	// Artefactos inválidos en ambos intentos → reintento por calidad, luego aislamiento
	// del chunk y cierre del job. NO propaga error (no tumba el evento).
	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("un chunk que no valida debe aislarse y continuar sin error, got %v", err)
	}
	if prov.digestCalls != 1+llmQualityRetries {
		t.Fatalf("se esperaban %d intentos de digest (1 + reintentos), got %d", 1+llmQualityRetries, prov.digestCalls)
	}
	if len(pipe.markFailedChunks) != 1 || pipe.markFailedChunks[0] != "c1" {
		t.Fatalf("el chunk envenenado debía aislarse (MarkChunkFailed c1), got %v", pipe.markFailedChunks)
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("no debe persistirse nada con artefactos inválidos")
	}
	for _, c := range seq {
		if c == "ProposeCandidates" {
			t.Fatalf("B no debe llamarse si A no valida; seq=%v", seq)
		}
		if c == "UpdateJobStatus:failed" {
			t.Fatalf("aislar un chunk NO debe marcar el JOB failed; seq=%v", seq)
		}
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("tras aislar el chunk el job debe cerrar done, got %+v", last)
	}
}

func TestMaterialProcess_SummaryTooLong_IsolatesChunk(t *testing.T) {
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

	// Summary inválido (>120 palabras) es fallo de CALIDAD: reintenta y aísla, sin error.
	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("summary inválido debe aislar el chunk y continuar sin error, got %v", err)
	}
	if len(pipe.markFailedChunks) != 1 {
		t.Fatalf("summary inválido en ambos intentos debía aislar el chunk, got %v", pipe.markFailedChunks)
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("no debe persistirse con summary inválido")
	}
}

func TestMaterialProcess_DigestQualityRetry_Succeeds(t *testing.T) {
	var seq []string
	// Primer intento: fallo de CALIDAD (llm.ErrLLMQuality). Segundo intento: digest válido.
	qualityErr := fmt.Errorf("%w: JSON sin cierre", llm.ErrLLMQuality)
	prov := &mockMaterialProvider{
		log: &seq,
		digestOutcomes: []digestOutcome{
			{result: nil, err: qualityErr},
			{result: validDigest(), err: nil},
		},
		candidates: []materialpipeline.CandidatePayloadV1{validCandidate()},
	}
	pipe := &mockMaterialPipeline{log: &seq, job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("el reintento por calidad debía rescatar el chunk, got %v", err)
	}
	if prov.digestCalls != 2 {
		t.Fatalf("se esperaban 2 llamadas a DigestChunk (1 + 1 reintento), got %d", prov.digestCalls)
	}
	// El primer intento va con la temperatura por defecto (nil); el reintento con jitter.
	if prov.digestTemps[0] != nil {
		t.Fatalf("el primer intento no debe forzar temperatura, got %v", *prov.digestTemps[0])
	}
	if prov.digestTemps[1] == nil || *prov.digestTemps[1] != llmRetryTemperature {
		t.Fatalf("el reintento debe usar temperatura %v (jitter), got %v", llmRetryTemperature, prov.digestTemps[1])
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("un reintento exitoso NO debe aislar el chunk, got %v", pipe.markFailedChunks)
	}
	if pipe.saveCalls != 1 {
		t.Fatalf("tras el reintento exitoso debe persistirse el chunk, got saveCalls=%d", pipe.saveCalls)
	}
}

func TestMaterialProcess_DigestInfraError_PropagatesNoRetry(t *testing.T) {
	// Un fallo de INFRA (sin sentinel de calidad) NO se reintenta ni aísla: sube como
	// transitorio para que el reintento del evento / DLQ operen igual que hoy.
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{digestErr: errors.New("ollama request failed: connection refused")}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un fallo de infra del digest debe ser transitorio, got %v", err)
	}
	if prov.digestCalls != 1 {
		t.Fatalf("un fallo de infra NO debe reintentarse in-processor, got %d llamadas", prov.digestCalls)
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("un fallo de infra NO debe aislar el chunk, got %v", pipe.markFailedChunks)
	}
}

func TestMaterialProcess_PoisonedChunk_IsolatesAndContinuesNext(t *testing.T) {
	// Dos chunks: el primero se envenena por calidad en ambos intentos; el segundo es
	// válido. El primero se aísla y el segundo se procesa; el job cierra done.
	badDigest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: nil, ChunkTopic: ""},
		Summary:   "resumen",
	}
	prov := &mockMaterialProvider{
		digestOutcomes: []digestOutcome{
			{result: badDigest, err: nil},     // c1 intento 1
			{result: badDigest, err: nil},     // c1 intento 2 (reintento)
			{result: validDigest(), err: nil}, // c2 intento 1
		},
		candidates: []materialpipeline.CandidatePayloadV1{validCandidate()},
	}
	pipe := &mockMaterialPipeline{
		job:     processingJob(),
		pending: []*m2m.NextChunk{pendingChunk("c1"), {ChunkID: "c2", JobID: "job-1", Seq: 1, ChunkText: "otro trozo", Status: "pending"}},
	}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("el chunk envenenado debe aislarse y el siguiente procesarse, got %v", err)
	}
	if len(pipe.markFailedChunks) != 1 || pipe.markFailedChunks[0] != "c1" {
		t.Fatalf("solo c1 debía aislarse, got %v", pipe.markFailedChunks)
	}
	if pipe.saveCalls != 1 {
		t.Fatalf("c2 debía persistirse (1 PUT), got saveCalls=%d", pipe.saveCalls)
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("el job debe cerrar done tras aislar c1 y procesar c2, got %+v", last)
	}
}

func TestMaterialProcess_IsolateInfraFails_Propagates(t *testing.T) {
	// Si el propio MarkChunkFailed falla por INFRA, se propaga como transitorio (no se
	// traga un error de infraestructura al aislar).
	badDigest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: nil, ChunkTopic: ""},
		Summary:   "resumen",
	}
	pipe := &mockMaterialPipeline{
		job:           processingJob(),
		pending:       []*m2m.NextChunk{pendingChunk("c1")},
		markFailedErr: errors.New("learning 503"),
	}
	prov := &mockMaterialProvider{digest: badDigest}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un fallo de infra al aislar debe subir como transitorio, got %v", err)
	}
}

func TestMaterialProcess_IsolateConflict_Continues(t *testing.T) {
	// Si al aislar el chunk ya estaba done (409), es una carrera benigna: se continúa y
	// el job cierra sin error.
	badDigest := &llm.DigestChunkResult{
		Artifacts: materialpipeline.ChunkArtifactsV1{Version: 1, MainIdeas: nil, ChunkTopic: ""},
		Summary:   "resumen",
	}
	pipe := &mockMaterialPipeline{
		job:           processingJob(),
		pending:       []*m2m.NextChunk{pendingChunk("c1")},
		markFailedErr: m2m.ErrPipelineConflict,
	}
	prov := &mockMaterialProvider{digest: badDigest}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("un 409 al aislar es benigno: debe continuar sin error, got %v", err)
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("tras el 409 al aislar el job debe cerrar done, got %+v", last)
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

func TestMaterialProcess_ZeroValidCandidates_PersistsAndContinues(t *testing.T) {
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
	prov := &mockMaterialProvider{
		digest:     validDigest(),
		candidates: []materialpipeline.CandidatePayloadV1{invalidCandidate(), invalidCandidate()},
	}

	// Cero candidatas válidas NO es fatal: los artefactos SÍ valen, se persisten sin
	// candidatas (el chunk queda done, la cadena de summary no se rompe) y el job cierra.
	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("cero candidatas válidas no debe tumbar el evento, got %v", err)
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 0 {
		t.Fatalf("se esperaba 1 PUT con 0 candidatas (artefactos igual persistidos), got saveCalls=%d cand=%v", pipe.saveCalls, pipe.savedCandidates)
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("cero candidatas NO debe marcar el chunk failed (sus artefactos son válidos), got %v", pipe.markFailedChunks)
	}
	if pipe.savedSummaries[0] != validDigest().Summary {
		t.Fatalf("el summary debe persistirse para encadenar, got %q", pipe.savedSummaries[0])
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("el job debe cerrar done, got %+v", last)
	}
}

func TestMaterialProcess_ProposeQualityRetry_Succeeds(t *testing.T) {
	// Fase B: primer intento falla por CALIDAD (JSON sin cierre → llm.ErrLLMQuality); el
	// reintento con jitter de temperatura devuelve una candidata válida.
	qualityErr := fmt.Errorf("%w: objeto JSON sin cierre balanceado", llm.ErrLLMQuality)
	prov := &mockMaterialProvider{
		digest: validDigest(),
		proposeOutcomes: []proposeOutcome{
			{candidates: nil, err: qualityErr},
			{candidates: []materialpipeline.CandidatePayloadV1{validCandidate()}, err: nil},
		},
	}
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("el reintento por calidad de la fase B debía rescatar el chunk, got %v", err)
	}
	if prov.proposeCalls != 2 {
		t.Fatalf("se esperaban 2 llamadas a ProposeCandidates (1 + 1 reintento), got %d", prov.proposeCalls)
	}
	if prov.proposeTemps[0] != nil {
		t.Fatalf("el primer intento no debe forzar temperatura, got %v", *prov.proposeTemps[0])
	}
	if prov.proposeTemps[1] == nil || *prov.proposeTemps[1] != llmRetryTemperature {
		t.Fatalf("el reintento debe usar temperatura %v (jitter), got %v", llmRetryTemperature, prov.proposeTemps[1])
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 1 {
		t.Fatalf("tras el reintento exitoso debe persistirse 1 candidata, got saveCalls=%d cand=%v", pipe.saveCalls, pipe.savedCandidates)
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("un reintento exitoso NO debe aislar el chunk, got %v", pipe.markFailedChunks)
	}
}

func TestMaterialProcess_ProposeQualityPersistent_PersistsEmptyAndContinues(t *testing.T) {
	// Fase B: la propuesta muere por CALIDAD en ambos intentos. El digest ES válido, así
	// que se persisten artefactos sin candidatas (chunk done, cadena de summary intacta),
	// NO se marca failed y el job cierra. Sin error (no tumba el evento → no DLQ).
	qualityErr := fmt.Errorf("%w: objeto JSON sin cierre balanceado", llm.ErrLLMQuality)
	prov := &mockMaterialProvider{
		digest:          validDigest(),
		proposeOutcomes: []proposeOutcome{{candidates: nil, err: qualityErr}}, // se repite en el retry
	}
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("una propuesta sin calidad persistente NO debe tumbar el evento, got %v", err)
	}
	if prov.proposeCalls != 1+llmQualityRetries {
		t.Fatalf("se esperaban %d intentos de propuesta, got %d", 1+llmQualityRetries, prov.proposeCalls)
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 0 {
		t.Fatalf("se esperaba 1 PUT con 0 candidatas (artefactos igual persistidos), got saveCalls=%d cand=%v", pipe.saveCalls, pipe.savedCandidates)
	}
	if pipe.savedSummaries[0] != validDigest().Summary {
		t.Fatalf("el summary debe persistirse para encadenar, got %q", pipe.savedSummaries[0])
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("una propuesta sin calidad NO debe marcar el chunk failed (los artefactos valen), got %v", pipe.markFailedChunks)
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusDone {
		t.Fatalf("el job debe cerrar done, got %+v", last)
	}
}

func TestMaterialProcess_ProposeInfraError_PropagatesNoRetry(t *testing.T) {
	// Un fallo de INFRA en la fase B (sin sentinel de calidad) NO se reintenta ni persiste
	// vacío: sube como transitorio para que el reintento del evento / DLQ operen igual.
	prov := &mockMaterialProvider{
		digest:     validDigest(),
		proposeErr: errors.New("ollama request failed: connection refused"),
	}
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}

	err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un fallo de infra de la fase B debe ser transitorio, got %v", err)
	}
	if prov.proposeCalls != 1 {
		t.Fatalf("un fallo de infra NO debe reintentarse in-processor, got %d llamadas", prov.proposeCalls)
	}
	if pipe.saveCalls != 0 {
		t.Fatalf("un fallo de infra NO debe persistir nada, got saveCalls=%d", pipe.saveCalls)
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("un fallo de infra NO debe aislar el chunk, got %v", pipe.markFailedChunks)
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

func TestMaterialProcess_FailedJob_Acks(t *testing.T) {
	pipe := &mockMaterialPipeline{job: &m2m.PipelineJob{JobID: "job-1", Status: jobStatusFailed, ChunkCounts: map[string]int{}}}
	if err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("job failed debería ACKear, got %v", err)
	}
	if pipe.nextIdx != 0 || len(pipe.updateCalls) != 0 || pipe.deliverCalls != 0 {
		t.Fatalf("job failed no debe entrar al loop, actualizar estado ni entregar")
	}
}

func TestMaterialProcess_DonePhase2_NoOpAcks(t *testing.T) {
	// Un job ya entregado (done + assessment_id) es fase 2 completa: no-op benigno. No
	// corre el reduce ni re-entrega.
	assessment := "assess-1"
	pipe := &mockMaterialPipeline{job: &m2m.PipelineJob{
		JobID: "job-1", Status: jobStatusDone, Phase: 2,
		AssessmentID: &assessment, ChunkCounts: map[string]int{},
	}}
	dedupe, relevance, quality, selection := &fakeDedupe{}, &fakeRelevance{}, &fakeQuality{}, &fakeSelection{}
	deps := ReduceDeps{Dedupe: dedupe, Relevance: relevance, Quality: quality, Selection: selection, TargetQuestionsDefault: testTargetQuestions}

	if err := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("job entregado debería ACKear, got %v", err)
	}
	if pipe.nextIdx != 0 || len(pipe.updateCalls) != 0 || pipe.deliverCalls != 0 {
		t.Fatalf("job ya entregado no debe entrar al loop, actualizar estado ni re-entregar")
	}
	if dedupe.calls != 0 || relevance.calls != 0 || quality.calls != 0 || selection.calls != 0 {
		t.Fatalf("job ya entregado no debe correr ninguna pasada del reduce")
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

// --- fase 2 (reduce): encadenamiento, reanudación y entrega ---

func TestMaterialProcess_Phase1ChainsPhase2(t *testing.T) {
	// La fase 1 completa (job processing sin pendientes → cierra done/phase1) encadena la
	// fase 2: las cuatro pasadas corren en orden y luego la entrega. El cupo de la selección
	// es el default del riel (el M2M no expone target_questions).
	pipe := &mockMaterialPipeline{job: processingJob(), deliverAssessmentID: "assess-9", deliverQuestions: 7}
	pipe.nextIdx, pipe.pending = 0, nil // sin pendientes: fase 1 cierra de una

	dedupe, relevance, quality, selection := &fakeDedupe{}, &fakeRelevance{}, &fakeQuality{}, &fakeSelection{}
	deps := ReduceDeps{Dedupe: dedupe, Relevance: relevance, Quality: quality, Selection: selection, TargetQuestionsDefault: testTargetQuestions}

	if err := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("el encadenamiento fase1→fase2 no debe fallar, got %v", err)
	}
	if dedupe.calls != 1 || relevance.calls != 1 || quality.calls != 1 || selection.calls != 1 {
		t.Fatalf("las cuatro pasadas deben correr una vez, got dedupe=%d rel=%d qual=%d sel=%d",
			dedupe.calls, relevance.calls, quality.calls, selection.calls)
	}
	if selection.gotTarget != testTargetQuestions {
		t.Fatalf("la selección debe recibir el cupo default %d, got %d", testTargetQuestions, selection.gotTarget)
	}
	if pipe.deliverCalls != 1 {
		t.Fatalf("la entrega M2M debe llamarse una vez, got %d", pipe.deliverCalls)
	}
	// La fase 1 cerró done/phase1; la fase 2 NO vuelve a tocar el status (deliver lo cierra
	// server-side). Debe haber exactamente un PATCH done/phase1.
	if len(pipe.updateCalls) != 1 || pipe.updateCalls[0].status != jobStatusDone || pipe.updateCalls[0].phase != 1 {
		t.Fatalf("la fase 2 no debe PATCHear el status del job, got %+v", pipe.updateCalls)
	}
}

func TestMaterialProcess_RetriggerDonePhase1_RunsOnlyPhase2(t *testing.T) {
	// Re-disparo del evento sobre un job ya done de fase 1 (sin assessment_id): se reanuda
	// DIRECTO en la fase 2, sin repetir fase 0/1 (no se pide un chunk pendiente ni se PATCHea
	// el status; deliver lo cierra server-side).
	pipe := &mockMaterialPipeline{
		job:                 &m2m.PipelineJob{JobID: "job-1", Status: jobStatusDone, Phase: 1, ChunkCounts: map[string]int{"done": 3}},
		deliverAssessmentID: "assess-2", deliverQuestions: 5,
	}
	dedupe, relevance, quality, selection := &fakeDedupe{}, &fakeRelevance{}, &fakeQuality{}, &fakeSelection{}
	deps := ReduceDeps{Dedupe: dedupe, Relevance: relevance, Quality: quality, Selection: selection, TargetQuestionsDefault: testTargetQuestions}

	if err := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("el re-disparo sobre done/fase1 debe correr solo la fase 2, got %v", err)
	}
	if dedupe.calls != 1 || relevance.calls != 1 || quality.calls != 1 || selection.calls != 1 {
		t.Fatalf("la fase 2 debe correr completa, got dedupe=%d rel=%d qual=%d sel=%d",
			dedupe.calls, relevance.calls, quality.calls, selection.calls)
	}
	if pipe.deliverCalls != 1 {
		t.Fatalf("la entrega debe llamarse una vez, got %d", pipe.deliverCalls)
	}
	if pipe.nextIdx != 0 {
		t.Fatalf("no debe repetirse la fase 1 (GetNextPendingChunk), nextIdx=%d", pipe.nextIdx)
	}
	if len(pipe.updateCalls) != 0 {
		t.Fatalf("la reanudación en fase 2 no debe PATCHear el status, got %+v", pipe.updateCalls)
	}
}

func TestMaterialProcess_Deliver422_PermanentNotDone(t *testing.T) {
	// La entrega devuelve 422 (sin candidatas seleccionadas) → ErrLearningPermanent: sube
	// como permanente (→ DLQ) y NO marca el job done de fase 2. failIfPermanent lo marca
	// failed best-effort; nunca done.
	pipe := &mockMaterialPipeline{
		job:        &m2m.PipelineJob{JobID: "job-1", Status: jobStatusDone, Phase: 1, ChunkCounts: map[string]int{"done": 3}},
		deliverErr: m2m.ErrLearningPermanent,
	}
	deps := defaultReduceDeps()

	err := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypePermanent {
		t.Fatalf("un 422 en la entrega debe ser permanente, got %v", err)
	}
	if pipe.deliverCalls != 1 {
		t.Fatalf("la entrega debe intentarse una vez, got %d", pipe.deliverCalls)
	}
	for _, u := range pipe.updateCalls {
		if u.status == jobStatusDone {
			t.Fatalf("un 422 en la entrega NO debe marcar el job done, got %+v", pipe.updateCalls)
		}
	}
	// best-effort: se marca failed/phase2 con rastro.
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusFailed || last.phase != 2 || last.lastError == nil {
		t.Fatalf("un permanente en la entrega debe marcar failed/phase2 (best-effort), got %+v", pipe.updateCalls)
	}
}

func TestMaterialProcess_ReducePassTransient_PropagatesNoDeliver(t *testing.T) {
	// Un fallo transitorio de una pasada del reduce (p.ej. 5xx del store) sube intacto: NO
	// marca failed (el redelivery reanuda por status) y NO entrega.
	pipe := &mockMaterialPipeline{
		job: &m2m.PipelineJob{JobID: "job-1", Status: jobStatusDone, Phase: 1, ChunkCounts: map[string]int{"done": 3}},
	}
	deps := defaultReduceDeps()
	deps.Relevance = &fakeRelevance{err: errors.New("learning 503")}

	err := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un fallo transitorio del reduce debe subir transitorio, got %v", err)
	}
	if pipe.deliverCalls != 0 {
		t.Fatalf("un fallo del reduce NO debe entregar, got deliverCalls=%d", pipe.deliverCalls)
	}
	for _, u := range pipe.updateCalls {
		if u.status == jobStatusFailed {
			t.Fatalf("un transitorio del reduce NO debe marcar failed, got %+v", pipe.updateCalls)
		}
	}
}
