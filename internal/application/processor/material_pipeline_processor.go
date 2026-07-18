package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// settingKeyPipelineMode es la clave de política por escuela que gobierna el carril
// material→evaluación (plan 043). A diferencia de generación/corrección, es binaria
// (off|on): solo prende o apaga el riel. NO elige provider —la fase 1 fuerza el LLM
// local por CÓDIGO (candado ADR 0036 §4)—; por eso el processor jamás lee un provider
// de settings.
const settingKeyPipelineMode = "llm.pipeline.mode"

// Valores de la política del carril de materiales.
const (
	pipelineModeOff = "off" // riel apagado (default de plataforma)
	pipelineModeOn  = "on"  // riel encendido para la escuela
)

// materialLanguage es el idioma que se pide al LLM en ambas llamadas del pipeline
// (regla global del ecosistema: español).
const materialLanguage = "es"

// maxSummaryWords es el techo de palabras del summary encadenable de la llamada A
// (D-043.7): escrito para otro modelo, mínimo en tokens. Un summary más largo se trata
// como salida no válida del LLM (transitorio), nunca se persiste.
const maxSummaryWords = 120

// ErrMalformedMaterialEvent marca un evento material.assessment_requested indecodificable
// o inválido. Permanente (→ DLQ): reintentar no lo arregla. classifyError lo trata como
// ErrMalformedEvent (mismo carril permanente) porque lo envuelve.
var ErrMalformedMaterialEvent = fmt.Errorf("%w: evento material.assessment_requested", ErrMalformedEvent)

// ErrInvalidChunkArtifacts marca artefactos/summary del LLM que no cumplen el contrato
// v1. Se trata como fallo del provider (transitorio): reintentar puede dar una salida
// válida; NUNCA se persiste una salida malformada (envenenaría la generación posterior).
var ErrInvalidChunkArtifacts = errors.New("artefactos del chunk inválidos (no cumplen el contrato v1)")

// ErrNoValidCandidates marca un chunk cuyo lote de candidatas quedó vacío tras validar.
// Transitorio (como un fallo del provider): no se persisten artefactos sin ninguna
// pregunta candidata; reintentar puede producir candidatas válidas.
var ErrNoValidCandidates = errors.New("el chunk no produjo candidatas válidas")

// MaterialPipelineClient es la porción del LearningPipelineClient M2M que usa el
// processor del carril de materiales. Es un superconjunto de phase0PipelineClient
// (añade el loop de fase 1: siguiente chunk pendiente + persistencia de artefactos),
// por eso un valor de esta interfaz satisface también a la fase 0. Se define como
// interfaz para mockearla en tests; *m2m.LearningPipelineClient la satisface.
type MaterialPipelineClient interface {
	// GetJob lee el estado del job (status, phase, chunk_counts). 404 → ErrLearningPermanent.
	GetJob(ctx context.Context, jobID string) (*m2m.PipelineJob, error)
	// GetFileURL obtiene la URL firmada del PDF del material del job (lo usa la fase 0).
	GetFileURL(ctx context.Context, jobID string) (*m2m.PresignedFile, error)
	// SaveChunks persiste las porciones (lo usa la fase 0). 409 → ErrPipelineConflict.
	SaveChunks(ctx context.Context, jobID string, chunks []m2m.ChunkInput) error
	// GetNextPendingChunk devuelve el siguiente chunk pendiente o (nil, nil) si no hay.
	GetNextPendingChunk(ctx context.Context, jobID string) (*m2m.NextChunk, error)
	// SaveChunkArtifacts persiste summary + artefactos + candidatas de un chunk.
	// 409 → ErrPipelineConflict (chunk ya cerrado por redelivery).
	SaveChunkArtifacts(ctx context.Context, chunkID string, summary *string, artifacts json.RawMessage, candidates []m2m.CandidatePayload) error
	// UpdateJobStatus avanza el estado/fase del job. 409 → ErrPipelineConflict.
	UpdateJobStatus(ctx context.Context, jobID, status string, phase int16, lastError *string) error
}

// MaterialLLMProvider es la porción del LLMProvider que usa el carril de materiales
// (las dos llamadas del pipeline). Se define como interfaz para mockearla en tests sin
// Ollama; *ollama.Provider (y el de api) la satisface. El processor recibe SOLO el
// provider LOCAL por código (candado ADR 0036 §4): ninguna rama elige provider por
// settings.
type MaterialLLMProvider interface {
	DigestChunk(ctx context.Context, in llm.DigestChunkInput) (*llm.DigestChunkResult, error)
	ProposeCandidates(ctx context.Context, in llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error)
	Name() string
}

