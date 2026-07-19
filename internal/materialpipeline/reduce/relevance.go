package reduce

import (
	"context"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/textmatch"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// statusDroppedIrrelevant es el estado terminal de una candidata descartada por la pasada
// 2 (relevancia bajo umbral) o por la pasada 3 (no valida el contrato import). El contrato
// §4 reusa este único estado para «no apta / no entra al draft»: no hay estado nuevo.
const statusDroppedIrrelevant = "dropped_irrelevant"

// Modos del paso de relevancia (D-044.4): por defecto TODO local (la fase 2 solo sale por
// API si el riel lo pide explícitamente, y jamás para candidatas local_only).
const (
	relevanceModeLocal = "local"
	relevanceModeAPI   = "api"
)

// defaultRelevanceMin es el umbral de score bajo el cual una candidata se descarta como
// irrelevante (D-044.3). Fallback cuando la config llega en cero. Escala 0..1 (D-044.4 /
// contrato §4): 0 = no se responde; ~0.5 = periférica; ~1 = central.
const defaultRelevanceMin = 0.4

// defaultRelevanceMaxIdeas es el tope de ideas del AGREGADO global que se muestrean para
// el prompt de una candidata (además de las source_ideas de su origen, siempre presentes).
// Acota el contexto del juez: en el job real CONASET el agregado llegó a >400 ideas y, al
// meterlas todas en cada prompt, no cabían en el num_ctx del modelo local (gemma4:e4b,
// 4096) → el parseo del juez fallaba y toda la selección quedaba en 0. El harness midió la
// condición buena con ~50 ideas globales por prompt (D-044.3). Fallback cuando la config
// llega en cero.
const defaultRelevanceMaxIdeas = 50

// RelevanceConfig parametriza la pasada 2 (relevancia). El constructor aplica defaults si
// un campo llega en su cero, de modo que el literal RelevanceConfig{} sea seguro.
type RelevanceConfig struct {
	// RelevanceMin: score < este umbral → dropped_irrelevant (default 0.4).
	RelevanceMin float64
	// Mode: "local" | "api" — provider del paso (default "local"). Una candidata
	// local_only usa SIEMPRE el provider local aunque el modo sea "api" (D-044.4).
	Mode string
	// VerbatimMaxWords: umbral del candado local_only (default 25, D-044.4).
	VerbatimMaxWords int
	// RelevanceMaxIdeas: tope de ideas del AGREGADO global muestreadas para el prompt de
	// cada candidata (default 50). Las source_ideas del origen van SIEMPRE además de esto
	// (ver ideasForCandidate). Acota el contexto del juez para que quepa en el num_ctx del
	// modelo local (ver defaultRelevanceMaxIdeas).
	RelevanceMaxIdeas int
}

// relevanceJudge es el juicio LLM de relevancia de UNA candidata (pasada 2, un candidata
// por llamada, contexto fresco). Los providers concretos (*ollama.Provider, *api.Provider)
// lo satisfacen vía ScoreRelevance; en test, un fake determinista. Interfaz mínima (ISP):
// no arrastra todo el puerto LLM ni obliga a extenderlo (no romper al harness).
type relevanceJudge interface {
	ScoreRelevance(ctx context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error)
}

// chunkTextResolver lee el texto crudo de un chunk (para el candado verbatim local_only).
// HOY el cliente M2M NO expone el chunk_text de los chunks ya procesados (solo
// GetNextPendingChunk devuelve el de los PENDIENTES), así que en producción esta pieza se
// inyecta nil hasta que F3 abra una ruta de lectura: es un GANCHO. Con resolver nil (o que
// devuelve vacío/err) el candado no puede verificar la candidata; en modo "api" eso se
// resuelve de forma CONSERVADORA (se enruta a local, nunca se arriesga una fuga por API).
type chunkTextResolver interface {
	ChunkText(ctx context.Context, chunkID string) (string, error)
}

// RelevancePass ejecuta la pasada 2 del reduce (relevancia + candado local_only, D-044.3/
// D-044.4). Aislada del processor (el cableado es F3c): recibe sus colaboradores por
// constructor tras interfaces mínimas.
type RelevancePass struct {
	store      candidateStore
	localJudge relevanceJudge
	apiJudge   relevanceJudge
	chunks     chunkTextResolver // puede ser nil (gancho: ver chunkTextResolver)
	cfg        RelevanceConfig
	logger     logger.Logger
}

// NewRelevancePass construye la pasada. `localJudge` es el provider local (obligatorio);
// `apiJudge` es el provider por API (puede ser nil si el modo nunca será "api");
// `chunks` resuelve el chunk_text para el candado verbatim (puede ser nil, ver
// chunkTextResolver). Aplica los defaults (0.4 / "local" / 25) si la config llega en cero.
func NewRelevancePass(store candidateStore, localJudge, apiJudge relevanceJudge, chunks chunkTextResolver, cfg RelevanceConfig, log logger.Logger) *RelevancePass {
	if cfg.RelevanceMin == 0 {
		cfg.RelevanceMin = defaultRelevanceMin
	}
	if cfg.Mode == "" {
		cfg.Mode = relevanceModeLocal
	}
	if cfg.VerbatimMaxWords == 0 {
		cfg.VerbatimMaxWords = defaultVerbatimMaxWords
	}
	if cfg.RelevanceMaxIdeas == 0 {
		cfg.RelevanceMaxIdeas = defaultRelevanceMaxIdeas
	}
	return &RelevancePass{
		store:      store,
		localJudge: localJudge,
		apiJudge:   apiJudge,
		chunks:     chunks,
		cfg:        cfg,
		logger:     log,
	}
}

// RelevanceReport resume lo que hizo la pasada sobre un job (logs/harness/observabilidad).
type RelevanceReport struct {
	Candidates        int  // total de candidatas leídas del job
	Processed         int  // candidatas status=candidate con score==nil (a puntuar)
	SkippedScored     int  // candidatas status=candidate que YA traían score (idempotencia)
	Scored            int  // candidatas a las que se les persistió un score
	DroppedIrrelevant int  // candidatas descartadas por score < RelevanceMin
	Unscored          int  // el LLM falló dos veces → score nil, NO descartada (conservador)
	LLMCalls          int  // llamadas a ScoreRelevance (incluye los reintentos)
	LocalForced       int  // candidatas que en modo "api" se enrutaron a local (candado verbatim o no verificable)
	IdeasFromSource   bool // true: las main_ideas del job se agregaron de source_ideas (desviación, ver Run)
}

// Run corre la pasada 2 sobre un job: puntúa la relevancia de cada representante viva
// (status=candidate, score==nil) con UNA llamada LLM por candidata y descarta las que caen
// bajo el umbral. Idempotente: las terminales y las que ya tienen score se saltan, así que
// re-invocar no re-llama al modelo. Nada se borra.
//
// DESVIACIÓN (reportada en IdeasFromSource): el prompt necesita las main_ideas AGREGADAS
// del job, pero el cliente M2M NO expone los ChunkArtifacts (main_ideas) de los chunks ya
// procesados. Como acordó el contrato de la tarea, se cae al AGREGADO de las source_ideas
// de las candidatas del job (unión normalizada) como proxy de las ideas del material.
//
// El agregado NO se pasa completo al juez: en un job real llega a cientos de ideas y no
// cabe en el num_ctx del modelo local, reventando el parseo del juez (ver
// defaultRelevanceMaxIdeas). Por candidata se arma una lista ACOTADA (ideasForCandidate):
// SIEMPRE sus source_ideas (su origen) + una muestra global determinista del agregado
// hasta RelevanceMaxIdeas. Así reproduce la condición medida por el harness.
//
// Errores: propaga los del store tal cual (el caller los clasifica). Un fallo del juez LLM
// NO aborta la pasada ni descarta la candidata: se reintenta una vez y, si persiste, se
// deja el score nil (conservador — nunca se reprueba por fallo de infra/calidad).
func (r *RelevancePass) Run(ctx context.Context, jobID string) (RelevanceReport, error) {
	records, err := r.store.ListCandidates(ctx, jobID)
	if err != nil {
		return RelevanceReport{}, fmt.Errorf("listando candidatas del job %s: %w", jobID, err)
	}
	report := RelevanceReport{Candidates: len(records), IdeasFromSource: true}

	// Ideas del job = unión normalizada de las source_ideas de TODAS las candidatas
	// (proxy de las main_ideas; ver DESVIACIÓN en el doc de Run).
	mainIdeas := aggregateSourceIdeas(records)
	r.logger.Warn("relevancia: main_ideas del job agregadas de source_ideas (el M2M no expone ChunkArtifacts); revisar en F3",
		"job_id", jobID, "ideas", len(mainIdeas))

	var updates []m2m.CandidateUpdate
	for i := range records {
		rec := records[i]
		if rec.Status != statusCandidate {
			continue // terminal: absorbente (idempotencia por status)
		}
		if rec.Score != nil {
			report.SkippedScored++ // ya puntuada: no re-llamar al modelo (idempotencia)
			continue
		}
		payload, perr := materialpipeline.ValidateCandidatePayload(rec.Payload)
		if perr != nil || payload == nil {
			// La calidad (pasada 3) descarta las que no parsean; aquí solo se puntúa lo
			// parseable. Se deja en candidate y se sigue.
			r.logger.Warn("candidata no parseable en relevancia, se omite del puntaje",
				"job_id", jobID, "candidate_id", rec.ID)
			continue
		}
		report.Processed++

		judge, localForced := r.judgeFor(ctx, rec, *payload)
		if localForced {
			report.LocalForced++
		}

		// Ideas acotadas para el prompt de ESTA candidata: sus source_ideas (origen) +
		// muestra global determinista del agregado hasta el tope (ver ideasForCandidate).
		ideas := ideasForCandidate(payload.SourceIdeas, mainIdeas, r.cfg.RelevanceMaxIdeas)
		result, calls, ok := r.scoreWithRetry(ctx, judge, *payload, ideas)
		report.LLMCalls += calls
		if !ok {
			r.logger.Warn("relevancia: el juez LLM falló dos veces; score nil (no se descarta)",
				"job_id", jobID, "candidate_id", rec.ID)
			report.Unscored++
			continue
		}

		score := result.Score
		update := m2m.CandidateUpdate{ID: rec.ID, Score: &score}
		if score < r.cfg.RelevanceMin {
			dropped := statusDroppedIrrelevant
			update.Status = &dropped
			report.DroppedIrrelevant++
		}
		updates = append(updates, update)
		report.Scored++
	}

	if len(updates) > 0 {
		if _, err := r.store.UpdateCandidates(ctx, updates); err != nil {
			return report, fmt.Errorf("persistiendo scores de relevancia del job %s: %w", jobID, err)
		}
	}

	r.logger.Info("pasada 2 de relevancia completa",
		"job_id", jobID,
		"candidatas", report.Candidates,
		"procesadas", report.Processed,
		"puntuadas", report.Scored,
		"dropped_irrelevant", report.DroppedIrrelevant,
		"sin_score", report.Unscored,
		"local_forzado", report.LocalForced,
		"llm_calls", report.LLMCalls)
	return report, nil
}

// judgeFor elige el provider para una candidata según el modo del paso y el candado
// local_only (D-044.4). Modo "local" (default): siempre local. Modo "api": local SOLO si
// la candidata es local_only (cita verbatim) o no se puede verificar (resolver nil/err/
// vacío) — conservador: en la duda nunca se arriesga una fuga por API. Devuelve el juez y
// si se forzó a local pese al modo "api".
func (r *RelevancePass) judgeFor(ctx context.Context, rec m2m.CandidateRecord, payload materialpipeline.CandidatePayloadV1) (relevanceJudge, bool) {
	if r.cfg.Mode != relevanceModeAPI || r.apiJudge == nil {
		return r.localJudge, false // default local (o api sin provider API): todo local, sin evaluar candado
	}

	chunkText := r.resolveChunkText(ctx, rec.ChunkID)
	if chunkText == "" {
		// No verificable (M2M no expone chunk_text hoy): conservador → local.
		return r.localJudge, true
	}
	if IsLocalOnly(candidateVerbatimText(payload), chunkText, r.cfg.VerbatimMaxWords) {
		return r.localJudge, true
	}
	return r.apiJudge, false
}

// resolveChunkText lee el chunk_text vía el resolver inyectado. Devuelve "" si no hay
// resolver o si falla (el caller lo trata como no verificable → local).
func (r *RelevancePass) resolveChunkText(ctx context.Context, chunkID string) string {
	if r.chunks == nil || chunkID == "" {
		return ""
	}
	text, err := r.chunks.ChunkText(ctx, chunkID)
	if err != nil {
		r.logger.Warn("relevancia: no se pudo leer chunk_text para el candado verbatim; se enruta a local",
			"chunk_id", chunkID, "error", err)
		return ""
	}
	return text
}

// scoreWithRetry llama al juez con UN reintento (D-044.3): una salida malformada o un
// fallo transitorio se reintenta una vez; si persiste, devuelve ok=false y el caller deja
// el score nil sin descartar. Devuelve (resultado, nº de llamadas hechas, ok).
func (r *RelevancePass) scoreWithRetry(ctx context.Context, judge relevanceJudge, payload materialpipeline.CandidatePayloadV1, mainIdeas []string) (llm.RelevanceResult, int, bool) {
	req := llm.RelevanceRequest{
		QuestionText: payload.QuestionText,
		MainIdeas:    mainIdeas,
		Language:     "es",
	}
	result, err := judge.ScoreRelevance(ctx, req)
	if err == nil {
		return result, 1, true
	}
	result, err = judge.ScoreRelevance(ctx, req)
	if err == nil {
		return result, 2, true
	}
	return llm.RelevanceResult{}, 2, false
}

// ideasForCandidate arma la lista de ideas para el prompt de UNA candidata, acotada para
// que quepa en el contexto del juez local (D-044.3). Composición:
//   - SIEMPRE las source_ideas del origen (deduplicadas por textmatch.Normalize,
//     preservando el crudo y el orden) — la candidata debe verse frente a SUS ideas.
//   - + una muestra uniforme DETERMINISTA del agregado global del job (uniformSample),
//     hasta maxGlobal ideas, con dedup exacto contra las del origen.
//
// El orden es estable (origen primero, luego la muestra en orden de aparición) y no usa
// aleatoriedad: la misma entrada produce siempre la misma salida. Un agregado más chico
// que el tope pasa entero.
func ideasForCandidate(sourceIdeas, aggregate []string, maxGlobal int) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(sourceIdeas)+maxGlobal)

	// Origen: siempre presente (dedup interno por normalización).
	for _, idea := range sourceIdeas {
		key := textmatch.Normalize(idea)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, idea)
	}

	// Muestra global determinista, dedup exacto contra el origen.
	for _, idea := range uniformSample(aggregate, maxGlobal) {
		key := textmatch.Normalize(idea)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, idea)
	}
	return out
}

