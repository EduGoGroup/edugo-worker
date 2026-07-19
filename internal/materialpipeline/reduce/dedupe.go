// Package reduce contiene las pasadas del reduce del pipeline material→evaluación
// (plan 044): destilan las candidatas sobregeneradas de la fase 1 hasta el draft del
// profesor. La primera pasada (DedupePass, D-044.2) agrupa duplicados en escalera de
// costo —letras (textmatch, gratis) → significado (embeddings, casi gratis) → LLM
// (solo la zona gris, un par por llamada)— y deja una representante por grupo.
//
// La pieza es INVOCABLE y aislada del processor (el cableado al MaterialPipelineProcessor
// es F3c): recibe sus colaboradores por constructor (cliente M2M, embedder, juez LLM,
// umbrales, logger) tras interfaces mínimas, de modo que los tests la ejercen con fakes
// deterministas sin tocar Ollama ni learning.
package reduce

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/textmatch"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// Estados de la candidata que la pasada 1 conoce (espejo del contrato §4). Solo las
// `candidate` se procesan; los terminales son absorbentes (idempotencia por status).
const (
	statusCandidate  = "candidate"
	statusDroppedDup = "dropped_dup"
)

// fuzzyThreshold es el umbral OSA del escalón de letras (textmatch), fijado por el
// contrato (D-044.2: Exact → Fuzzy 0.85).
const fuzzyThreshold = 0.85

// Config son los umbrales de coseno del dedupe (D-044.2), calibrados en el harness F1b
// (embeddinggemma; 0.93/0.50 sobre question_text CRUDO). El constructor cae a esos
// defaults si un umbral llega en cero, para que el literal Config{} sea seguro.
type Config struct {
	// DupHigh: coseno ≥ → duplicado directo (sin LLM).
	DupHigh float64
	// DupLow: coseno < → distintas (sin LLM). [DupLow, DupHigh) es la zona gris (LLM).
	DupLow float64
}

// candidateStore es el subconjunto del cliente M2M que el dedupe necesita (ISP): leer
// las candidatas del job y persistir cambios parciales (embeddings, status, grupo). El
// *m2m.LearningPipelineClient lo satisface; en test se usa un fake en memoria.
type candidateStore interface {
	ListCandidates(ctx context.Context, jobID string) ([]m2m.CandidateRecord, error)
	UpdateCandidates(ctx context.Context, updates []m2m.CandidateUpdate) (int, error)
}

// pairJudge es el juicio LLM de equivalencia de UN par (solo zona gris, D-044.2): «¿estas
// dos preguntas preguntan lo mismo?». El llm.LLMProvider lo satisface; en test, un fake
// determinista. Se aísla en una interfaz mínima para no arrastrar todo el puerto LLM.
type pairJudge interface {
	JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error)
}

// DedupePass ejecuta la pasada 1 del reduce (dedupe en escalera, D-044.2).
type DedupePass struct {
	store    candidateStore
	embedder llm.Embedder
	judge    pairJudge
	cfg      Config
	logger   logger.Logger
}

// NewDedupePass construye la pasada. Aplica los defaults calibrados (0.93/0.50) si un
// umbral llega en cero.
func NewDedupePass(store candidateStore, embedder llm.Embedder, judge pairJudge, cfg Config, log logger.Logger) *DedupePass {
	if cfg.DupHigh == 0 {
		cfg.DupHigh = 0.93
	}
	if cfg.DupLow == 0 {
		cfg.DupLow = 0.50
	}
	return &DedupePass{
		store:    store,
		embedder: embedder,
		judge:    judge,
		cfg:      cfg,
		logger:   log,
	}
}

// DedupeReport resume lo que hizo la pasada sobre un job (para logs/harness/observabilidad).
type DedupeReport struct {
	Candidates         int // total de candidatas leídas del job
	Processed          int // candidatas en status=candidate efectivamente comparadas
	Clusters           int // grupos de duplicados con ≥2 miembros
	DroppedDup         int // candidatas marcadas dropped_dup (perdedoras del grupo)
	EmbeddingsComputed int // embeddings nuevos calculados y persistidos
	LLMCalls           int // llamadas a JudgePairEquivalence (solo zona gris)
	PairsText          int // pares resueltos en el escalón de letras (textmatch)
	PairsEmbed         int // pares resueltos en el escalón de significado (coseno)
	PairsLLM           int // pares resueltos por el juez LLM (zona gris)
}