// MaterialPipelineProcessor consume material.assessment_requested y orquesta el carril
// material→evaluación (plan 043 D-043.8). Orquestador PURO: cero SQL, todo por M2M.
// Resuelve la política de la escuela (off = ack), ejecuta la fase 0 determinista
// (compuesta, no reimplementada) y el loop de fase 1 (LLM local): por cada chunk
// pendiente lee (DigestChunk), valida artefactos y summary, propone candidatas
// (ProposeCandidates), descarta las inválidas y persiste el resto. Idempotente y
// reanudable: los 409 del carril son guardas de estado, no fallos.
type MaterialPipelineProcessor struct {
	settings SchoolSettingsReader
	pipeline MaterialPipelineClient
	provider MaterialLLMProvider
	phase0   *MaterialPipelinePhase0
	logger   logger.Logger
}

// NewMaterialPipelineProcessor construye el processor y COMPONE la fase 0 con las
// mismas dependencias (pipeline M2M, descarga, extractor de PDF, config de porcionado).
// provider DEBE ser el LLM local (candado ADR 0036 §4); el caller (bootstrap) cablea
// b.llmProviders["local"] por código.
func NewMaterialPipelineProcessor(
	settings SchoolSettingsReader,
	pipeline MaterialPipelineClient,
	provider MaterialLLMProvider,
	extractor pdfExtractor,
	download fileDownloader,
	chunkCfg chunking.Config,
	maxDownloadBytes int64,
	log logger.Logger,
) *MaterialPipelineProcessor {
	// La fase 0 toma una interfaz más estrecha (phase0PipelineClient); MaterialPipelineClient
	// es su superconjunto, así que el mismo cliente satisface ambas.
	phase0 := NewMaterialPipelinePhase0(pipeline, download, extractor, chunkCfg, maxDownloadBytes, log)
	return &MaterialPipelineProcessor{
		settings: settings,
		pipeline: pipeline,
		provider: provider,
		phase0:   phase0,
		logger:   log,
	}
}

// EventType satisface processor.Processor.
func (p *MaterialPipelineProcessor) EventType() string {
	return events.EventTypeMaterialAssessmentRequested
}

// Process decodifica el evento, aplica la política de la escuela y orquesta el pipeline.
// Errores:
//   - evento malformado → ErrMalformedMaterialEvent (permanente → DLQ).
//   - política ausente o ≠ "on" → ACK sin trabajo (no es fallo).
//   - settings inaccesible → transitorio (el consumer reintenta).
func (p *MaterialPipelineProcessor) Process(ctx context.Context, payload []byte) error {
	var evt events.MaterialAssessmentRequestedEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrMalformedMaterialEvent, err)
	}
	if err := validateMaterialEvent(evt); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedMaterialEvent, err)
	}

	schoolID := evt.Payload.SchoolID
	jobID := evt.Payload.JobID

	// Política de la escuela. Un fallo del settings client es transitorio (sube).
	settings, err := p.settings.GetSettings(ctx, schoolID)
	if err != nil {
		return fmt.Errorf("leyendo settings de escuela %s: %w", schoolID, err)
	}
	mode := settingValueOr(settings, settingKeyPipelineMode, pipelineModeOff)
	if mode != pipelineModeOn {
		p.logger.Info("carril material→evaluación apagado para la escuela (llm.pipeline.mode≠on), se ignora (ACK)",
			"job_id", jobID, "material_id", evt.Payload.MaterialID, "school_id", schoolID, "mode", mode)
		return nil
	}

	return p.orchestrate(ctx, jobID)
}

