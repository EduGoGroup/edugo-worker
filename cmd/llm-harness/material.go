package main

// material.go — modo "material" del arnés (AR-IA.3): la medida del riel
// material→evaluación ENTERO. Por cada caso (un PDF) monta el procesador de
// producción sobre un almacén en memoria (material_rail.go) y lo deja correr: fase 0
// (extraer y trocear), fase 1 (leer y preguntar por trozo) y las cuatro pasadas de la
// fase 2, con los reintentos de producción. Solo necesita Ollama.
//
// MIDE, NO JUZGA (ADR 0048): una corrida por caso; una salida inválida del modelo es
// un dato; no imprime veredictos y guarda la corrida en el historial. Termina con
// código != 0 solo si no pudo correr (sin modelo, sin PDF, una sección declarada que
// no aparece en el texto, sin poder guardar).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// defaultMaterialDir es la carpeta de datos del modo, relativa a la raíz del worker.
const defaultMaterialDir = "cmd/llm-harness/testdata/material"

// defaultMaterialCases son los casos congelados de la línea base.
const defaultMaterialCases = defaultMaterialDir + "/cases.json"

// materialCase es un caso: un PDF y, si se declaran, los títulos literales de sus
// secciones (para contar cuántas quedan con pregunta en el borrador).
type materialCase struct {
	ID       string   `json:"id"`
	PDF      string   `json:"pdf"`
	Title    string   `json:"title,omitempty"`
	Sections []string `json:"sections,omitempty"`
}

// loadMaterialCases lee el fichero de casos; las rutas de PDF son relativas a él.
// Devuelve también su huella.
func loadMaterialCases(path string) ([]materialCase, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("leyendo los casos %q: %w", path, err)
	}
	var f struct {
		Cases []materialCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, "", fmt.Errorf("los casos %q no parsean: %w", path, err)
	}
	if len(f.Cases) == 0 {
		return nil, "", fmt.Errorf("los casos %q están vacíos", path)
	}
	seen := map[string]struct{}{}
	for i := range f.Cases {
		c := &f.Cases[i]
		if c.ID == "" || c.PDF == "" {
			return nil, "", fmt.Errorf("caso %d de %q sin id o sin pdf", i, path)
		}
		if _, dup := seen[c.ID]; dup {
			return nil, "", fmt.Errorf("id de caso repetido: %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		if !filepath.IsAbs(c.PDF) {
			c.PDF = filepath.Join(filepath.Dir(path), c.PDF)
		}
	}
	return f.Cases, fileSHA256(raw), nil
}

// casesFromInputs arma casos sin secciones a partir de -material-inputs, para medir
// otro PDF que no está en los casos congelados.
func casesFromInputs(csv string) []materialCase {
	var out []materialCase
	for i, p := range splitCSV(csv) {
		id := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		out = append(out, materialCase{ID: fmt.Sprintf("%s-%d", id, i), PDF: p})
	}
	return out
}

// preparedCase es un caso ya comprobado, antes de llamar a ningún modelo: el PDF
// existe, se extrae, y cada sección declarada aparece en su texto.
type preparedCase struct {
	materialCase
	PDFSHA256      string
	Text           string
	SectionOffsets []int
	ExpectedChunks int
}

// prepareMaterialCases comprueba todos los casos con el extractor y el troceo de
// producción. Un error aquí es del arnés (no pudo correr), no del modelo.
func prepareMaterialCases(cases []materialCase, chunkCfg chunking.Config, log logger.Logger) ([]preparedCase, error) {
	extractor := pdf.NewExtractor(log)
	out := make([]preparedCase, 0, len(cases))
	for _, c := range cases {
		raw, err := os.ReadFile(c.PDF)
		if err != nil {
			return nil, fmt.Errorf("caso %q: leyendo el PDF: %w", c.ID, err)
		}
		res, err := extractor.ExtractWithMetadata(context.Background(), bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("caso %q: extrayendo el texto de %s: %w", c.ID, c.PDF, err)
		}
		offsets, err := sectionOffsets(res.Text, c.Sections)
		if err != nil {
			return nil, fmt.Errorf("caso %q: %w", c.ID, err)
		}
		out = append(out, preparedCase{
			materialCase:   c,
			PDFSHA256:      fileSHA256(raw),
			Text:           res.Text,
			SectionOffsets: offsets,
			ExpectedChunks: len(chunking.Split(res.Text, chunkCfg)),
		})
	}
	return out, nil
}

