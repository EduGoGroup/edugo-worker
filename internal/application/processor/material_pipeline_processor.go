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

// Política de resiliencia de las dos llamadas del LLM (plan 043). Ante una salida
// DEGENERADA (summary vacío, JSON sin cierre, listas con ítems vacíos) la llamada se
// reintenta con un jitter de temperatura antes de rendirse. Solo la CALIDAD activa el
// reintento; los fallos de INFRA (Ollama caído/timeout/5xx) siguen siendo transitorios
// (reintento del evento / DLQ), sin reintento in-processor. La misma política gobierna la
// fase A (digest) y la fase B (propuesta de candidatas): ambas envuelven sus errores de
// parseo con llm.ErrLLMQuality para distinguir calidad de infra.
const (
	// llmQualityRetries son los reintentos in-processor de una llamada del LLM ante un
	// fallo de CALIDAD (1 reintento ⇒ 2 intentos totales).
	llmQualityRetries = 1
	// llmRetryTemperature es la temperatura del muestreo SOLO en el/los reintento(s):
	// temp 0 (greedy) repite la misma basura, un poco de jitter desatasca la salida.
	llmRetryTemperature = 0.3
)

// ErrMalformedMaterialEvent marca un evento material.assessment_requested indecodificable
// o inválido. Permanente (→ DLQ): reintentar no lo arregla. classifyError lo trata como
// ErrMalformedEvent (mismo carril permanente) porque lo envuelve.
var ErrMalformedMaterialEvent = fmt.Errorf("%w: evento material.assessment_requested", ErrMalformedEvent)

// ErrInvalidChunkArtifacts marca artefactos/summary del LLM que no cumplen el contrato
// v1. Se trata como fallo del provider (transitorio): reintentar puede dar una salida
// válida; NUNCA se persiste una salida malformada (envenenaría la generación posterior).
var ErrInvalidChunkArtifacts = errors.New("artefactos del chunk inválidos (no cumplen el contrato v1)")

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
// summary + artefactos + candidatas válidas. Su contrato de continuidad:
//   - Fase A resiliente: un fallo de CALIDAD del digest se reintenta con jitter de
//     temperatura y, si persiste, AÍSLA el chunk (failed) y devuelve nil para seguir con
//     el siguiente; un fallo de INFRA sube intacto (transitorio → reintento del evento).
//   - Fase B tolerante: si ninguna candidata valida (sesgo multiple_select conocido) NO
//     es fatal: los artefactos SÍ son válidos, se persisten sin candidatas y se continúa.
//   - Un 409 en el PUT (chunk ya cerrado por redelivery) también devuelve nil (continuar).
//
// Jamás se persiste una salida malformada del LLM (envenenaría la generación posterior).
func (p *MaterialPipelineProcessor) processChunk(ctx context.Context, jobID string, chunk *m2m.NextChunk) error {
	// A ("leer") con reintento por calidad y aislamiento del envenenado.
	digest, artifactsJSON, err := p.digestWithQualityRetry(ctx, jobID, chunk)
	if err != nil {
		return err
	}
	if digest == nil {
		// Chunk aislado (failed) o carrera 409 al aislarlo: no hay fase B, se continúa.
		return nil
	}

	// B ("preguntar"): trabaja SOLO con los artefactos (nunca el texto crudo), con
	// reintento por calidad. Un fallo de CALIDAD persistente NO tumba el evento: se
	// persiste sin candidatas (el digest ES válido); un fallo de INFRA sube transitorio.
	valid, err := p.proposeWithQualityRetry(ctx, jobID, chunk, digest.Artifacts)
	if err != nil {
		return err
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
		"candidatas_validas", len(valid), "provider", p.provider.Name())
	return nil
}

