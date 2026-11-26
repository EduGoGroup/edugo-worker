package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
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

// ErrMalformedMaterialEvent marca un evento material.assessment_requested indecodificable
// o inválido. Permanente (→ DLQ): reintentar no lo arregla. classifyError lo trata como
// ErrMalformedEvent (mismo carril permanente) porque lo envuelve.
var ErrMalformedMaterialEvent = fmt.Errorf("%w: evento material.assessment_requested", ErrMalformedEvent)

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
	// MarkChunkFailed aísla un chunk envenenado marcándolo failed. 409 → ErrPipelineConflict
	// (el chunk ya está done: carrera benigna).
	MarkChunkFailed(ctx context.Context, chunkID, reason string) error
	// UpdateJobStatus avanza el estado/fase del job: `processing` (tomar el job, fin de
	// fase, latido) o `failed`. El worker NUNCA pide `done`: learning lo rechaza (409) y
	// solo la entrega lo escribe (HU-IA.5). 409 → ErrPipelineConflict.
	UpdateJobStatus(ctx context.Context, jobID, status string, phase int16, lastError *string) error
	// DeliverJob cierra la fase 2 (reduce): arma el draft desde las candidatas `selected`
	// y lo persiste server-side (assessment_id + job a done fase 2; es lo único que deja
	// un job en `done`). Devuelve el id del assessment y su número de preguntas. 422 sin
	// selected → m2m.ErrNoSelectedCandidates; otro 422 / 404 → ErrLearningPermanent.
	DeliverJob(ctx context.Context, jobID string) (assessmentID string, questions int, err error)
}

// Las pasadas del reduce (fase 2, plan 044) se inyectan tras interfaces mínimas para
// mockearlas en los tests del processor sin tocar embeddings/LLM/learning reales. Los
// tipos concretos del paquete reduce (DedupePass, RelevancePass, QualityPass,
// SelectionPass) las satisfacen; el bootstrap los cablea con Resources (F3c).
type (
	// dedupeRunner ejecuta la pasada 1 (dedupe en escalera, D-044.2).
	dedupeRunner interface {
		Run(ctx context.Context, jobID string) (reduce.DedupeReport, error)
	}
	// relevanceRunner ejecuta la pasada 2 (relevancia + candado local_only, D-044.3/4).
	relevanceRunner interface {
		Run(ctx context.Context, jobID string) (reduce.RelevanceReport, error)
	}
	// qualityRunner ejecuta la pasada 3 (calidad determinista, D-044.3).
	qualityRunner interface {
		Run(ctx context.Context, jobID string) (reduce.QualityReport, error)
	}
	// selectionRunner ejecuta la pasada 4 (selección final, D-044.5). target = cupo de
	// preguntas del job (por parámetro: el M2M no lo expone hoy — ver ReduceDeps).
	selectionRunner interface {
		Run(ctx context.Context, jobID string, targetQuestions int) (reduce.SelectionReport, error)
	}
)

// ReduceDeps agrupa las cuatro pasadas del reduce (fase 2) más el cupo de preguntas por
// defecto, para no engordar la firma del constructor del processor. Se construye en
// bootstrap (F3c) con Resources (embedder, providers local/api según RelevanceMode,
// cliente M2M, config, logger).
//
// TargetQuestionsDefault: el cupo de la selección final. Es el ÚNICO sitio del que sale
// (config del riel, D-044.5; HU-IA.5): learning ya no escribe `target_questions` en los
// params del job y el read-model M2M del GET job (PipelineJob) no trae params.
type ReduceDeps struct {
	Dedupe                 dedupeRunner
	Relevance              relevanceRunner
	Quality                qualityRunner
	Selection              selectionRunner
	TargetQuestionsDefault int
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
	reduce   ReduceDeps
	logger   logger.Logger

	// heartbeatInterval es cada cuánto late el job mientras Process lo tiene (HU-IA.5).
	// El constructor lo deja en jobHeartbeatInterval; los tests lo acortan.
	heartbeatInterval time.Duration
}

