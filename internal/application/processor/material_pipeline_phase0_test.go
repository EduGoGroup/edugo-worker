package processor

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// --- mocks de la fase 0 ---

// mockPhase0Pipeline registra el orden de las llamadas y los payloads para verificar
// la secuencia (chunks antes que el PATCH) y la abstención en los casos de reanudación.
type mockPhase0Pipeline struct {
	job       *m2m.PipelineJob
	getJobErr error

	file       *m2m.PresignedFile
	fileURLErr error

	saveChunksErr error
	patchErr      error

	calls       []string
	savedChunks []m2m.ChunkInput
	patchStatus string
	patchPhase  int16
}

func (m *mockPhase0Pipeline) GetJob(_ context.Context, _ string) (*m2m.PipelineJob, error) {
	m.calls = append(m.calls, "GetJob")
	if m.getJobErr != nil {
		return nil, m.getJobErr
	}
	return m.job, nil
}

func (m *mockPhase0Pipeline) GetFileURL(_ context.Context, _ string) (*m2m.PresignedFile, error) {
	m.calls = append(m.calls, "GetFileURL")
	if m.fileURLErr != nil {
		return nil, m.fileURLErr
	}
	return m.file, nil
}

func (m *mockPhase0Pipeline) SaveChunks(_ context.Context, _ string, chunks []m2m.ChunkInput) error {
	m.calls = append(m.calls, "SaveChunks")
	m.savedChunks = chunks
	return m.saveChunksErr
}

func (m *mockPhase0Pipeline) UpdateJobStatus(_ context.Context, _, status string, phase int16, _ *string) error {
	m.calls = append(m.calls, "UpdateJobStatus")
	m.patchStatus = status
	m.patchPhase = phase
	return m.patchErr
}

// mockPhase0Extractor devuelve un ExtractionResult (o error) fijo y drena el reader
// para simular una extracción real.
type mockPhase0Extractor struct {
	result *pdf.ExtractionResult
	err    error
	called bool
}

func (m *mockPhase0Extractor) ExtractWithMetadata(_ context.Context, reader io.Reader) (*pdf.ExtractionResult, error) {
	m.called = true
	_, _ = io.Copy(io.Discard, reader)
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// downloadRecorder captura la invocación de descarga y devuelve bytes/error fijos.
type downloadRecorder struct {
	called bool
	gotURL string
	gotMax int64
	data   []byte
	err    error
}

func (d *downloadRecorder) fn(_ context.Context, url string, maxBytes int64) ([]byte, error) {
	d.called = true
	d.gotURL = url
	d.gotMax = maxBytes
	return d.data, d.err
}

// newPhase0 arma la pieza con los mocks y una config de porcionado real.
func newPhase0(pipe *mockPhase0Pipeline, dl *downloadRecorder, ex *mockPhase0Extractor) *MaterialPipelinePhase0 {
	return NewMaterialPipelinePhase0(pipe, dl.fn, ex, chunking.DefaultConfig(), 10*1024*1024, newTestLogger())
}

// --- casos ---

func TestPhase0_HappyPath(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job:  &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file: &m2m.PresignedFile{URL: "https://signed/pdf"},
	}
	dl := &downloadRecorder{data: []byte("%PDF-fake-bytes")}
	ex := &mockPhase0Extractor{result: &pdf.ExtractionResult{Text: "Un material breve pero con texto suficiente para un trozo."}}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run devolvió error inesperado: %v", err)
	}

	// Orden: GetJob → GetFileURL → SaveChunks → UpdateJobStatus (chunks antes del PATCH).
	want := []string{"GetJob", "GetFileURL", "SaveChunks", "UpdateJobStatus"}
	if len(pipe.calls) != len(want) {
		t.Fatalf("secuencia de llamadas = %v, se esperaba %v", pipe.calls, want)
	}
	for i := range want {
		if pipe.calls[i] != want[i] {
			t.Fatalf("secuencia de llamadas = %v, se esperaba %v", pipe.calls, want)
		}
	}

	if !dl.called {
		t.Fatal("no se llamó al downloader")
	}
	if dl.gotURL != "https://signed/pdf" {
		t.Fatalf("url de descarga = %q, se esperaba la firmada", dl.gotURL)
	}
	if dl.gotMax != 10*1024*1024 {
		t.Fatalf("maxBytes de descarga = %d, se esperaba el configurado", dl.gotMax)
	}
	if !ex.called {
		t.Fatal("no se llamó al extractor")
	}
	if len(pipe.savedChunks) == 0 {
		t.Fatal("no se persistió ningún chunk")
	}
	if pipe.savedChunks[0].Seq != 0 || pipe.savedChunks[0].ChunkText == "" {
		t.Fatalf("chunk 0 mal formado: %+v", pipe.savedChunks[0])
	}
	if pipe.patchStatus != jobStatusProcessing || pipe.patchPhase != 0 {
		t.Fatalf("PATCH = status %q phase %d, se esperaba processing/0", pipe.patchStatus, pipe.patchPhase)
	}
}

func TestPhase0_Reanudacion_ConChunks(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job: &m2m.PipelineJob{JobID: "job-1", Status: jobStatusProcessing, ChunkCounts: map[string]int{"done": 3}},
	}
	dl := &downloadRecorder{}
	ex := &mockPhase0Extractor{}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run devolvió error inesperado: %v", err)
	}
	if dl.called {
		t.Fatal("no debía descargar nada en reanudación")
	}
	if ex.called {
		t.Fatal("no debía extraer nada en reanudación")
	}
	if len(pipe.calls) != 1 || pipe.calls[0] != "GetJob" {
		t.Fatalf("solo debía leer el job, llamadas = %v", pipe.calls)
	}
}