// proposeWithQualityRetry ejecuta la fase B ("preguntar") con la misma política de
// resiliencia que el digest: un fallo de CALIDAD de ProposeCandidates (parseo/JSON, vía
// llm.ErrLLMQuality) se reintenta hasta llmQualityRetries veces con jitter de temperatura
// y, si persiste, NO se propaga —el digest del chunk ES válido—: devuelve una lista vacía
// para que el caller persista artefactos sin candidatas (chunk `done`, cadena de summary
// preservada, NO failed). Un fallo de INFRA se propaga (transitorio → reintento del
// evento). El caso "la llamada respondió pero ninguna candidata cumple el contrato" NO es
// fallo: se filtra y se devuelve la lista (posiblemente vacía) sin reintentar (sesgo
// multiple_select conocido, D-043.7).
func (p *MaterialPipelineProcessor) proposeWithQualityRetry(ctx context.Context, jobID string, chunk *m2m.NextChunk, artifacts materialpipeline.ChunkArtifactsV1) ([]m2m.CandidatePayload, error) {
	var lastQualityErr error
	for attempt := 0; attempt <= llmQualityRetries; attempt++ {
		var tempOverride *float64
		if attempt > 0 {
			t := llmRetryTemperature
			tempOverride = &t
			p.logger.Warn("propuesta de candidatas falló por calidad del LLM; se reintenta con jitter de temperatura",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq,
				"intento", attempt+1, "temp_retry", llmRetryTemperature, "motivo", lastQualityErr.Error())
		}

		candidates, err := p.provider.ProposeCandidates(ctx, llm.ProposeCandidatesInput{
			Artifacts:   artifacts,
			Language:    materialLanguage,
			Temperature: tempOverride,
		})
		if err != nil {
			if !isQualityFailure(err) {
				// INFRA: transitorio, sube sin reintento in-processor.
				return nil, fmt.Errorf("LLM proponiendo candidatas del chunk %s (job %s): %w", chunk.ChunkID, jobID, err)
			}
			lastQualityErr = err
			continue
		}

		valid := p.filterValidCandidates(jobID, chunk, candidates)
		if len(valid) == 0 {
			// CERO válidas NO es fatal (sesgo multiple_select conocido): los artefactos
			// SÍ valen, se persisten sin candidatas para no romper la cadena de summary.
			p.logger.Warn("chunk sin candidatas válidas tras validar; se persisten artefactos sin candidatas y se continúa (no se marca failed)",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq, "propuestas", len(candidates))
		}
		return valid, nil
	}

	// Agotados los reintentos y la propuesta sigue sin calidad: persistir sin candidatas.
	p.logger.Warn("propuesta de candidatas sin calidad tras reintento; se persisten artefactos sin candidatas y se continúa (no se marca failed)",
		"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq, "motivo", lastQualityErr.Error())
	return nil, nil
}

// digestWithQualityRetry ejecuta la fase A ("leer") con la política de resiliencia del
// carril: hasta digestQualityRetries reintentos ante un fallo de CALIDAD (el reintento
// aplica digestRetryTemperature como jitter) y, si el chunk sigue degenerado tras
// agotarlos, lo AÍSLA (failed vía M2M) para continuar con el siguiente. Contrato de
// retorno:
//   - (digest≠nil, artifactsJSON, nil) → éxito: procesar la fase B.
//   - (nil, nil, nil) → chunk aislado (o carrera 409 al aislarlo): continuar SIN fase B.
//   - (nil, nil, err) → fallo de INFRA (del digest o al marcar failed): transitorio, sube.
func (p *MaterialPipelineProcessor) digestWithQualityRetry(ctx context.Context, jobID string, chunk *m2m.NextChunk) (*llm.DigestChunkResult, json.RawMessage, error) {
	var lastQualityErr error
	for attempt := 0; attempt <= llmQualityRetries; attempt++ {
		var tempOverride *float64
		if attempt > 0 {
			t := llmRetryTemperature
			tempOverride = &t
			p.logger.Warn("digest del chunk falló por calidad del LLM; se reintenta con jitter de temperatura",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq,
				"intento", attempt+1, "temp_retry", llmRetryTemperature, "motivo", lastQualityErr.Error())
		}

		digest, artifactsJSON, err := p.attemptDigest(ctx, jobID, chunk, tempOverride)
		if err == nil {
			return digest, artifactsJSON, nil
		}
		if !isQualityFailure(err) {
			// INFRA (Ollama caído/timeout/5xx): transitorio, sube sin reintento in-processor.
			return nil, nil, err
		}
		lastQualityErr = err
	}

	// Agotados los reintentos y el chunk sigue degenerado: aislarlo y continuar.
	if err := p.isolatePoisonedChunk(ctx, jobID, chunk, lastQualityErr); err != nil {
		return nil, nil, err
	}
	return nil, nil, nil
}

// attemptDigest ejecuta UNA pasada de la llamada A: digest del LLM → normalización de
// listas → validación de artefactos y summary contra el contrato v1. Devuelve el digest
// (con sus artefactos ya normalizados) y su serialización lista para persistir. El error
// viene CLASIFICADO: los de CALIDAD (parseo/JSON de DigestChunk vía llm.ErrLLMQuality;
// artefactos que no cumplen el contrato tras normalizar o summary inválido vía
// ErrInvalidChunkArtifacts) permiten distinguirlos de los de INFRA, que suben sin
// sentinel. tempOverride, si != nil, fuerza la temperatura solo en esta pasada.
func (p *MaterialPipelineProcessor) attemptDigest(ctx context.Context, jobID string, chunk *m2m.NextChunk, tempOverride *float64) (*llm.DigestChunkResult, json.RawMessage, error) {
	digest, err := p.provider.DigestChunk(ctx, llm.DigestChunkInput{
		ChunkText:   chunk.ChunkText,
		PrevSummary: chunk.PrevSummary,
		Language:    materialLanguage,
		Temperature: tempOverride,
	})
	if err != nil {
		// El provider ya distingue calidad (llm.ErrLLMQuality) de infra (sin sentinel);
		// el envoltorio preserva errors.Is.
		return nil, nil, fmt.Errorf("LLM leyendo el chunk %s (job %s): %w", chunk.ChunkID, jobID, err)
	}

	// Normalización previa a validar: filtra ítems vacíos de las listas de ideas (salida
	// degenerada intermitente del modelo). NO afloja el contrato: si main_ideas queda
	// vacía tras filtrar, la validación de abajo falla igual.
	digest.Artifacts = materialpipeline.NormalizeChunkArtifacts(digest.Artifacts)

	artifactsJSON, err := digest.Artifacts.Marshal()
	if err != nil {
		return nil, nil, fmt.Errorf("serializando artefactos del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, err)
	}
	if _, verr := materialpipeline.ValidateChunkArtifacts(artifactsJSON); verr != nil {
		p.logger.Warn("artefactos del chunk inválidos tras normalizar, se descartan (no se persisten)",
			"chunk_id", chunk.ChunkID, "job_id", jobID, "provider", p.provider.Name(), "motivo", verr.Error())
		return nil, nil, fmt.Errorf("validando artefactos del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, verr)
	}
	if verr := validateSummary(digest.Summary); verr != nil {
		p.logger.Warn("summary del chunk inválido, se descarta (no se persiste)",
			"chunk_id", chunk.ChunkID, "job_id", jobID, "provider", p.provider.Name(), "motivo", verr.Error())
		return nil, nil, fmt.Errorf("validando summary del chunk %s: %w: %v", chunk.ChunkID, ErrInvalidChunkArtifacts, verr)
	}
	return digest, artifactsJSON, nil
}

// isQualityFailure distingue un fallo de CALIDAD del LLM (salida degenerada: parseo/JSON,
// artefactos que no cumplen el contrato, summary inválido) de uno de INFRA (Ollama caído,
// timeout, HTTP 5xx). Solo la calidad activa el reintento con jitter y el aislamiento del
// chunk; la infra sigue subiendo como transitorio (reintento del evento / DLQ intactos).
func isQualityFailure(err error) bool {
	return errors.Is(err, llm.ErrLLMQuality) || errors.Is(err, ErrInvalidChunkArtifacts)
}

// isolatePoisonedChunk marca el chunk como failed en learning (M2M) y devuelve nil para
// CONTINUAR con el siguiente, en vez de tumbar el evento entero por un solo trozo que el
// modelo no logra digerir con calidad ni al reintentar. El job cierra igual: los chunks
// failed no vuelven a salir de GetNextPendingChunk. Un 409 (el chunk ya está done: carrera)
// se trata como benigno y también continúa; un fallo de INFRA al marcar failed se PROPAGA
// como transitorio (no se traga un error de infraestructura al aislar).
func (p *MaterialPipelineProcessor) isolatePoisonedChunk(ctx context.Context, jobID string, chunk *m2m.NextChunk, cause error) error {
	const reason = "digest del LLM sin calidad tras reintento"
	if err := p.pipeline.MarkChunkFailed(ctx, chunk.ChunkID, reason); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("el chunk ya estaba cerrado al intentar aislarlo (409), se continúa al siguiente",
				"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq)
			return nil
		}
		return fmt.Errorf("aislando el chunk envenenado %s (job %s): %w", chunk.ChunkID, jobID, err)
	}
	p.logger.Warn("chunk envenenado aislado (marcado failed) tras reintento por calidad; se continúa con el siguiente",
		"chunk_id", chunk.ChunkID, "job_id", jobID, "seq", chunk.Seq, "motivo", cause.Error())
	return nil
}

// filterValidCandidates valida cada candidata propuesta por la fase B y devuelve solo las
// que cumplen el contrato v1; descarta las inválidas con Warn (sobregenerar está bien,
// D-043.7: el filtrado fino es del plan 044).
func (p *MaterialPipelineProcessor) filterValidCandidates(jobID string, chunk *m2m.NextChunk, candidates []materialpipeline.CandidatePayloadV1) []m2m.CandidatePayload {
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
	return valid
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