// item es una candidata en proceso: el registro crudo + su payload parseado + el
// embedding (reusado o recién calculado) + las claves normalizadas de correct_answer.
type item struct {
	record  m2m.CandidateRecord
	payload materialpipeline.CandidatePayloadV1
	embed   []float32
	answers []string // claves normalizadas de correct_answer (desempate del escalón 1)
}

// Run corre la pasada 1 sobre un job: lee candidatas, agrupa duplicados en escalera y
// persiste el resultado (representante en `candidate` con dedupe_group; perdedoras en
// `dropped_dup`). Es idempotente: las candidatas ya terminales se saltan, así que
// re-invocar sobre un job ya depurado no re-agrupa ni re-escribe. Nada se borra.
//
// Errores: propaga los del store/embedder/juez tal cual (el caller los clasifica —
// ErrPipelineConflict, ErrLearningPermanent o transitorio— con su semántica del carril).
func (d *DedupePass) Run(ctx context.Context, jobID string) (DedupeReport, error) {
	records, err := d.store.ListCandidates(ctx, jobID)
	if err != nil {
		return DedupeReport{}, fmt.Errorf("listando candidatas del job %s: %w", jobID, err)
	}
	report := DedupeReport{Candidates: len(records)}

	// Solo se procesan las candidatas vivas (status=candidate). Los terminales son
	// absorbentes: saltarlos es lo que hace la pasada idempotente (D-044.3).
	items := make([]*item, 0, len(records))
	for _, rec := range records {
		if rec.Status != statusCandidate {
			continue
		}
		payload, perr := materialpipeline.ValidateCandidatePayload(rec.Payload)
		if perr != nil || payload == nil {
			// La fase 1 ya validó antes de persistir; una candidata que hoy no parsea es
			// una anomalía de datos. No se puede comparar por significado, así que se deja
			// en `candidate` (no se borra ni agrupa) y se sigue con las demás.
			d.logger.Warn("candidata no parseable en dedupe, se omite del agrupado (se deja en candidate)",
				"job_id", jobID, "candidate_id", rec.ID)
			continue
		}
		items = append(items, &item{
			record:  rec,
			payload: *payload,
			answers: answerKeys(*payload),
		})
	}
	report.Processed = len(items)

	// Con 0 o 1 candidata viva no hay pares que comparar.
	if len(items) < 2 {
		return report, nil
	}

	// Escalón de significado: cada candidata necesita su embedding para el coseno par a
	// par. Se reusa el persistido (idempotencia) y se calcula solo el que falte, en un
	// único lote, persistido antes de comparar.
	computed, err := d.ensureEmbeddings(ctx, jobID, items)
	if err != nil {
		return report, err
	}
	report.EmbeddingsComputed = computed

	// Union-find sobre los pares duplicados: agrupa transitivamente (A~B, B~C ⇒ {A,B,C}).
	uf := newUnionFind(len(items))
	cascade := textmatch.NewCascade(textmatch.Exact{}, textmatch.NewFuzzy(fuzzyThreshold))
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			dup, tier, jerr := d.arePairDuplicate(ctx, cascade, items[i], items[j])
			if jerr != nil {
				return report, jerr
			}
			switch tier {
			case tierText:
				report.PairsText++
			case tierEmbed:
				report.PairsEmbed++
			case tierLLM:
				report.PairsLLM++
				report.LLMCalls++
			}
			if dup {
				uf.union(i, j)
			}
		}
	}

	// Materializa los grupos y decide representante por grupo (determinista).
	updates, clusters, dropped := d.resolveGroups(items, uf)
	report.Clusters = clusters
	report.DroppedDup = dropped

	if len(updates) > 0 {
		if _, err := d.store.UpdateCandidates(ctx, updates); err != nil {
			return report, fmt.Errorf("persistiendo agrupado de dedupe del job %s: %w", jobID, err)
		}
	}

	d.logger.Info("pasada 1 de dedupe completa",
		"job_id", jobID,
		"candidatas", report.Candidates,
		"procesadas", report.Processed,
		"grupos", report.Clusters,
		"dropped_dup", report.DroppedDup,
		"embeddings_nuevos", report.EmbeddingsComputed,
		"llm_calls", report.LLMCalls)
	return report, nil
}

