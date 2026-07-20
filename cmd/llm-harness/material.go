package main

// material.go — modo "material" del harness (plan 043 F3b): mide las DOS llamadas del
// pipeline material→evaluación sobre documentos reales de testdata.
//
//   - Batería A (digest): por cada trozo corre DigestChunk ("leer") encadenando el
//     summary del trozo anterior, y valida los artefactos contra ChunkArtifactsV1.
//   - Batería B (candidatas): con las ideas de A corre ProposeCandidates ("preguntar")
//     y valida cada candidata contra CandidatePayloadV1.
//
// El flujo espeja al processor de F3c: extrae texto (pdf.Extractor para .pdf, lectura
// directa para .txt), porciona con la Config de los flags -chunk-* (default = la config
// productiva 300/400/200/80, no chunking.DefaultConfig) y encadena A→B por trozo. Mide
// —no en papel— para elegir modelo y variante de prompt (-digest-prompt v1|v2). Reporta
// tabla + JSON; no corre nada si Ollama no responde (reporta el error del provider).
//
// NOTA (hallazgo F3b): el extractor de PDF (internal/infrastructure/pdf) devuelve hoy el
// content-stream crudo (operadores + texto entre paréntesis) y además rechaza PDFs de una
// sola página porque no invoca EnsurePageCount antes de leer PageCount. Por eso los .txt
// hermanos son la entrada por defecto de la medición; los .pdf de testdata ejercitan el
// camino pdf.Extractor y quedan listos para cuando el dueño de ese paquete lo corrija.