// NewMaterialPipelineProcessor construye el processor y COMPONE la fase 0 con las
// mismas dependencias (pipeline M2M, descarga, extractor de PDF, config de porcionado).
// provider DEBE ser el LLM local (candado ADR 0036 §4); el caller (bootstrap) cablea
// b.llmProviders["local"] por código. `reduce` trae las cuatro pasadas de la fase 2
// (F3c), cableadas en bootstrap con Resources.
func NewMaterialPipelineProcessor(
	settings SchoolSettingsReader,
	pipeline MaterialPipelineClient,
	provider MaterialLLMProvider,
	extractor pdfExtractor,
	download fileDownloader,
	chunkCfg chunking.Config,
	maxDownloadBytes int64,
	reduceDeps ReduceDeps,
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
		reduce:   reduceDeps,
		logger:   log,

		heartbeatInterval: jobHeartbeatInterval,
	}
}

// EventType satisface processor.Processor.
func (p *MaterialPipelineProcessor) EventType() string {
	return events.EventTypeMaterialAssessmentRequested
}

// Process decodifica el evento, aplica la política de la escuela y orquesta el pipeline.
// Errores:
//   - evento malformado → ErrMalformedMaterialEvent (permanente → DLQ).
//   - política ausente o ≠ "on" → el job se cierra como failed con su motivo y ACK
//     (HU-IA.5: la docente ve «Falló» y por qué, no «En cola» para siempre).
//   - settings inaccesible → transitorio (el consumer reintenta).
//   - un fallo de servicio en el ÚLTIMO reintento cierra el job como failed antes de
//     que el mensaje muera; el error sube igual.
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
		p.logger.Info("carril material→evaluación apagado para la escuela (llm.pipeline.mode≠on): el job se cierra como failed",
			"job_id", jobID, "material_id", evt.Payload.MaterialID, "school_id", schoolID, "mode", mode)
		return p.closeJobPipelineOff(ctx, jobID)
	}

	return p.failOnLastAttempt(ctx, jobID, p.orchestrate(ctx, jobID))
}