// ensureEmbeddings garantiza que cada item tenga su embedding: reusa el persistido y
// calcula en UN lote (question_text CRUDO, sin normalizar — así lo fijó F1b) los que
// falten, persistiéndolos vía UpdateCandidates. Devuelve cuántos calculó.
func (d *DedupePass) ensureEmbeddings(ctx context.Context, jobID string, items []*item) (int, error) {
	var (
		missingIdx  []int
		missingText []string
	)
	for i, it := range items {
		if vec, ok := parseEmbedding(it.record.Embedding); ok {
			it.embed = vec
			continue
		}
		missingIdx = append(missingIdx, i)
		missingText = append(missingText, it.payload.QuestionText)
	}
	if len(missingIdx) == 0 {
		return 0, nil
	}

	vecs, err := d.embedder.Embed(ctx, missingText)
	if err != nil {
		return 0, fmt.Errorf("calculando embeddings del job %s: %w", jobID, err)
	}
	if len(vecs) != len(missingIdx) {
		return 0, fmt.Errorf("embedder devolvió %d vectores para %d textos (job %s)", len(vecs), len(missingIdx), jobID)
	}

	updates := make([]m2m.CandidateUpdate, 0, len(missingIdx))
	for k, idx := range missingIdx {
		items[idx].embed = vecs[k]
		raw, merr := json.Marshal(vecs[k])
		if merr != nil {
			return 0, fmt.Errorf("serializando embedding: %w", merr)
		}
		updates = append(updates, m2m.CandidateUpdate{
			ID:        items[idx].record.ID,
			Embedding: raw,
		})
	}
	if _, err := d.store.UpdateCandidates(ctx, updates); err != nil {
		return 0, fmt.Errorf("persistiendo embeddings del job %s: %w", jobID, err)
	}
	return len(missingIdx), nil
}

// tier identifica en qué escalón se resolvió un par (para el reporte).
type tier int

const (
	tierText  tier = iota // letras (textmatch)
	tierEmbed             // significado (coseno)
	tierLLM               // juez LLM (zona gris)
)

// arePairDuplicate aplica la escalera a UN par y devuelve si son duplicados y en qué
// escalón se decidió. Letras primero (gratis): si el texto empata, desempata por
// correct_answer (respuestas equivalentes ⇒ duplicado; distintas ⇒ NO). Si el texto no
// empata, coseno; y solo la zona gris llega al juez LLM.
func (d *DedupePass) arePairDuplicate(ctx context.Context, cascade *textmatch.Cascade, a, b *item) (bool, tier, error) {
	// Escalón 1 — letras. textmatch normaliza internamente (Exact/Fuzzy sobre Normalize).
	res, err := cascade.Compare(ctx, a.payload.QuestionText, b.payload.QuestionText)
	if err != nil {
		return false, tierText, fmt.Errorf("comparando textos en dedupe: %w", err)
	}
	if res.Outcome == textmatch.OutcomeMatch {
		// Texto igual/casi-igual: la respuesta desempata. Equivalente (o ambas vacías,
		// caso open_ended) ⇒ duplicado; claramente distinta ⇒ preguntas distintas que
		// comparten redacción.
		return equalAnswerSets(a.answers, b.answers), tierText, nil
	}

	// Escalón 2 — significado (coseno sobre embeddings crudos).
	cos, ok := cosine(a.embed, b.embed)
	if ok {
		if cos >= d.cfg.DupHigh {
			return true, tierEmbed, nil
		}
		if cos < d.cfg.DupLow {
			return false, tierEmbed, nil
		}
	}
	// Sin embeddings comparables (longitudes distintas / vector nulo) la zona gris
	// absorbe el par: que decida el juez, no un coseno inválido.

	// Escalón 3 — juez LLM (solo zona gris), un par por llamada.
	verdict, err := d.judge.JudgePairEquivalence(ctx, llm.PairEquivalenceRequest{
		QuestionText: a.payload.QuestionText,
		Expected:     a.payload.QuestionText,
		Candidate:    b.payload.QuestionText,
		Language:     "es",
	})
	if err != nil {
		return false, tierLLM, fmt.Errorf("juez de equivalencia en dedupe: %w", err)
	}
	return verdict.Verdict == llm.VerdictCorrect, tierLLM, nil
}