// orchestrate ejecuta el pipeline para un job cuya escuela tiene el riel encendido:
// guarda de estado (404/terminal), fase 0 determinista y loop de fase 1. Un error
// permanente marca el job failed (best-effort) antes de subir para que caiga al DLQ con
// rastro; uno transitorio se deja subir intacto (el redelivery reanuda sin marcar nada).
func (p *MaterialPipelineProcessor) orchestrate(ctx context.Context, jobID string) error {
	job, err := p.pipeline.GetJob(ctx, jobID)
	if err != nil {
		// 404 = job borrado: permanente (→ DLQ). No se marca failed (no hay job que marcar).
		if errors.Is(err, m2m.ErrLearningPermanent) {
			p.logger.Warn("job inexistente o borrado (permanente → DLQ)", "job_id", jobID, "error", err.Error())
		}
		return fmt.Errorf("leyendo job %s del pipeline: %w", jobID, err)
	}
	if job.Status == jobStatusDone || job.Status == jobStatusFailed {
		p.logger.Info("job en estado terminal, nada que procesar (ACK)", "job_id", jobID, "status", job.Status)
		return nil
	}

	// Fase 0 (determinista, sin LLM): idempotente y reanudable; se salta sola si el job
	// ya está porcionado. Sus errores permanentes (sentinels de PDF) marcan failed.
	if err := p.phase0.Run(ctx, jobID); err != nil {
		return p.failIfPermanent(ctx, jobID, 0, err)
	}

	// Fase 1 (LLM local): loop de chunks pendientes.
	return p.runPhase1(ctx, jobID)
}

// runPhase1 recorre los chunks pendientes hasta agotarlos y cierra el job en done.
// Respeta la cancelación del contexto entre chunks (shutdown ordenado).
func (p *MaterialPipelineProcessor) runPhase1(ctx context.Context, jobID string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		next, err := p.pipeline.GetNextPendingChunk(ctx, jobID)
		if err != nil {
			return p.failIfPermanent(ctx, jobID, 1, fmt.Errorf("leyendo el siguiente chunk pendiente del job %s: %w", jobID, err))
		}
		if next == nil {
			// No quedan pendientes: cerrar el job. 409 = otro worker ya lo cerró (ACK).
			if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusDone, 1, nil); err != nil {
				if errors.Is(err, m2m.ErrPipelineConflict) {
					p.logger.Info("el job ya estaba cerrado por otro worker (409 en el PATCH done), ACK", "job_id", jobID)
					return nil
				}
				return p.failIfPermanent(ctx, jobID, 1, fmt.Errorf("cerrando el job %s (done): %w", jobID, err))
			}
			p.logger.Info("fase 1 completa: job en done (todas las porciones procesadas)", "job_id", jobID)
			return nil
		}

		if err := p.processChunk(ctx, jobID, next); err != nil {
			return p.failIfPermanent(ctx, jobID, 1, err)
		}
	}
}

