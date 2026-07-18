package processor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// Estados del job del carril material→evaluación (espejo del contrato de learning,
// plan 043 F1). Se declaran como constantes locales a propósito: la fase 0 no importa
// nada de learning, solo conoce el vocabulario del contrato.
const (
	jobStatusPending    = "pending"
	jobStatusProcessing = "processing"
	jobStatusDone       = "done"
	jobStatusFailed     = "failed"
)

// Guardas de conformidad: las implementaciones reales que F3c cableará satisfacen
// las interfaces locales estrechas de la fase 0. Si alguna firma deriva, esto rompe
// la compilación aquí y no en el wiring.
var (
	_ phase0PipelineClient = (*m2m.LearningPipelineClient)(nil)
	_ pdfExtractor         = (pdf.Extractor)(nil)
	_ fileDownloader       = m2m.DownloadFile
)

// phase0PipelineClient es la porción del LearningPipelineClient M2M que consume la
// fase 0. Interfaz local y estrecha (patrón question_prep_processor.go) para poder
// mockearla en tests; *m2m.LearningPipelineClient la satisface.
type phase0PipelineClient interface {
	// GetJob lee el estado del job (status, phase, chunk_counts).
	GetJob(ctx context.Context, jobID string) (*m2m.PipelineJob, error)
	// GetFileURL obtiene la URL firmada del PDF del material del job.
	GetFileURL(ctx context.Context, jobID string) (*m2m.PresignedFile, error)
	// SaveChunks persiste las porciones. 409 → m2m.ErrPipelineConflict (idempotencia).
	SaveChunks(ctx context.Context, jobID string, chunks []m2m.ChunkInput) error
	// UpdateJobStatus avanza el estado/fase del job. 409 → m2m.ErrPipelineConflict.
	UpdateJobStatus(ctx context.Context, jobID, status string, phase int16, lastError *string) error
}

// fileDownloader baja los bytes de una URL firmada respetando un tope de tamaño.
// m2m.DownloadFile satisface esta firma. Se toma como función (no interfaz) porque
// es una operación sin estado y así el test inyecta un closure sin ceremonia.
type fileDownloader func(ctx context.Context, url string, maxBytes int64) ([]byte, error)

// pdfExtractor es la porción del Extractor de PDF que usa la fase 0. Interfaz local
// para mockearla en tests sin PDFs reales; *pdf.Extractor la satisface.
type pdfExtractor interface {
	ExtractWithMetadata(ctx context.Context, reader io.Reader) (*pdf.ExtractionResult, error)
}

// MaterialPipelinePhase0 ejecuta la fase 0 (determinista, sin LLM) del carril
// material→evaluación (plan 043 D-043.6, pasos 3-4 de D-043.8): baja el PDF del
// material, extrae su texto, lo porciona y persiste los trozos, y avanza el job a
// `processing`. No consume Rabbit ni conoce el evento: es una pieza pura y testeable
// que el MaterialPipelineProcessor (F3c) invoca. Toda su lógica es reanudable e
// idempotente: si otro worker ya porcionó (409) o el job ya trae chunks, se abstiene
// sin ruido.
type MaterialPipelinePhase0 struct {
	pipeline         phase0PipelineClient
	download         fileDownloader
	extractor        pdfExtractor
	chunkCfg         chunking.Config
	maxDownloadBytes int64
	logger           logger.Logger
}

// NewMaterialPipelinePhase0 construye la pieza de fase 0.
func NewMaterialPipelinePhase0(
	pipeline phase0PipelineClient,
	download fileDownloader,
	extractor pdfExtractor,
	chunkCfg chunking.Config,
	maxDownloadBytes int64,
	log logger.Logger,
) *MaterialPipelinePhase0 {
	return &MaterialPipelinePhase0{
		pipeline:         pipeline,
		download:         download,
		extractor:        extractor,
		chunkCfg:         chunkCfg,
		maxDownloadBytes: maxDownloadBytes,
		logger:           log,
	}
}

