package main

// material_rail.go — monta el procesador de PRODUCCIÓN del riel material→evaluación
// (processor.NewMaterialPipelineProcessor y su Process) sobre dobles en memoria
// (AR-IA.3, decisión 1): el almacén hace de learning, la política de la escuela está
// encendida y la descarga lee el PDF del disco. Lo demás es lo real: el extractor de
// PDF, el troceo, las dos llamadas de la fase 1 con su reintento de calidad y las
// cuatro pasadas de la fase 2, con el mismo cableado que
// internal/bootstrap/resource_builder.go. Sin cola, sin learning y sin base.
//
// Una reentrega de la cola se imita volviendo a llamar a Process sobre el mismo
// almacén (decisión 9a). Nada de lo que escribe el modelo se repite para que salga
// bien: solo se reentrega tras un error transitorio, como haría el consumer.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
)

// maxRedeliveries es cuántas veces se vuelve a llamar a Process tras un error
// transitorio antes de guardar el caso como incompleto (decisión 9a).
const maxRedeliveries = 3

// Desenlaces de un caso.
const (
	outcomeComplete   = "complete"   // el job quedó done con borrador
	outcomeNoDraft    = "no_draft"   // terminó sin borrador por un permanente (p. ej. sin seleccionadas): es un dato
	outcomeIncomplete = "incomplete" // se agotaron las reentregas: el caso quedó a medias
)

// chunkConfigFrom arma la configuración del troceo desde la del riel.
func chunkConfigFrom(mp config.MaterialPipelineConfig) chunking.Config {
	return chunking.Config{
		TargetWords:         mp.ChunkTargetWords,
		MaxWords:            mp.ChunkMaxWords,
		MinWords:            mp.ChunkMinWords,
		MergeThresholdWords: mp.ChunkMergeThresholdWords,
	}
}

// pipelineOnSettings es la política de una escuela con el riel encendido.
type pipelineOnSettings struct{}

func (pipelineOnSettings) GetSettings(_ context.Context, schoolID string) (m2m.SchoolSettings, error) {
	return m2m.SchoolSettings{
		SchoolID: schoolID,
		Settings: []m2m.ResolvedSetting{{Key: "llm.pipeline.mode", Value: "on", Source: "school"}},
	}, nil
}

// localFileDownload hace de descarga: la «URL» es una ruta del disco.
func localFileDownload(_ context.Context, url string, maxBytes int64) ([]byte, error) {
	data, err := os.ReadFile(url)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes", pdf.ErrPDFTooLarge, len(data))
	}
	return data, nil
}