// processChunk ejecuta las dos llamadas del pipeline para un chunk (A "lee", B
// "pregunta"), valida las salidas contra el contrato v1 ANTES de persistir y guarda
// summary + artefactos + candidatas válidas. Devuelve nil (continuar al siguiente chunk)
// también ante un 409 en el PUT (chunk ya cerrado por redelivery). Cualquier salida no
// validable del LLM es fallo transitorio: jamás se persiste algo malformado.
func (p *MaterialPipelineProcessor) processChunk(ctx context.Context, jobID string, chunk *m2m.NextChunk) error {
	// A ("leer"): un fallo del LLM es transitorio (aún no se escribió nada).
	digest, err := p.provider.DigestChunk(ctx, llm.DigestChunkInput{
		ChunkText:   chunk.ChunkText,
		PrevSummary: chunk.PrevSummary,
		Language:    materialLanguage,
	})
	if err != nil {
		return fmt.Errorf("LLM leyendo el chunk %s (job %s): %w", chunk.ChunkID, jobID, err)
	}

	// Validación de contrato ANTES del PUT: artefactos + summary. Salida no válida =
	// fallo del provider (transitorio), nunca se persiste.
	artifactsJSON, err := digest.Artifacts.Marshal()
	if err != nil {
		return fmt.Errorf("serializando artefactos del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, err)
	}
	if _, verr := materialpipeline.ValidateChunkArtifacts(artifactsJSON); verr != nil {
		p.logger.Warn("artefactos del chunk inválidos, se descartan (no se persisten)",
			"chunk_id", chunk.ChunkID, "job_id", jobID, "provider", p.provider.Name(), "motivo", verr.Error())
		return fmt.Errorf("validando artefactos del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, verr)
	}
	if verr := validateSummary(digest.Summary); verr != nil {
		p.logger.Warn("summary del chunk inválido, se descarta (no se persiste)",
			"chunk_id", chunk.ChunkID, "job_id", jobID, "provider", p.provider.Name(), "motivo", verr.Error())
		return fmt.Errorf("validando summary del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, verr)
	}

	// B ("preguntar"): trabaja SOLO con los artefactos (nunca el texto crudo). Fallo del
	// LLM = transitorio.
	candidates, err := p.provider.ProposeCandidates(ctx, llm.ProposeCandidatesInput{
		Artifacts: digest.Artifacts,
		Language:  materialLanguage,
	})
	if err != nil {
		return fmt.Errorf("LLM proponiendo candidatas del chunk %s (job %s): %w", chunk.ChunkID, jobID, err)
	}

	// Validar cada candidata; descartar las inválidas con Warn (sobregenerar está bien,
	// D-043.7: el filtrado fino es del plan 044). CERO válidas = transitorio: no se
	// persisten artefactos sin ninguna pregunta.
	valid := make([]m2m.CandidatePayload, 0, len(candidates))
	for i, cand := range candidates {
		raw, merr := cand.Marshal()
		if merr != nil {
			p.logger.Warn("candidata no serializable, se descarta",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "idx", i, "error", merr.Error())
			continue
		}
		if _, verr := materialpipeline.ValidateCandidatePayload(raw); verr != nil {
			p.logger.Warn("candidata inválida, se descarta",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "idx", i, "motivo", verr.Error())
			continue
		}
		valid = append(valid, m2m.CandidatePayload{Payload: raw})
	}
	if len(valid) == 0 {
		return fmt.Errorf("chunk %s (job %s): %w (de %d propuestas)", chunk.ChunkID, jobID, ErrNoValidCandidates, len(candidates))
	}

	// Persistir. 409 = el chunk ya lo cerró otro worker (redelivery): no es fallo, se
	// continúa al siguiente.
	summary := digest.Summary
	if err := p.pipeline.SaveChunkArtifacts(ctx, chunk.ChunkID, &summary, artifactsJSON, valid); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("chunk ya cerrado por otro worker (409 en el PUT), se continúa al siguiente",
				"chunk_id", chunk.ChunkID, "job_id", jobID)
			return nil
		}
		return fmt.Errorf("persistiendo los artefactos del chunk %s: %w", chunk.ChunkID, err)
	}

	p.logger.Info("chunk procesado (fase 1): artefactos + candidatas persistidos",
		"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq,
		"candidatas_validas", len(valid), "candidatas_descartadas", len(candidates)-len(valid),
		"provider", p.provider.Name())
	return nil
}

// failIfPermanent marca el job como failed (best-effort) SOLO si el error es permanente,
// para que el mensaje caiga al DLQ con rastro del último error. Ignora el error del PATCH
// (best-effort): si falla, el redelivery/DLQ nativo sigue operando. Devuelve el error
// original intacto para que retry.go lo clasifique. Los transitorios NO marcan failed:
// el redelivery reanuda el job donde quedó.
func (p *MaterialPipelineProcessor) failIfPermanent(ctx context.Context, jobID string, phase int16, err error) error {
	if classifyError(err) != ErrorTypePermanent {
		return err
	}
	msg := err.Error()
	if perr := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusFailed, phase, &msg); perr != nil {
		p.logger.Warn("no se pudo marcar el job como failed (best-effort, se ignora)",
			"job_id", jobID, "phase", phase, "error", perr.Error())
	} else {
		p.logger.Info("job marcado como failed antes de subir al DLQ (error permanente)",
			"job_id", jobID, "phase", phase, "last_error", msg)
	}
	return err
}

// validateMaterialEvent comprueba los campos mínimos del evento del carril (los tres
// identificadores no vacíos). El worker lee todo lo demás fresco por M2M.
func validateMaterialEvent(evt events.MaterialAssessmentRequestedEvent) error {
	if evt.EventType != events.EventTypeMaterialAssessmentRequested {
		return fmt.Errorf("event_type inesperado: %q", evt.EventType)
	}
	if evt.Payload.JobID == "" {
		return errors.New("job_id vacío")
	}
	if evt.Payload.MaterialID == "" {
		return errors.New("material_id vacío")
	}
	if evt.Payload.SchoolID == "" {
		return errors.New("school_id vacío")
	}
	return nil
}

// validateSummary comprueba el summary encadenable de la llamada A: no vacío y ≤120
// palabras (D-043.7). Un summary fuera de rango es salida no válida del LLM.
func validateSummary(summary string) error {
	s := strings.TrimSpace(summary)
	if s == "" {
		return errors.New("summary vacío")
	}
	if n := len(strings.Fields(s)); n > maxSummaryWords {
		return fmt.Errorf("summary de %d palabras excede el máximo de %d", n, maxSummaryWords)
	}
	return nil
}