// resolveGroups convierte el union-find en updates M2M: por cada grupo con ≥2 miembros
// elige representante (más source_ideas; empate → menor chunk_sequence; empate → menor
// id) y marca a las demás `dropped_dup`; todo el grupo (representante incluida) comparte
// un dedupe_group nuevo. Los singletons no se tocan (dedupe_group NULL — decisión de la
// pieza; el contrato lo deja abierto). Devuelve (updates, nº de grupos, nº de dropped).
func (d *DedupePass) resolveGroups(items []*item, uf *unionFind) ([]m2m.CandidateUpdate, int, int) {
	groups := make(map[int][]int)
	for i := range items {
		root := uf.find(i)
		groups[root] = append(groups[root], i)
	}

	var (
		updates    []m2m.CandidateUpdate
		numGroups  int
		numDropped int
	)
	for _, members := range groups {
		if len(members) < 2 {
			continue // singleton: sin duplicado, no se toca
		}
		numGroups++
		group := uuid.NewString()
		repIdx := representative(items, members)
		for _, idx := range members {
			if idx == repIdx {
				// La representante sigue viva; solo hereda el grupo.
				updates = append(updates, m2m.CandidateUpdate{
					ID:          items[idx].record.ID,
					DedupeGroup: strptr(group),
				})
				continue
			}
			dropped := statusDroppedDup
			updates = append(updates, m2m.CandidateUpdate{
				ID:          items[idx].record.ID,
				Status:      &dropped,
				DedupeGroup: strptr(group),
			})
			numDropped++
		}
	}
	return updates, numGroups, numDropped
}

// representative elige la representante del grupo con el criterio determinista del
// contrato: más source_ideas; empate → menor chunk_sequence; empate → menor id.
func representative(items []*item, members []int) int {
	best := members[0]
	for _, idx := range members[1:] {
		if betterRepresentative(items[idx], items[best]) {
			best = idx
		}
	}
	return best
}

// betterRepresentative decide si a debe ganar sobre b como representante.
func betterRepresentative(a, b *item) bool {
	an, bn := len(a.payload.SourceIdeas), len(b.payload.SourceIdeas)
	if an != bn {
		return an > bn
	}
	if a.record.ChunkSequence != b.record.ChunkSequence {
		return a.record.ChunkSequence < b.record.ChunkSequence
	}
	return a.record.ID < b.record.ID
}

// answerKeys extrae las claves normalizadas de correct_answer (polimórfico como en el
// import: string escalar, array para multiple_select, ausente/"" en open_ended). Se usan
// como conjunto para el desempate del escalón 1.
func answerKeys(c materialpipeline.CandidatePayloadV1) []string {
	if len(c.CorrectAnswer) == 0 {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(c.CorrectAnswer, &arr); err == nil {
		return normalizeNonBlank(arr)
	}
	var s string
	if err := json.Unmarshal(c.CorrectAnswer, &s); err == nil {
		return normalizeNonBlank([]string{s})
	}
	return nil
}

// normalizeNonBlank normaliza cada texto (textmatch.Normalize) y descarta los que quedan
// vacíos, devolviendo el conjunto ordenado y sin repetidos (para comparar como set).
func normalizeNonBlank(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		n := textmatch.Normalize(s)
		if n == "" {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// equalAnswerSets compara dos conjuntos de claves de respuesta ya normalizados y
// ordenados. Dos conjuntos vacíos son equivalentes (open_ended o sin respuesta: el texto
// ya empató y no hay respuesta que los distinga).
func equalAnswerSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cosine calcula la similitud coseno de dos vectores. ok=false si las longitudes difieren
// o algún vector es nulo (norma 0): el caller trata ese caso como zona gris (lo decide el
// LLM), nunca como un número inventado.
func cosine(a, b []float32) (float64, bool) {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0, false
	}
	var dot, na, nb float64
	for i := range a {
		fa, fb := float64(a[i]), float64(b[i])
		dot += fa * fb
		na += fa * fa
		nb += fb * fb
	}
	if na == 0 || nb == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), true
}

// parseEmbedding decodifica el embedding crudo persistido ([]float32 en JSON). Un valor
// ausente/null/vacío devuelve ok=false (hay que calcularlo).
func parseEmbedding(raw json.RawMessage) ([]float32, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var vec []float32
	if err := json.Unmarshal(raw, &vec); err != nil || len(vec) == 0 {
		return nil, false
	}
	return vec, true
}

func strptr(s string) *string { return &s }

// unionFind es un union-find clásico con compresión de caminos y unión por rango.
type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	uf := &unionFind{parent: make([]int, n), rank: make([]int, n)}
	for i := range uf.parent {
		uf.parent[i] = i
	}
	return uf
}

func (uf *unionFind) find(x int) int {
	for uf.parent[x] != x {
		uf.parent[x] = uf.parent[uf.parent[x]]
		x = uf.parent[x]
	}
	return x
}

func (uf *unionFind) union(a, b int) {
	ra, rb := uf.find(a), uf.find(b)
	if ra == rb {
		return
	}
	if uf.rank[ra] < uf.rank[rb] {
		ra, rb = rb, ra
	}
	uf.parent[rb] = ra
	if uf.rank[ra] == uf.rank[rb] {
		uf.rank[ra]++
	}
}