import (
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
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// maxSummaryWords es el tope de palabras del summary encadenable (D-043.7): un resumen
// escrito para otro LLM debe ser mínimo en tokens. Por encima cuenta como "no OK".
const maxSummaryWords = 120

// defaultMaterialDir es la carpeta de testdata usada cuando no se pasa -material-inputs.
const defaultMaterialDir = "cmd/llm-harness/testdata/material"

// materialChunkMetric son las métricas de correr A+B sobre UN trozo (o de un input que
// ni siquiera pudo cargarse: ChunkSeq = -1 y LoadErr poblado).
type materialChunkMetric struct {
	Input      string `json:"input"`
	ChunkSeq   int    `json:"chunk_seq"`
	ChunkWords int    `json:"chunk_words"`
	LoadErr    string `json:"load_err,omitempty"`

	// Batería A (digest).
	DigestMS        int64    `json:"digest_ms"`
	DigestErr       string   `json:"digest_err,omitempty"`
	ArtifactsValid  bool     `json:"artifacts_valid"`
	ArtifactsIssues []string `json:"artifacts_issues,omitempty"`
	MainIdeas       int      `json:"main_ideas"`
	SecondaryIdeas  int      `json:"secondary_ideas"`
	SummaryWords    int      `json:"summary_words"`
	SummaryOK       bool     `json:"summary_ok"`

	// Batería B (candidatas).
	BSkipped    bool   `json:"b_skipped,omitempty"`
	ProposeMS   int64  `json:"propose_ms"`
	ProposeErr  string `json:"propose_err,omitempty"`
	CandTotal   int    `json:"cand_total"`
	CandValid   int    `json:"cand_valid"`
	CandInRange bool   `json:"cand_in_range"`
	// CandDeictic cuenta candidatas cuyo enunciado referencia el contexto del prompt
	// («según las ideas», «según el texto»…): no autocontenidas (deuda 043). Se mide con
	// el detector determinista de materialpipeline, independiente de la validez de forma.
	CandDeictic int              `json:"cand_deictic"`
	TypeCounts  map[string]int   `json:"type_counts,omitempty"`
	CandIssues  []candidateIssue `json:"cand_issues,omitempty"`

	summaryText string // no exportado: se encadena al trozo siguiente
}

// candidateIssue registra por qué una candidata puntual no validó (D-043.5): su índice
// en la tanda, su tipo declarado y los Issue textuales del *ValidationError. Se usa para
// diagnosticar sesgos sistemáticos del prompt B (F3b, medición).
type candidateIssue struct {
	Index        int      `json:"index"`
	QuestionType string   `json:"question_type"`
	Issues       []string `json:"issues"`
}

// materialInputs resuelve la lista de entradas: la lista explícita (coma-separada) o,
// si viene vacía, todos los .txt de la carpeta de testdata por defecto.
func materialInputs(explicit string) ([]string, error) {
	if strings.TrimSpace(explicit) != "" {
		var out []string
		for p := range strings.SplitSeq(explicit, ",") {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	}
	entries, err := os.ReadDir(defaultMaterialDir)
	if err != nil {
		return nil, fmt.Errorf("leyendo carpeta de testdata %q: %w", defaultMaterialDir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
			out = append(out, filepath.Join(defaultMaterialDir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// loadMaterialText carga el texto de un input: .pdf pasa por pdf.Extractor (camino real
// del processor); cualquier otra extensión se lee como texto plano.
func loadMaterialText(path string, log logger.Logger) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".pdf") {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer func() { _ = f.Close() }()
		return pdf.NewExtractor(log).Extract(context.Background(), f)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// materialOptions agrupa los parámetros del modo material. El porcionado es
// configurable por flags (default = la config productiva de internal/config, no el
// DefaultConfig del paquete chunking) para que el harness mida lo que corre en vivo.
type materialOptions struct {
	timeout   time.Duration
	inputsCSV string
	chunkCfg  chunking.Config
	// skipPropose omite la Batería B: para experimentos que solo miden el digest A
	// ahorra la mitad de la corrida.
	skipPropose bool
	// digestVariant elige el prompt de la llamada A: "v2" = tarea partida (la ruta
	// productiva del provider ollama desde el experimento 2026-07-19; default) o
	// "v1" = llamada única legacy para regresión (vive en material_v1.go, solo local).
	digestVariant string
	provider      string
	ollamaURL     string
	ollamaModel   string
}

// runMaterial corre las baterías A y B sobre las entradas y reporta tabla + JSON.
func runMaterial(p llm.LLMProvider, opts materialOptions) {
	inputs, err := materialInputs(opts.inputsCSV)
	if err != nil {
		fatalf("resolviendo entradas: %v", err)
	}
	if len(inputs) == 0 {
		fatalf("no hay entradas para el modo material (pasa -material-inputs o coloca .txt en %s)", defaultMaterialDir)
	}
	if err := opts.chunkCfg.Validate(); err != nil {
		fatalf("config de chunking inválida: %v", err)
	}
	if opts.digestVariant != "v1" && opts.digestVariant != "v2" {
		fatalf("variante de digest desconocida %q (usa v1|v2)", opts.digestVariant)
	}
	// v1 legacy vive en el harness contra Ollama; con provider api el modo material
	// corre siempre la ruta productiva del provider (que en api sigue siendo la
	// llamada única — la partición v2 se cableó solo en ollama, donde se midió).
	if opts.digestVariant == "v1" && opts.provider != "local" && opts.provider != "ollama" {
		fatalf("-digest-prompt=v1 (llamada única legacy) solo está implementada para el provider local/ollama")
	}

	log := logger.NewZapLogger("error", "console")

	fmt.Printf("== llm-harness (material) ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("entradas : %d\n", len(inputs))
	fmt.Printf("chunking : target=%d max=%d min=%d merge=%d\n", opts.chunkCfg.TargetWords, opts.chunkCfg.MaxWords, opts.chunkCfg.MinWords, opts.chunkCfg.MergeThresholdWords)
	fmt.Printf("digest   : %s%s\n\n", opts.digestVariant, map[bool]string{true: " (Batería B omitida)", false: ""}[opts.skipPropose])

	var metrics []materialChunkMetric
	for _, in := range inputs {
		name := filepath.Base(in)
		text, err := loadMaterialText(in, log)
		if err != nil {
			fmt.Printf("  %-24s FALLO carga/extracción: %v\n", name, err)
			metrics = append(metrics, materialChunkMetric{Input: name, ChunkSeq: -1, LoadErr: err.Error()})
			continue
		}
		chunks := chunking.Split(text, opts.chunkCfg)
		fmt.Printf("  %-24s %d bytes → %d trozos\n", name, len(text), len(chunks))

		var prevSummary *string
		for _, ch := range chunks {
			m := runMaterialChunk(p, opts, name, ch, prevSummary)
			metrics = append(metrics, m)
			if strings.TrimSpace(m.summaryText) != "" {
				s := m.summaryText
				prevSummary = &s
			}
		}
	}

	fmt.Println()
	printMaterialTable(metrics)
	printMaterialJSON(p, metrics)
}

// runMaterialChunk corre A y —si A dio ideas— B sobre un trozo. La medición NO reintenta
// (a diferencia de los modos prep/review): F3b quiere ver la tasa cruda del modelo por
// llamada, no la tasa tras reintentos.
func runMaterialChunk(p llm.LLMProvider, opts materialOptions, input string, ch chunking.Chunk, prevSummary *string) materialChunkMetric {
	timeout := opts.timeout
	m := materialChunkMetric{
		Input:      input,
		ChunkSeq:   ch.Seq,
		ChunkWords: len(strings.Fields(ch.Text)),
		TypeCounts: map[string]int{},
	}

	// --- Batería A: DigestChunk (v2 = ruta productiva del provider, partida en dos;
	// v1 = llamada única legacy para regresión, material_v1.go) ---
	var digest *llm.DigestChunkResult
	var errA error
	startA := time.Now()
	if opts.digestVariant == "v1" {
		digest, errA = digestChunkV1(opts, ch.Text, prevSummary)
	} else {
		ctxA, cancelA := context.WithTimeout(context.Background(), timeout)
		digest, errA = p.DigestChunk(ctxA, llm.DigestChunkInput{
			ChunkText:   ch.Text,
			PrevSummary: prevSummary,
			Language:    "es",
		})
		cancelA()
	}
	m.DigestMS = time.Since(startA).Milliseconds()

	if errA != nil {
		m.DigestErr = errA.Error()
		return m // sin artefactos no hay B
	}

	m.MainIdeas = len(digest.Artifacts.MainIdeas)
	m.SecondaryIdeas = len(digest.Artifacts.SecondaryIdeas)
	m.summaryText = digest.Summary
	m.SummaryWords = len(strings.Fields(digest.Summary))
	m.SummaryOK = m.SummaryWords > 0 && m.SummaryWords <= maxSummaryWords

	rawArtifacts, _ := digest.Artifacts.Marshal()
	if _, verr := materialpipeline.ValidateChunkArtifacts(rawArtifacts); verr != nil {
		m.ArtifactsIssues = issueStrings(verr)
	} else {
		m.ArtifactsValid = true
	}

	// --- Batería B: ProposeCandidates (solo si A dejó ideas utilizables) ---
	if opts.skipPropose {
		m.BSkipped = true
		return m
	}
	if m.MainIdeas == 0 {
		m.ProposeErr = "A no produjo main_ideas; B omitida"
		return m
	}

	ctxB, cancelB := context.WithTimeout(context.Background(), timeout)
	startB := time.Now()
	candidates, errB := p.ProposeCandidates(ctxB, llm.ProposeCandidatesInput{
		Artifacts: digest.Artifacts,
		Language:  "es",
	})
	m.ProposeMS = time.Since(startB).Milliseconds()
	cancelB()

	if errB != nil {
		m.ProposeErr = errB.Error()
		return m
	}

	m.CandTotal = len(candidates)
	m.CandInRange = m.CandTotal >= 2 && m.CandTotal <= 4
	for i, c := range candidates {
		m.TypeCounts[c.QuestionType]++
		if materialpipeline.DetectDeicticReference(c.QuestionText) != "" {
			m.CandDeictic++
		}
		rawCand, _ := c.Marshal()
		if _, verr := materialpipeline.ValidateCandidatePayload(rawCand); verr == nil {
			m.CandValid++
		} else {
			m.CandIssues = append(m.CandIssues, candidateIssue{
				Index:        i,
				QuestionType: c.QuestionType,
				Issues:       issueStrings(verr),
			})
		}
	}
	return m
}

// issueStrings aplana los Issue de un *materialpipeline.ValidationError a texto.
func issueStrings(err error) []string {
	ve, ok := err.(*materialpipeline.ValidationError)
	if !ok {
		return []string{err.Error()}
	}
	out := make([]string, len(ve.Issues))
	for i, iss := range ve.Issues {
		out[i] = iss.String()
	}
	return out
}

// printMaterialTable imprime una fila por trozo con las métricas de A y B.
func printMaterialTable(metrics []materialChunkMetric) {
	fmt.Printf("%-22s %-5s %-6s %-7s %-5s %-9s %-7s %-5s %-4s %-6s %-5s %s\n",
		"INPUT", "TROZO", "PALS", "A(ms)", "ARTF", "SUM(pal)", "B(ms)", "CAND", "2-4", "VÁLID", "DEÍC", "TIPOS")
	for _, m := range metrics {
		if m.ChunkSeq < 0 {
			fmt.Printf("%-22s %-5s %s\n", trunc(m.Input, 22), "-", "FALLO carga: "+m.LoadErr)
			continue
		}
		artf := okFail(m.ArtifactsValid)
		if m.DigestErr != "" {
			artf = "ERR"
		}
		sum := fmt.Sprintf("%d%s", m.SummaryWords, tick(m.SummaryOK))
		b := fmt.Sprintf("%d", m.ProposeMS)
		cand, rng, valid, deic, types := "-", "-", "-", "-", ""
		if m.DigestErr == "" && m.ProposeErr == "" && m.MainIdeas > 0 && !m.BSkipped {
			cand = fmt.Sprintf("%d", m.CandTotal)
			rng = okFail(m.CandInRange)
			valid = fmt.Sprintf("%d/%d", m.CandValid, m.CandTotal)
			deic = fmt.Sprintf("%d", m.CandDeictic)
			types = typeSummary(m.TypeCounts)
		} else if m.ProposeErr != "" {
			cand = "ERR"
		}
		fmt.Printf("%-22s %-5d %-6d %-7d %-5s %-9s %-7s %-5s %-4s %-6s %-5s %s\n",
			trunc(m.Input, 22), m.ChunkSeq, m.ChunkWords, m.DigestMS, artf, sum, b, cand, rng, valid, deic, types)
		if m.DigestErr != "" {
			fmt.Printf("      A error: %s\n", trunc(m.DigestErr, 100))
		}
		if len(m.ArtifactsIssues) > 0 {
			fmt.Printf("      A inválida: %s\n", strings.Join(m.ArtifactsIssues, "; "))
		}
		if m.ProposeErr != "" && m.MainIdeas > 0 {
			fmt.Printf("      B error: %s\n", trunc(m.ProposeErr, 100))
		}
		for _, ci := range m.CandIssues {
			fmt.Printf("      B candidata #%d inválida [%s]: %s\n", ci.Index, ci.QuestionType, strings.Join(ci.Issues, "; "))
		}
	}
}

// printMaterialJSON imprime el resumen agregado de la corrida como JSON (para diffs y
// para comparar modelos entre corridas).
func printMaterialJSON(p llm.LLMProvider, metrics []materialChunkMetric) {
	agg := aggregateMaterial(p, metrics)
	fmt.Printf("\n--- resumen ---\n")
	fmt.Printf("trozos procesados        : %d\n", agg.Chunks)
	fmt.Printf("artefactos A válidos     : %d/%d (%s)\n", agg.ArtifactsValid, agg.Chunks, pct(agg.ArtifactsValid, agg.Chunks))
	fmt.Printf("summaries ≤%d palabras   : %d/%d (%s)\n", maxSummaryWords, agg.SummariesOK, agg.Chunks, pct(agg.SummariesOK, agg.Chunks))
	fmt.Printf("candidatas B válidas     : %d/%d (%s)\n", agg.CandValid, agg.CandTotal, pct(agg.CandValid, agg.CandTotal))
	fmt.Printf("candidatas B deícticas   : %d/%d (%s)\n", agg.CandDeictic, agg.CandTotal, pct(agg.CandDeictic, agg.CandTotal))
	fmt.Printf("trozos con 2–4 candidatas: %d/%d (%s)\n", agg.ChunksCandInRange, agg.ChunksWithB, pct(agg.ChunksCandInRange, agg.ChunksWithB))
	fmt.Printf("distribución de tipos    : %s\n", typeSummary(agg.TypeCounts))
	fmt.Printf("latencia A (prom/total)  : %d ms / %d ms\n", avg(agg.DigestMSTotal, agg.Chunks), agg.DigestMSTotal)
	fmt.Printf("latencia B (prom/total)  : %d ms / %d ms\n", avg(agg.ProposeMSTotal, agg.ChunksWithB), agg.ProposeMSTotal)

	out := struct {
		Aggregate materialAggregate     `json:"aggregate"`
		Chunks    []materialChunkMetric `json:"chunks"`
	}{Aggregate: agg, Chunks: metrics}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	fmt.Printf("\n--- JSON ---\n%s\n", string(b))
}

// materialAggregate agrega la corrida. Se serializa dentro del JSON de salida.
type materialAggregate struct {
	Provider          string         `json:"provider"`
	Chunks            int            `json:"chunks"`
	ArtifactsValid    int            `json:"artifacts_valid"`
	SummariesOK       int            `json:"summaries_ok"`
	ChunksWithB       int            `json:"chunks_with_b"`
	ChunksCandInRange int            `json:"chunks_cand_in_range"`
	CandTotal         int            `json:"cand_total"`
	CandValid         int            `json:"cand_valid"`
	CandDeictic       int            `json:"cand_deictic"`
	TypeCounts        map[string]int `json:"type_counts"`
	DigestMSTotal     int64          `json:"digest_ms_total"`
	ProposeMSTotal    int64          `json:"propose_ms_total"`
}

func aggregateMaterial(p llm.LLMProvider, metrics []materialChunkMetric) materialAggregate {
	agg := materialAggregate{Provider: p.Name(), TypeCounts: map[string]int{}}
	for _, m := range metrics {
		if m.ChunkSeq < 0 {
			continue
		}
		agg.Chunks++
		agg.DigestMSTotal += m.DigestMS
		if m.ArtifactsValid {
			agg.ArtifactsValid++
		}
		if m.SummaryOK {
			agg.SummariesOK++
		}
		if m.DigestErr == "" && m.ProposeErr == "" && m.MainIdeas > 0 && !m.BSkipped {
			agg.ChunksWithB++
			agg.ProposeMSTotal += m.ProposeMS
			agg.CandTotal += m.CandTotal
			agg.CandValid += m.CandValid
			agg.CandDeictic += m.CandDeictic
			if m.CandInRange {
				agg.ChunksCandInRange++
			}
			for t, n := range m.TypeCounts {
				agg.TypeCounts[t] += n
			}
		}
	}
	return agg
}

// --- helpers de formato ---

func typeSummary(counts map[string]int) string {
	if len(counts) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", k, counts[k])
	}
	return strings.Join(parts, " ")
}

func okFail(ok bool) string {
	if ok {
		return "OK"
	}
	return "FAIL"
}

func tick(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func pct(n, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(total))
}

func avg(total int64, n int) int64 {
	if n == 0 {
		return 0
	}
	return total / int64(n)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