// orchestrate ejecuta el pipeline para un job cuya escuela tiene el riel encendido:
// guarda de estado, tomar el job, fase 0 determinista, loop de fase 1 (LLM local) y
// fase 2 (reduce). El modelo de fases del job (status/phase) dirige la reanudación SIN
// mecanismo nuevo (HU-IA.5):
//   - failed → dead-end, nada que hacer (ACK) y NINGÚN PATCH: lo cerró un fallo
//     anterior o el barrido de jobs colgados.
//   - done → solo la entrega lo escribe, así que ya tiene borrador: no-op (ACK).
//   - pending/processing → el worker TOMA el job (PATCH processing, que además es el
//     primer latido) y late mientras lo procesa. Con fase < 1 corre la fase 0 y la 1;
//     con fase ≥ 1 (reentrega de un job ya digerido) va directo a la fase 2.
//
// Un error permanente marca el job failed con su motivo antes de subir para que caiga
// al DLQ con rastro; uno transitorio se deja subir intacto (el redelivery reanuda por
// status sin marcar nada: las pasadas del reduce saltan lo terminal).
func (p *MaterialPipelineProcessor) orchestrate(ctx context.Context, jobID string) error {
	job, err := p.pipeline.GetJob(ctx, jobID)
	if err != nil {
		// 404 = job borrado: permanente (→ DLQ). No se marca failed (no hay job que marcar).
		if errors.Is(err, m2m.ErrLearningPermanent) {
			p.logger.Warn("job inexistente o borrado (permanente → DLQ)", "job_id", jobID, "error", err.Error())
		}
		return fmt.Errorf("leyendo job %s del pipeline: %w", jobID, err)
	}

	if job.Status == jobStatusFailed {
		p.logger.Info("job en estado failed, nada que procesar (ACK)", "job_id", jobID)
		return nil
	}
	if job.Status == jobStatusDone {
		p.logger.Info("job ya entregado (done), no-op (ACK)", "job_id", jobID)
		return nil
	}

	// Tomar el job: pending → processing (o processing → processing en una reentrega).
	// Va DESPUÉS de leer la política y el estado: con el riel apagado, o sobre un job
	// ya cerrado, no sale ningún PATCH processing. 409 = el job se cerró entre la
	// lectura y el PATCH (el barrido, otro worker): nada que procesar.
	if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusProcessing, job.Phase, nil); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("el job se cerró antes de tomarlo (409 en el PATCH processing), ACK", "job_id", jobID)
			return nil
		}
		return p.failIfPermanent(ctx, jobID, job.Phase, fmt.Errorf("tomando el job %s (processing): %w", jobID, err))
	}
	stopHeartbeat := p.startJobHeartbeat(ctx, jobID)
	defer stopHeartbeat()

	if job.Phase < 1 {
		// Fase 0 (determinista, sin LLM): idempotente y reanudable; se salta sola si el
		// job ya está porcionado. Sus errores permanentes (sentinels de PDF) marcan failed.
		if err := p.phase0.Run(ctx, jobID); err != nil {
			return p.failIfPermanent(ctx, jobID, 0, err)
		}

		// Fase 1 (LLM local): loop de chunks pendientes. Deja el job en processing/phase 1.
		if err := p.runPhase1(ctx, jobID); err != nil {
			return err
		}
	} else {
		p.logger.Info("job con la fase 1 completa, se reanuda en la fase 2 (reduce)", "job_id", jobID)
	}

	// Fase 2 (reduce): con el job en processing/phase 1; la entrega lo cierra en done.
	return p.runPhase2(ctx, jobID)
}