// materialParams son los parámetros con los que corrió la medida: los de producción,
// leídos de la configuración sin entorno, más los del arnés.
type materialParams struct {
	ChunkTargetWords         int     `json:"chunk_target_words"`
	ChunkMaxWords            int     `json:"chunk_max_words"`
	ChunkMinWords            int     `json:"chunk_min_words"`
	ChunkMergeThresholdWords int     `json:"chunk_merge_threshold_words"`
	DedupeHigh               float64 `json:"dedupe_high"`
	DedupeLow                float64 `json:"dedupe_low"`
	RelevanceMin             float64 `json:"relevance_min"`
	RelevanceMode            string  `json:"relevance_mode"`
	TargetQuestions          int     `json:"target_questions"`
	CallTimeoutS             float64 `json:"call_timeout_s"`
	EmbedTimeoutS            float64 `json:"embed_timeout_s"`
	MaxRedeliveries          int     `json:"max_redeliveries"`
}

func newMaterialParams(cfg config.MaterialPipelineConfig, callTimeout, embedTimeout time.Duration) materialParams {
	return materialParams{
		ChunkTargetWords:         cfg.ChunkTargetWords,
		ChunkMaxWords:            cfg.ChunkMaxWords,
		ChunkMinWords:            cfg.ChunkMinWords,
		ChunkMergeThresholdWords: cfg.ChunkMergeThresholdWords,
		DedupeHigh:               cfg.DedupeHigh,
		DedupeLow:                cfg.DedupeLow,
		RelevanceMin:             cfg.RelevanceMin,
		RelevanceMode:            cfg.RelevanceMode,
		TargetQuestions:          cfg.TargetQuestionsDefault,
		CallTimeoutS:             callTimeout.Seconds(),
		EmbedTimeoutS:            embedTimeout.Seconds(),
		MaxRedeliveries:          maxRedeliveries,
	}
}

// materialCaseRun es un caso de la corrida: su desenlace, sus cuentas y, enteros,
// sus trozos y sus candidatas con el estado final.
type materialCaseRun struct {
	ID           string           `json:"id"`
	Title        string           `json:"title,omitempty"`
	PDF          string           `json:"pdf"`
	PDFSHA256    string           `json:"pdf_sha256"`
	Sections     []string         `json:"sections,omitempty"`
	Outcome      string           `json:"outcome"`
	Attempts     []processAttempt `json:"attempts"`
	JobStatus    string           `json:"job_status"`
	JobPhase     int16            `json:"job_phase"`
	JobLastError string           `json:"job_last_error,omitempty"`
	Metrics      caseMetrics      `json:"metrics"`
	Passes       passRuns         `json:"passes"`
	Chunks       []chunkRun       `json:"chunks"`
	Candidates   []candidateRun   `json:"candidates"`
}

// materialRunData es lo que el modo guarda en el historial.
type materialRunData struct {
	CasesSHA256 string            `json:"cases_sha256,omitempty"`
	Params      materialParams    `json:"params"`
	ElapsedMS   int64             `json:"elapsed_ms"`
	Cases       []materialCaseRun `json:"cases"`
}

// runMaterialCase pasa un caso por el riel y lo cuenta.
func runMaterialCase(ctx context.Context, deps railDeps, c preparedCase) materialCaseRun {
	res := runRail(ctx, deps, "job-"+c.ID, c.PDF)
	metrics, detail := computeCaseMetrics(res, c.Text, c.SectionOffsets, deps.cfg.TargetQuestionsDefault)
	run := materialCaseRun{
		ID: c.ID, Title: c.Title, PDF: filepath.Base(c.PDF), PDFSHA256: c.PDFSHA256, Sections: c.Sections,
		Outcome: res.Outcome, Attempts: res.Attempts,
		JobStatus: res.store.job.Status, JobPhase: res.store.job.Phase,
		Metrics: metrics, Passes: collectPassRuns(res),
		Chunks: detail.Chunks, Candidates: detail.Candidates,
	}
	if res.store.job.LastError != nil {
		run.JobLastError = *res.store.job.LastError
	}
	return run
}