// uniformSample toma hasta `size` elementos de `items` con paso fijo, repartidos de forma
// uniforme sobre el slice ordenado (determinista, sin aleatoriedad). Si `items` no supera
// el tope, se devuelve entero. El índice i·len/size reparte las tomas a lo largo de todo el
// slice y —como len > size— nunca repite índice.
func uniformSample(items []string, size int) []string {
	if size <= 0 {
		return nil
	}
	if len(items) <= size {
		return items
	}
	out := make([]string, 0, size)
	for i := 0; i < size; i++ {
		out = append(out, items[i*len(items)/size])
	}
	return out
}

// aggregateSourceIdeas reúne la unión NORMALIZADA de las source_ideas de todas las
// candidatas del job (proxy de las main_ideas del material; ver DESVIACIÓN en Run). Se
// deduplica por textmatch.Normalize preservando el primer texto crudo visto, y el orden de
// aparición (determinista: los records vienen ordenados por chunk_sequence, id).
func aggregateSourceIdeas(records []m2m.CandidateRecord) []string {
	seen := make(map[string]struct{})
	var out []string
	for i := range records {
		payload, err := materialpipeline.ValidateCandidatePayload(records[i].Payload)
		if err != nil || payload == nil {
			continue
		}
		for _, idea := range payload.SourceIdeas {
			key := textmatch.Normalize(idea)
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, idea)
		}
	}
	return out
}