// runPhase2 ejecuta el reduce (fase 2, plan 044) sobre un job con la fase 1 completa:
// destila las candidatas sobregeneradas hasta el draft del profesor en la escalera de
// costo (dedupe → relevancia → calidad → selección) y las entrega vía M2M. Es
// RE-INVOCABLE por status sin mecanismo nuevo: cada pasada salta lo terminal, así que un
// fallo a mitad no corrompe —el redelivery reanuda donde quedó—. Contrato de errores
// idéntico a la fase 1: un permanente marca el job failed (best-effort) antes de subir al
// DLQ; un transitorio sube intacto para que el evento se reintente. El job sigue en
// processing durante toda la fase 2 (HU-IA.5): no hay `done` hasta que hay borrador.
func (p *MaterialPipelineProcessor) runPhase2(ctx context.Context, jobID string) error {
	const phase = int16(2)

	// Pasada 1 — dedupe (letras → significado → LLM residual, D-044.2).
	dedupeRep, err := p.reduce.Dedupe.Run(ctx, jobID)
	if err != nil {
		return p.failIfPermanent(ctx, jobID, phase, fmt.Errorf("reduce: dedupe del job %s: %w", jobID, err))
	}
	p.logger.Info("fase 2 · pasada 1 (dedupe) completa",
		"job_id", jobID, "candidatas", dedupeRep.Candidates, "procesadas", dedupeRep.Processed,
		"grupos", dedupeRep.Clusters, "dropped_dup", dedupeRep.DroppedDup, "llm_calls", dedupeRep.LLMCalls)

	// Pasada 2 — relevancia (LLM, una candidata por llamada, D-044.3).
	relRep, err := p.reduce.Relevance.Run(ctx, jobID)
	if err != nil {
		return p.failIfPermanent(ctx, jobID, phase, fmt.Errorf("reduce: relevancia del job %s: %w", jobID, err))
	}
	p.logger.Info("fase 2 · pasada 2 (relevancia) completa",
		"job_id", jobID, "puntuadas", relRep.Scored, "dropped_irrelevant", relRep.DroppedIrrelevant,
		"sin_score", relRep.Unscored, "local_forzado", relRep.LocalForced, "llm_calls", relRep.LLMCalls)

	// Pasada 3 — calidad (determinista Go, gratis, D-044.3).
	qualRep, err := p.reduce.Quality.Run(ctx, jobID)
	if err != nil {
		return p.failIfPermanent(ctx, jobID, phase, fmt.Errorf("reduce: calidad del job %s: %w", jobID, err))
	}
	p.logger.Info("fase 2 · pasada 3 (calidad) completa",
		"job_id", jobID, "validas", qualRep.Valid, "dropped_invalid", qualRep.DroppedInvalid)

	// Pasada 4 — selección final (determinista Go, D-044.5). El cupo viene por parámetro:
	// el M2M no expone target_questions hoy, así que se usa el default del riel.
	target := p.reduce.TargetQuestionsDefault
	selRep, err := p.reduce.Selection.Run(ctx, jobID, target)
	if err != nil {
		return p.failIfPermanent(ctx, jobID, phase, fmt.Errorf("reduce: selección del job %s: %w", jobID, err))
	}
	p.logger.Info("fase 2 · pasada 4 (selección) completa",
		"job_id", jobID, "seleccionadas", selRep.Selected, "target", target,
		"ideas_cubiertas", selRep.IdeasCubiertas, "ideas_sin_cubrir", len(selRep.IdeasSinCubrir))

	// Entrega M2M: learning arma el draft desde las candidatas `selected` y cierra la
	// fase 2 server-side (assessment_id + done/phase2). Sin selected se llama igual: su
	// 422 es permanente y deja el job failed con su motivo (deuda 171).
	assessmentID, questions, err := p.pipeline.DeliverJob(ctx, jobID)
	if err != nil {
		return p.failIfPermanent(ctx, jobID, phase, fmt.Errorf("reduce: entrega del job %s: %w: %w", jobID, errDraftDelivery, err))
	}
	p.logger.Info("fase 2 completa: draft entregado (assessment creado, job en done fase 2)",
		"job_id", jobID, "assessment_id", assessmentID, "preguntas", questions, "seleccionadas", selRep.Selected)
	return nil
}

// runPhase1 recorre los chunks pendientes hasta agotarlos y deja el job en
// processing/phase 1 (HU-IA.5: el job NO pasa a done aquí; lo cierra la entrega).
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
			// No quedan pendientes: marcar el fin de la fase 1. El 409 (el job se cerró
			// mientras tanto) sube como error: la fase 2 no debe correr sobre un job cerrado.
			if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusProcessing, 1, nil); err != nil {
				return p.failIfPermanent(ctx, jobID, 1, fmt.Errorf("marcando el fin de la fase 1 del job %s: %w", jobID, err))
			}
			p.logger.Info("fase 1 completa: job en processing, fase 1 (todas las porciones procesadas)", "job_id", jobID)
			return nil
		}

		if err := p.processChunk(ctx, jobID, next); err != nil {
			return p.failIfPermanent(ctx, jobID, 1, err)
		}
	}
}

// failIfPermanent marca el job como failed (best-effort) SOLO si el error es permanente,
// para que el mensaje caiga al DLQ con rastro: last_error lleva primero el motivo en
// español que ve la docente y detrás el detalle técnico (jobFailureReason). Ignora el
// error del PATCH (best-effort): si falla, el redelivery/DLQ nativo sigue operando.
// Devuelve el error original intacto para que retry.go lo clasifique. Los transitorios
// NO marcan failed aquí: el redelivery reanuda el job donde quedó (y si era el último
// reintento lo cierra failOnLastAttempt).
func (p *MaterialPipelineProcessor) failIfPermanent(ctx context.Context, jobID string, phase int16, err error) error {
	if classifyError(err) != ErrorTypePermanent {
		return err
	}
	p.markJobFailed(ctx, jobID, phase, jobFailureReason(phase, err))
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