func TestPhase0_JobTerminal_NoOp(t *testing.T) {
	for _, status := range []string{jobStatusDone, jobStatusFailed} {
		pipe := &mockPhase0Pipeline{
			job: &m2m.PipelineJob{JobID: "job-1", Status: status, ChunkCounts: map[string]int{}},
		}
		dl := &downloadRecorder{}
		ex := &mockPhase0Extractor{}

		err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
		if err != nil {
			t.Fatalf("status %q: Run devolvió error inesperado: %v", status, err)
		}
		if dl.called || ex.called {
			t.Fatalf("status %q: no debía hacer trabajo", status)
		}
		if len(pipe.calls) != 1 {
			t.Fatalf("status %q: solo debía leer el job, llamadas = %v", status, pipe.calls)
		}
	}
}

func TestPhase0_PDFCorrupto_SubePermanente(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job:  &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file: &m2m.PresignedFile{URL: "https://signed/pdf"},
	}
	dl := &downloadRecorder{data: []byte("basura")}
	ex := &mockPhase0Extractor{err: pdf.ErrPDFCorrupt}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err == nil {
		t.Fatal("se esperaba error por PDF corrupto")
	}
	if !errors.Is(err, pdf.ErrPDFCorrupt) {
		t.Fatalf("el error no envuelve ErrPDFCorrupt: %v", err)
	}
	// No debe intentar persistir chunks de un PDF que no se pudo extraer.
	for _, c := range pipe.calls {
		if c == "SaveChunks" || c == "UpdateJobStatus" {
			t.Fatalf("no debía escribir nada tras un PDF corrupto, llamadas = %v", pipe.calls)
		}
	}
	// La clasificación de retry.go debe verlo permanente (sin tocar retry.go).
	if got := classifyError(err); got != ErrorTypePermanent {
		t.Fatalf("classifyError(ErrPDFCorrupt) = %v, se esperaba permanente", got)
	}
}

func TestPhase0_PorcionadoVacio_EsPermanente(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job:  &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file: &m2m.PresignedFile{URL: "https://signed/pdf"},
	}
	dl := &downloadRecorder{data: []byte("x")}
	ex := &mockPhase0Extractor{result: &pdf.ExtractionResult{Text: "   \n\n   "}} // solo espacios → 0 trozos

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err == nil {
		t.Fatal("se esperaba error por porcionado vacío")
	}
	if !errors.Is(err, pdf.ErrPDFEmpty) {
		t.Fatalf("el error no envuelve ErrPDFEmpty: %v", err)
	}
	if classifyError(err) != ErrorTypePermanent {
		t.Fatal("el porcionado vacío debía clasificarse permanente")
	}
}

func TestPhase0_SaveChunks409_ContinuaConPatch(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job:           &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file:          &m2m.PresignedFile{URL: "https://signed/pdf"},
		saveChunksErr: m2m.ErrPipelineConflict,
	}
	dl := &downloadRecorder{data: []byte("%PDF")}
	ex := &mockPhase0Extractor{result: &pdf.ExtractionResult{Text: "Texto con suficientes palabras para un trozo válido."}}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("un 409 en SaveChunks no es fallo, Run devolvió: %v", err)
	}
	// Pese al 409, debe haber avanzado el job a processing (idempotencia).
	if pipe.patchStatus != jobStatusProcessing {
		t.Fatalf("no avanzó a processing tras el 409, PATCH = %q", pipe.patchStatus)
	}
	want := []string{"GetJob", "GetFileURL", "SaveChunks", "UpdateJobStatus"}
	if len(pipe.calls) != len(want) {
		t.Fatalf("llamadas = %v, se esperaba %v", pipe.calls, want)
	}
}

func TestPhase0_Patch409_RetornaNil(t *testing.T) {
	pipe := &mockPhase0Pipeline{
		job:      &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file:     &m2m.PresignedFile{URL: "https://signed/pdf"},
		patchErr: m2m.ErrPipelineConflict,
	}
	dl := &downloadRecorder{data: []byte("%PDF")}
	ex := &mockPhase0Extractor{result: &pdf.ExtractionResult{Text: "Texto con suficientes palabras para un trozo válido."}}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("un 409 en el PATCH no es fallo (guard de estado), Run devolvió: %v", err)
	}
}

func TestPhase0_ErrorDeRed_SubeTransitorio(t *testing.T) {
	netErr := errors.New("dial tcp: connection refused")
	pipe := &mockPhase0Pipeline{
		job:  &m2m.PipelineJob{JobID: "job-1", Status: jobStatusPending, ChunkCounts: map[string]int{}},
		file: &m2m.PresignedFile{URL: "https://signed/pdf"},
	}
	dl := &downloadRecorder{err: netErr}
	ex := &mockPhase0Extractor{}

	err := newPhase0(pipe, dl, ex).Run(context.Background(), "job-1")
	if err == nil {
		t.Fatal("se esperaba el error de descarga")
	}
	if !errors.Is(err, netErr) {
		t.Fatalf("el error no envuelve el de red: %v", err)
	}
	if ex.called {
		t.Fatal("no debía extraer si la descarga falló")
	}
	// Un error de red genérico se clasifica transitorio (retry.go).
	if got := classifyError(err); got != ErrorTypeTransient {
		t.Fatalf("classifyError(red) = %v, se esperaba transitorio", got)
	}
}