// passRun es una ejecución de una pasada de la fase 2: su informe, su error y lo que
// tardó. Con reentregas hay más de una por pasada.
type passRun[R any] struct {
	Report    R      `json:"report"`
	Error     string `json:"error,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// passLog guarda las ejecuciones de una pasada.
type passLog[R any] struct{ runs []passRun[R] }

func (l *passLog[R]) record(report R, err error, start time.Time) {
	run := passRun[R]{Report: report, ElapsedMS: time.Since(start).Milliseconds()}
	if err != nil {
		run.Error = err.Error()
		run.ErrorKind = classifyModelError(err)
	}
	l.runs = append(l.runs, run)
}

// firstOK devuelve el informe de la primera ejecución sin error: la que vio el
// trabajo entero (una reentrega posterior encuentra ya resuelto lo que la primera
// guardó y su informe sale corto).
func (l *passLog[R]) firstOK() (R, bool) {
	for _, r := range l.runs {
		if r.Error == "" {
			return r.Report, true
		}
	}
	var zero R
	return zero, false
}

func (l *passLog[R]) elapsedMS() int64 {
	var total int64
	for _, r := range l.runs {
		total += r.ElapsedMS
	}
	return total
}

// Las cuatro pasadas reales, envueltas para guardar su informe.
type (
	dedupeProbe struct {
		inner *reduce.DedupePass
		log   passLog[reduce.DedupeReport]
	}
	relevanceProbe struct {
		inner *reduce.RelevancePass
		log   passLog[reduce.RelevanceReport]
	}
	qualityProbe struct {
		inner *reduce.QualityPass
		log   passLog[reduce.QualityReport]
	}
	selectionProbe struct {
		inner *reduce.SelectionPass
		log   passLog[reduce.SelectionReport]
	}
)

func (d *dedupeProbe) Run(ctx context.Context, jobID string) (reduce.DedupeReport, error) {
	start := time.Now()
	rep, err := d.inner.Run(ctx, jobID)
	d.log.record(rep, err, start)
	return rep, err
}

func (r *relevanceProbe) Run(ctx context.Context, jobID string) (reduce.RelevanceReport, error) {
	start := time.Now()
	rep, err := r.inner.Run(ctx, jobID)
	r.log.record(rep, err, start)
	return rep, err
}

func (q *qualityProbe) Run(ctx context.Context, jobID string) (reduce.QualityReport, error) {
	start := time.Now()
	rep, err := q.inner.Run(ctx, jobID)
	q.log.record(rep, err, start)
	return rep, err
}

func (s *selectionProbe) Run(ctx context.Context, jobID string, target int) (reduce.SelectionReport, error) {
	start := time.Now()
	rep, err := s.inner.Run(ctx, jobID, target)
	s.log.record(rep, err, start)
	return rep, err
}

// railDeps es lo que el montaje recibe de fuera: el modelo, el embedder, la
// configuración del riel (la de producción) y el logger.
type railDeps struct {
	model    railModel
	embedder llm.Embedder
	cfg      config.MaterialPipelineConfig
	log      logger.Logger
}

// processAttempt es una llamada a Process (la primera entrega o una reentrega).
type processAttempt struct {
	ElapsedMS int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
	// ErrorKind separa la salida inválida del modelo del fallo del proveedor;
	// Permanent dice si el consumer lo mandaría a la cola muerta sin reintentar.
	ErrorKind string `json:"error_kind,omitempty"`
	Permanent bool   `json:"permanent,omitempty"`
}

// railResult es todo lo que deja la corrida de un caso por el riel.
type railResult struct {
	Outcome   string
	Attempts  []processAttempt
	StartedAt time.Time
	EndedAt   time.Time

	store     *memoryStore
	model     *probeModel
	embedder  *probeEmbedder
	dedupe    *dedupeProbe
	relevance *relevanceProbe
	quality   *qualityProbe
	selection *selectionProbe
}

// runRail pasa UN PDF por el procesador real y devuelve lo que quedó. No falla: un
// error de Process es un dato de la corrida.
func runRail(ctx context.Context, deps railDeps, jobID, pdfPath string) *railResult {
	store := newMemoryStore(jobID, pdfPath)
	model := newProbeModel(deps.model)
	embedder := &probeEmbedder{inner: deps.embedder}

	res := &railResult{
		store:    store,
		model:    model,
		embedder: embedder,
		// El juez de duplicadas es el proveedor local, por código, como en producción.
		dedupe: &dedupeProbe{inner: reduce.NewDedupePass(store, embedder, model,
			reduce.Config{DupHigh: deps.cfg.DedupeHigh, DupLow: deps.cfg.DedupeLow}, deps.log)},
		relevance: &relevanceProbe{inner: reduce.NewRelevancePass(store, model, nil, store,
			reduce.RelevanceConfig{
				RelevanceMin:      deps.cfg.RelevanceMin,
				Mode:              deps.cfg.RelevanceMode,
				VerbatimMaxWords:  deps.cfg.VerbatimMaxWords,
				RelevanceMaxIdeas: deps.cfg.RelevanceMaxIdeas,
			}, deps.log)},
		quality:   &qualityProbe{inner: reduce.NewQualityPass(store, deps.log)},
		selection: &selectionProbe{inner: reduce.NewSelectionPass(store, store, deps.log)},
	}

	proc := processor.NewMaterialPipelineProcessor(
		pipelineOnSettings{},
		store,
		model,
		pdf.NewExtractor(deps.log),
		localFileDownload,
		chunkConfigFrom(deps.cfg),
		deps.cfg.DownloadMaxBytes,
		processor.ReduceDeps{
			Dedupe:                 res.dedupe,
			Relevance:              res.relevance,
			Quality:                res.quality,
			Selection:              res.selection,
			TargetQuestionsDefault: deps.cfg.TargetQuestionsDefault,
		},
		deps.log,
	)

	payload, _ := json.Marshal(events.MaterialAssessmentRequestedEvent{
		EventID:      "harness-" + jobID,
		EventType:    events.EventTypeMaterialAssessmentRequested,
		EventVersion: "1.0",
		Timestamp:    time.Now().UTC(),
		Payload: events.MaterialAssessmentRequestedPayload{
			JobID:      jobID,
			MaterialID: "material-" + jobID,
			SchoolID:   "school-harness",
		},
	})

	res.StartedAt = time.Now()
	res.Outcome = outcomeIncomplete
	for attempt := 0; attempt <= maxRedeliveries; attempt++ {
		start := time.Now()
		err := proc.Process(ctx, payload)
		a := processAttempt{ElapsedMS: time.Since(start).Milliseconds()}
		if err != nil {
			a.Error = err.Error()
			a.ErrorKind = classifyModelError(err)
			a.Permanent = processor.IsPermanent(err)
		}
		res.Attempts = append(res.Attempts, a)

		if store.delivered() {
			res.Outcome = outcomeComplete
			break
		}
		// Sin borrador y sin error transitorio: el consumer no reentregaría (un
		// permanente va a la cola muerta; un nil es un ACK). Es contenido, un dato.
		if err == nil || a.Permanent {
			res.Outcome = outcomeNoDraft
			break
		}
	}
	res.EndedAt = time.Now()
	return res
}