// Run ejecuta la fase 0 para el job dado. Contrato de errores:
//   - Estado terminal (done/failed) o job ya porcionado → nada que hacer, nil.
//   - Sentinels del PDF (corrupt/scanned/too-large/empty) suben tal cual: permanentes
//     (retry.go los manda a DLQ sin reintento). NUNCA se envuelven de forma que rompa
//     errors.Is.
//   - 409 al persistir chunks o al avanzar el job = guard de idempotencia, no fallo:
//     se sigue (o se ACKea) sin error.
//   - Cualquier otro error sube sin tragar: retry.go decide transitorio (retry/DLQ) o
//     permanente según su clasificación.
func (p *MaterialPipelinePhase0) Run(ctx context.Context, jobID string) error {
	job, err := p.pipeline.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("leyendo job %s del pipeline: %w", jobID, err)
	}

	// a. Estado terminal: el job ya terminó (done) o falló (failed). Nada que hacer.
	if job.Status != jobStatusPending && job.Status != jobStatusProcessing {
		p.logger.Info("job en estado terminal, la fase 0 no aplica (no-op)",
			"job_id", jobID, "status", job.Status)
		return nil
	}

	// b. Reanudación: si el job ya trae chunks, la fase 0 ya corrió; el loop de fase 1
	// es F3 y aquí no hay más trabajo.
	if total := totalChunks(job.ChunkCounts); total > 0 {
		p.logger.Info("job ya porcionado, la fase 0 no se repite (reanudación → fase 1)",
			"job_id", jobID, "chunk_total", total)
		return nil
	}

	// c. URL firmada + descarga (GET presignado, sin Authorization).
	file, err := p.pipeline.GetFileURL(ctx, jobID)
	if err != nil {
		return fmt.Errorf("obteniendo la url del pdf del job %s: %w", jobID, err)
	}
	data, err := p.download(ctx, file.URL, p.maxDownloadBytes)
	if err != nil {
		return fmt.Errorf("descargando el pdf del job %s: %w", jobID, err)
	}

	// d. Extracción. Los 4 sentinels del PDF suben tal cual (permanentes → DLQ).
	result, err := p.extractor.ExtractWithMetadata(ctx, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("extrayendo texto del pdf del job %s: %w", jobID, err)
	}

	// e. Porcionado determinista. Cero trozos (aun con texto extraído) = PDF sin
	// contenido útil: permanente (se trata como ErrPDFEmpty, va a DLQ sin reintento).
	chunks := chunking.Split(result.Text, p.chunkCfg)
	if len(chunks) == 0 {
		return fmt.Errorf("el porcionado del job %s no produjo trozos: %w", jobID, pdf.ErrPDFEmpty)
	}

	// f. Persistir las porciones. 409 = otro worker ya cerró el porcionado: no es
	// fallo, se sigue al PATCH (idempotencia).
	inputs := make([]m2m.ChunkInput, len(chunks))
	for i, c := range chunks {
		inputs[i] = m2m.ChunkInput{Seq: c.Seq, ChunkText: c.Text}
	}
	if err := p.pipeline.SaveChunks(ctx, jobID, inputs); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("chunks ya cerrados por otro worker (409), la fase 0 continúa idempotente",
				"job_id", jobID)
		} else {
			return fmt.Errorf("persistiendo los chunks del job %s: %w", jobID, err)
		}
	} else {
		p.logger.Info("material porcionado y persistido (fase 0)",
			"job_id", jobID, "chunk_total", len(inputs))
	}

	// g. Avanzar el job a `processing`, fase 0. 409 = ya estaba processing (redelivery):
	// guard de estado, no fallo → ACK (nil).
	if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusProcessing, 0, nil); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("el job ya estaba en processing (409 en el PATCH), la fase 0 es idempotente (ACK)",
				"job_id", jobID)
			return nil
		}
		return fmt.Errorf("avanzando el job %s a processing: %w", jobID, err)
	}

	p.logger.Info("fase 0 completa: job en processing, listo para la fase 1",
		"job_id", jobID, "chunk_total", len(inputs))
	return nil
}

// totalChunks suma los conteos por status del job. Cero = sin porcionar.
func totalChunks(counts map[string]int) int {
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}