// runMaterial es la entrada del modo: corre los casos ya comprobados, imprime las
// tablas y guarda la corrida.
func runMaterial(deps railDeps, info runInfo, runsDir string, cases []preparedCase, casesSum string, params materialParams) {
	fmt.Printf("== llm-harness (material) ==\n")
	fmt.Printf("modelo    : %s (embeddings: %s)\n", info.Model, info.EmbedModel)
	fmt.Printf("casos     : %d\n\n", len(cases))

	start := time.Now()
	data := materialRunData{CasesSHA256: casesSum, Params: params}
	for _, c := range cases {
		fmt.Printf("caso %s: %s (%d trozos esperados)...\n", c.ID, filepath.Base(c.PDF), c.ExpectedChunks)
		run := runMaterialCase(context.Background(), deps, c)
		fmt.Printf("  desenlace: %s · %d s · entregas: %d\n", run.Outcome, run.Metrics.Cost.TotalMS/1000, len(run.Attempts))
		data.Cases = append(data.Cases, run)
	}
	data.ElapsedMS = time.Since(start).Milliseconds()

	printMaterialTables(data.Cases)
	saveRun(runsDir, info, data)
}

// runMaterialPairs es la entrada del modo material-pairs.
func runMaterialPairs(deps railDeps, info runInfo, runsDir string, f pairsFile, pairsSum string, params materialParams) {
	fmt.Printf("== llm-harness (material-pairs) ==\n")
	fmt.Printf("modelo    : %s (embeddings: %s)\n", info.Model, info.EmbedModel)
	fmt.Printf("parejas   : %d (cada una, dos medidas)\n\n", len(f.Pairs))

	start := time.Now()
	runs, judge, embed, err := runPairs(context.Background(), f, deps.model, deps.embedder, deps.cfg, deps.log, func(r pairRun) {
		fmt.Printf("  %-8s real=%s/%s%s  judge_only=%s/%s%s\n", r.ID,
			orDash(r.Real.Tier), orDash(r.Real.Verdict), errSuffix(r.Real),
			orDash(r.JudgeOnly.Tier), orDash(r.JudgeOnly.Verdict), errSuffix(r.JudgeOnly))
	})
	if err != nil {
		fatalf("midiendo las parejas: %v", err)
	}
	data := pairsRunData{
		PairsSHA256: pairsSum, Params: params,
		Table: pairsTable(f.Categories, runs), Judge: judge, Embed: embed,
		ElapsedMS: time.Since(start).Milliseconds(), Pairs: runs,
	}
	printPairsTable(data.Table)
	saveRun(runsDir, info, data)
}

func errSuffix(m pairMeasure) string {
	if m.ErrorKind == "" {
		return ""
	}
	return " [" + m.ErrorKind + "]"
}

// typeCounts escribe un conteo por tipo en orden fijo: «multiple_choice:3 true_false:1».
func typeCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}

// histogramString escribe el reparto entero: «3×2 4×5» son dos trozos con 3 y cinco con 4.
func histogramString(h map[int]int) string {
	if len(h) == 0 {
		return "-"
	}
	keys := make([]int, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d×%d", k, h[k]))
	}
	return strings.Join(parts, " ")
}

// printMaterialTables imprime las cinco tablas de la ficha, una fila por caso. Sin
// veredicto en ninguna celda.
func printMaterialTables(runs []materialCaseRun) {
	fmt.Printf("\n-- Generación --\n")
	for _, r := range runs {
		g := r.Metrics.Generation
		fmt.Printf("%-16s trozos=%d fallidos=%d propuestas=%d por_trozo(min/med/max)=%d/%.1f/%d reparto=[%s] sin_abierta=%d descartadas=%d citan_texto=%d\n",
			trunc(r.ID, 16), g.Chunks, g.ChunksFailed, g.ProposedTotal,
			g.ProposedPerChunk.Min, g.ProposedPerChunk.Median, g.ProposedPerChunk.Max,
			histogramString(g.ProposedPerChunk.Histogram), g.ChunksWithoutOpenEnded, len(g.DiscardedByContract), g.DeicticStatements)
		fmt.Printf("%-16s   tipos: %s\n", "", typeCounts(g.ProposedTypes))
		for _, d := range g.DiscardedByContract {
			fmt.Printf("%-16s   descartada (trozo %d, %s): %s\n", "", d.ChunkSeq, d.Type, trunc(strings.Join(d.Issues, "; "), 120))
		}
	}

	fmt.Printf("\n-- Sustento (la respuesta correcta está en su trozo; indicio sin modelo) --\n")
	for _, r := range runs {
		s := r.Metrics.Support
		types := make([]string, 0, len(s.ByType))
		for t := range s.ByType {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			c := s.ByType[t]
			fmt.Printf("%-16s %-16s toda=%d a_medias=%d no=%d no_aplica=%d\n", trunc(r.ID, 16), t,
				c[supportFull], c[supportPartial], c[supportNone], c[supportNotApplicable])
		}
		fmt.Printf("%-16s source_ideas dadas=%d de %d · candidatas con todas dadas=%d de %d\n", trunc(r.ID, 16),
			s.SourceIdeasGiven, s.SourceIdeas, s.CandidatesAllTrue, s.Candidates)
	}

	fmt.Printf("\n-- Reducción --\n")
	for _, r := range runs {
		x := r.Metrics.Reduction
		fmt.Printf("%-16s pares(letras/coseno/juez)=%d/%d/%d grupos=%d caen_dup=%d | puntuadas=%d caen_relevancia=%d sin_puntuar=%d | caen_calidad=%d | final: %s\n",
			trunc(r.ID, 16), x.PairsText, x.PairsEmbed, x.PairsLLM, x.Clusters, x.DroppedDup,
			x.Scored, x.DroppedIrrelevant, x.Unscored, x.DroppedInvalid, typeCounts(x.FinalStatus))
	}

	fmt.Printf("\n-- Borrador --\n")
	for _, r := range runs {
		d := r.Metrics.Draft
		fmt.Printf("%-16s seleccionadas=%d de cupo %d · tipos: %s · ideas cubiertas=%d sin_cubrir=%d · secciones con pregunta=%d de %d\n",
			trunc(r.ID, 16), d.Selected, d.Target, typeCounts(d.SelectedTypes), d.IdeasCovered, d.IdeasUncovered,
			d.SectionsWithSelected, d.Sections)
	}

	fmt.Printf("\n-- Coste --\n")
	for _, r := range runs {
		c := r.Metrics.Cost
		fmt.Printf("%-16s llamadas: leer=%d preguntar=%d juez=%d relevancia=%d embeddings=%d(%d textos) · reintentos_calidad=%d · reentregas=%d\n",
			trunc(r.ID, 16), c.Digest.Calls, c.Propose.Calls, c.Judge.Calls, c.Relevance.Calls, c.Embed.Calls, c.Embed.Texts,
			c.Digest.QualityRetries+c.Propose.QualityRetries, c.Redeliveries)
		fmt.Printf("%-16s   segundos: fase0=%.1f fase1=%.1f duplicadas=%.1f relevancia=%.1f calidad=%.1f selección=%.1f total=%.1f · desenlace=%s\n", "",
			float64(c.Phase0MS)/1000, float64(c.Phase1MS)/1000, float64(c.DedupeMS)/1000, float64(c.RelevanceMS)/1000,
			float64(c.QualityMS)/1000, float64(c.SelectionMS)/1000, float64(c.TotalMS)/1000, r.Outcome)
		fmt.Printf("%-16s   salidas inválidas: leer=%d preguntar=%d juez=%d relevancia=%d · errores del proveedor: leer=%d preguntar=%d juez=%d relevancia=%d\n", "",
			c.Digest.InvalidOutputs, c.Propose.InvalidOutputs, c.Judge.InvalidOutputs, c.Relevance.InvalidOutputs,
			c.Digest.ProviderErrors, c.Propose.ProviderErrors, c.Judge.ProviderErrors, c.Relevance.ProviderErrors)
	}

	for _, r := range runs {
		if len(r.Metrics.Draft.Questions) == 0 {
			continue
		}
		fmt.Printf("\n-- Preguntas seleccionadas: %s --\n", r.ID)
		for i, q := range r.Metrics.Draft.Questions {
			fmt.Printf("%2d. [%s · trozo %d] %s\n", i+1, q.Type, q.ChunkSeq, q.Text)
		}
	}
}

// trunc corta un texto a n caracteres para las tablas (lo usa también embed.go).
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
