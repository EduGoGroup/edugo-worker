package main

// embed.go — modo "embed" del harness (plan 044 F1b): calibra el dedupe por embeddings
// ANTES de cablearlo (D-044.7 «medir antes de cablear»). Por cada modelo candidato
// vectoriza una batería de pares dup/no_dup en español, calcula el coseno de cada par y
// deriva los umbrales operativos del dedupe:
//
//   - dup_high: menor umbral con CERO falsos duplicados (ningún no_dup lo alcanza). Por
//     encima → se declara duplicado directo sin gastar LLM.
//   - dup_low: mayor umbral con CERO duplicados perdidos (ningún dup queda debajo). Por
//     debajo → se descarta como distinto sin gastar LLM.
//   - zona gris [dup_low, dup_high): los pares que ningún umbral resuelve solo → costo LLM
//     residual del carril.
//
// El contrato (§5) fija que el dedupe embebe question_text PURO. Como no se sabe aún si la
// pasada 1 (F1c) normalizará antes de embeder, se mide en DOS variantes por par —texto
// crudo y texto pasado por textmatch.Normalize (minúsculas + sin tildes, «ñ» preservada)—
// y se recomienda con datos, no en papel. La batería marca aparte los no_dup «respuesta
// distinta»: texto casi idéntico pero respuesta divergente, la prueba de fuego de embeder
// solo el enunciado.
//
// No instala nada ni asume Ollama corriendo: si el backend no responde, reporta el error y
// termina con código != 0. Un modelo a la vez (secuencial), batch por llamada.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/textmatch"
	"github.com/EduGoGroup/edugo-worker/internal/llm/ollama"
)

// defaultEmbedPairs es la batería por defecto del modo embed (relativa a la raíz del worker,
// que es donde se corre `go run ./cmd/llm-harness`).
const defaultEmbedPairs = "cmd/llm-harness/testdata/embed/pairs.json"

// embedBatchSize es cuántos textos se mandan por llamada a /api/embed. Los modelos de
// embeddings son chicos y la batería también; un batch holgado evita N llamadas HTTP.
const embedBatchSize = 32

// embedPair es una fila de la batería: dos enunciados y su veredicto humano.
type embedPair struct {
	ID         string `json:"id"`
	A          string `json:"a"`
	B          string `json:"b"`
	Label      string `json:"label"` // "dup" | "no_dup"
	Difficulty string `json:"difficulty"`
	Source     string `json:"source"`
	Note       string `json:"note"`
}

// isDup indica si el par es un duplicado según el veredicto humano.
func (p embedPair) isDup() bool { return p.Label == "dup" }

// answerDistinct reconoce la familia adversaria de no_dup: enunciado casi calcado pero
// respuesta/regla/zona/condición divergente (la nota lo marca con «…distinta/distintas»).
// Son el caso límite de embeder question_text puro y se reportan aparte porque marcan el
// techo de lo que el coseno solo del enunciado puede discriminar: al ser texto casi igual,
// el modelo los puntúa alto aunque NO sean duplicados.
func (p embedPair) answerDistinct() bool {
	return !p.isDup() && strings.Contains(strings.ToLower(p.Note), "distint")
}

// pairCosine guarda el coseno de un par en las dos variantes de texto.
type pairCosine struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Difficulty string  `json:"difficulty"`
	Raw        float64 `json:"cos_raw"`        // coseno de a vs b crudos
	Normalized float64 `json:"cos_normalized"` // coseno de Normalize(a) vs Normalize(b)
	AnswerDist bool    `json:"answer_distinct,omitempty"`
}

// thresholds son los umbrales derivados de una variante (raw o normalized) de una corrida.
type thresholds struct {
	Variant string `json:"variant"`

	DupHigh        float64 `json:"dup_high"`          // = max coseno no_dup (umbral > esto ⇒ 0 falsos dup)
	DupsCapturedHi int     `json:"dups_captured_hi"`  // dup con coseno > DupHigh (capturados gratis)
	DupLow         float64 `json:"dup_low"`           // = min coseno dup (umbral ≤ esto ⇒ 0 dup perdidos)
	NoDupDroppedLo int     `json:"no_dup_dropped_lo"` // no_dup con coseno < DupLow (descartados gratis)

	GrayCount int     `json:"gray_count"`  // pares en [DupLow, DupHigh)
	GrayDup   int     `json:"gray_dup"`    // de esos, cuántos dup
	GrayNoDup int     `json:"gray_no_dup"` // de esos, cuántos no_dup
	GrayPct   float64 `json:"gray_pct"`    // GrayCount / total
	Overlap   bool    `json:"overlap"`     // ¿el mejor no_dup ≥ el peor dup?
	Separates bool    `json:"separates"`   // ¿un umbral único separa 100%? (= !Overlap)
	WorstDup  float64 `json:"worst_dup"`   // min coseno dup (= DupLow)
	BestNoDup float64 `json:"best_no_dup"` // max coseno no_dup (= DupHigh)
}

// distStats es la distribución de cosenos de un grupo (label, difficulty).
type distStats struct {
	Group  string  `json:"group"`
	N      int     `json:"n"`
	Min    float64 `json:"min"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	Max    float64 `json:"max"`
}

// modelResult es todo lo medido para un modelo, serializado a results-<modelo>.json.
type modelResult struct {
	Model        string       `json:"model"`
	Pairs        int          `json:"pairs"`
	EmbedMS      int64        `json:"embed_ms"`
	Dim          int          `json:"dim"`
	RawStats     []distStats  `json:"raw_stats"`
	NormStats    []distStats  `json:"norm_stats"`
	RawThresh    thresholds   `json:"raw_thresholds"`
	NormThresh   thresholds   `json:"norm_thresholds"`
	AnswerDistID []string     `json:"answer_distinct_ids"`
	Cosines      []pairCosine `json:"cosines"`
}

// runEmbed carga la batería, corre cada modelo secuencial y reporta tabla + JSON, además de
// escribir results-<modelo>.json. Sale con código != 0 si un modelo no responde.
func runEmbed(ollamaURL, modelsCSV, pairsPath, outDir string, timeout time.Duration) {
	pairs, err := loadEmbedPairs(pairsPath)
	if err != nil {
		fatalf("cargando batería %q: %v", pairsPath, err)
	}
	models := splitCSV(modelsCSV)
	if len(models) == 0 {
		fatalf("no hay modelos para el modo embed (pasa -embed-models)")
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Dir(pairsPath)
	}

	nDup, nNoDup := countLabels(pairs)
	fmt.Printf("== llm-harness (embed) ==\n")
	fmt.Printf("batería : %s (%d pares: %d dup / %d no_dup)\n", pairsPath, len(pairs), nDup, nNoDup)
	fmt.Printf("modelos : %s\n\n", strings.Join(models, ", "))

	results := make([]modelResult, 0, len(models))
	for _, model := range models {
		res, err := runEmbedModel(ollamaURL, model, pairs, timeout)
		if err != nil {
			fatalf("modelo %q: %v", model, err)
		}
		printEmbedModel(res)
		if err := writeModelResult(outDir, res); err != nil {
			fmt.Printf("  aviso: no se pudo escribir results-%s.json: %v\n", sanitizeModel(model), err)
		} else {
			fmt.Printf("  → %s\n", filepath.Join(outDir, "results-"+sanitizeModel(model)+".json"))
		}
		results = append(results, res)
	}

	printEmbedComparison(results)
}

// runEmbedModel vectoriza la batería con un modelo y computa todo. Deduplica textos
// idénticos para no re-embeder, batchea las llamadas y valida la dimensión.
func runEmbedModel(ollamaURL, model string, pairs []embedPair, timeout time.Duration) (modelResult, error) {
	embedder := ollama.NewEmbedder(ollama.EmbedConfig{BaseURL: ollamaURL, Model: model, Timeout: timeout})

	// Junta todos los textos únicos (crudos y normalizados) en un solo diccionario.
	need := map[string]struct{}{}
	for _, p := range pairs {
		need[p.A] = struct{}{}
		need[p.B] = struct{}{}
		need[textmatch.Normalize(p.A)] = struct{}{}
		need[textmatch.Normalize(p.B)] = struct{}{}
	}
	texts := make([]string, 0, len(need))
	for t := range need {
		texts = append(texts, t)
	}
	sort.Strings(texts) // orden estable → salida reproducible

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	vecs := map[string][]float32{}
	dim := 0
	for i := 0; i < len(texts); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		out, err := embedder.Embed(ctx, batch)
		if err != nil {
			return modelResult{}, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		for j, v := range out {
			vecs[batch[j]] = v
			if dim == 0 {
				dim = len(v)
			}
		}
	}
	embedMS := time.Since(start).Milliseconds()

	cosines := make([]pairCosine, 0, len(pairs))
	var answerDistIDs []string
	for _, p := range pairs {
		pc := pairCosine{
			ID:         p.ID,
			Label:      p.Label,
			Difficulty: p.Difficulty,
			Raw:        cosine(vecs[p.A], vecs[p.B]),
			Normalized: cosine(vecs[textmatch.Normalize(p.A)], vecs[textmatch.Normalize(p.B)]),
			AnswerDist: p.answerDistinct(),
		}
		if p.answerDistinct() {
			answerDistIDs = append(answerDistIDs, p.ID)
		}
		cosines = append(cosines, pc)
	}

	return modelResult{
		Model:        model,
		Pairs:        len(pairs),
		EmbedMS:      embedMS,
		Dim:          dim,
		RawStats:     groupStats(cosines, false),
		NormStats:    groupStats(cosines, true),
		RawThresh:    deriveThresholds("raw", cosines, false),
		NormThresh:   deriveThresholds("normalized", cosines, true),
		AnswerDistID: answerDistIDs,
		Cosines:      cosines,
	}, nil
}

// deriveThresholds calcula dup_high/dup_low, la zona gris y la separación para una variante.
func deriveThresholds(variant string, cosines []pairCosine, normalized bool) thresholds {
	t := thresholds{Variant: variant}
	minDup, maxNoDup := math.Inf(1), math.Inf(-1)
	hasDup, hasNoDup := false, false
	for _, c := range cosines {
		v := c.Raw
		if normalized {
			v = c.Normalized
		}
		if c.Label == "dup" {
			hasDup = true
			if v < minDup {
				minDup = v
			}
		} else {
			hasNoDup = true
			if v > maxNoDup {
				maxNoDup = v
			}
		}
	}
	if !hasDup || !hasNoDup {
		return t
	}

	// dup_high = mejor no_dup: cualquier umbral por ENCIMA declara dup sin falsos positivos.
	// dup_low  = peor dup: cualquier umbral en o por DEBAJO conserva todos los dup.
	t.BestNoDup, t.WorstDup = maxNoDup, minDup
	t.DupHigh, t.DupLow = maxNoDup, minDup

	for _, c := range cosines {
		v := c.Raw
		if normalized {
			v = c.Normalized
		}
		if c.Label == "dup" && v > t.DupHigh {
			t.DupsCapturedHi++ // dup capturado gratis por encima de todo no_dup
		}
		if c.Label == "no_dup" && v < t.DupLow {
			t.NoDupDroppedLo++ // no_dup descartado gratis por debajo de todo dup
		}
		// Zona gris [dup_low, dup_high): ni umbral la resuelve → iría a LLM.
		if v >= t.DupLow && v < t.DupHigh {
			t.GrayCount++
			if c.Label == "dup" {
				t.GrayDup++
			} else {
				t.GrayNoDup++
			}
		}
	}
	if len(cosines) > 0 {
		t.GrayPct = 100 * float64(t.GrayCount) / float64(len(cosines))
	}
	// Overlap ⇔ el mejor no_dup alcanza o supera al peor dup: no hay umbral único que
	// separe 100%. Si el peor dup supera al mejor no_dup, un solo corte separa perfecto.
	t.Overlap = maxNoDup >= minDup
	t.Separates = !t.Overlap
	return t
}

// groupStats calcula la distribución por (label, difficulty) para una variante.
func groupStats(cosines []pairCosine, normalized bool) []distStats {
	order := []string{"dup/easy", "dup/hard", "no_dup/easy", "no_dup/hard"}
	buckets := map[string][]float64{}
	for _, c := range cosines {
		key := c.Label + "/" + c.Difficulty
		v := c.Raw
		if normalized {
			v = c.Normalized
		}
		buckets[key] = append(buckets[key], v)
	}
	var out []distStats
	for _, key := range order {
		vals := buckets[key]
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		out = append(out, distStats{
			Group:  key,
			N:      len(vals),
			Min:    vals[0],
			P25:    quantile(vals, 0.25),
			Median: quantile(vals, 0.50),
			P75:    quantile(vals, 0.75),
			Max:    vals[len(vals)-1],
		})
	}
	return out
}

// --- salida ---

func printEmbedModel(r modelResult) {
	fmt.Printf("── modelo: %s (dim %d, %d ms para %d pares) ──\n", r.Model, r.Dim, r.EmbedMS, r.Pairs)

	fmt.Printf("  distribución de cosenos (raw / normalized):\n")
	fmt.Printf("  %-13s %-4s %-13s %-13s %-13s %-13s %-13s\n", "GRUPO", "N", "MIN", "P25", "MEDIANA", "P75", "MAX")
	for i := range r.RawStats {
		rs, ns := r.RawStats[i], r.NormStats[i]
		fmt.Printf("  %-13s %-4d %-13s %-13s %-13s %-13s %-13s\n",
			rs.Group, rs.N,
			pair2(rs.Min, ns.Min), pair2(rs.P25, ns.P25), pair2(rs.Median, ns.Median),
			pair2(rs.P75, ns.P75), pair2(rs.Max, ns.Max))
	}

	fmt.Printf("\n  umbrales (variante · dup_high · dup_low · zona gris · overlap):\n")
	printThreshRow(r.RawThresh, r.Pairs)
	printThreshRow(r.NormThresh, r.Pairs)

	if len(r.AnswerDistID) > 0 {
		fmt.Printf("\n  no_dup «respuesta distinta» (prueba de fuego, %d pares): coseno raw / norm\n", len(r.AnswerDistID))
		set := map[string]bool{}
		for _, id := range r.AnswerDistID {
			set[id] = true
		}
		for _, c := range r.Cosines {
			if set[c.ID] {
				fmt.Printf("    %-6s %s   (%s)\n", c.ID, pair2(c.Raw, c.Normalized), c.Difficulty)
			}
		}
	}
	fmt.Println()
}

func printThreshRow(t thresholds, total int) {
	if t.DupHigh == 0 && t.DupLow == 0 {
		fmt.Printf("  %-11s (sin datos)\n", t.Variant)
		return
	}
	sep := "overlap"
	if t.Separates {
		sep = "SEPARA 100%"
	}
	fmt.Printf("  %-11s high=%.4f (dup≥ gratis: %d)  low=%.4f (no_dup< gratis: %d)  gris=%d/%d (%.0f%%: %ddup/%dnd)  %s\n",
		t.Variant, t.DupHigh, t.DupsCapturedHi, t.DupLow, t.NoDupDroppedLo,
		t.GrayCount, total, t.GrayPct, t.GrayDup, t.GrayNoDup, sep)
}

// printEmbedComparison imprime la tabla comparativa final (una fila por modelo·variante).
func printEmbedComparison(results []modelResult) {
	fmt.Printf("== comparación (una fila por modelo·variante) ==\n")
	fmt.Printf("%-18s %-11s %-9s %-9s %-11s %-9s %s\n",
		"MODELO", "VARIANTE", "DUP_HIGH", "DUP_LOW", "GRIS n/%", "OVERLAP", "COMPOSICIÓN GRIS")
	for _, r := range results {
		for _, t := range []thresholds{r.RawThresh, r.NormThresh} {
			ov := "sí"
			if t.Separates {
				ov = "no"
			}
			fmt.Printf("%-18s %-11s %-9.4f %-9.4f %-11s %-9s %ddup/%dnd\n",
				trunc(r.Model, 18), t.Variant, t.DupHigh, t.DupLow,
				fmt.Sprintf("%d/%.0f%%", t.GrayCount, t.GrayPct), ov, t.GrayDup, t.GrayNoDup)
		}
	}
}

// writeModelResult escribe results-<modelo>.json en outDir.
func writeModelResult(outDir string, r modelResult) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(outDir, "results-"+sanitizeModel(r.Model)+".json")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// --- helpers ---

// cosine calcula la similitud coseno de dos vectores. Devuelve 0 si alguno está vacío o es
// nulo (defensa: un texto sin vector no debe reventar el reporte).
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// quantile devuelve el cuantil q ∈ [0,1] de un slice YA ordenado, con interpolación lineal.
func quantile(sorted []float64, q float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	pos := q * float64(n-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// pair2 formatea dos cosenos como "raw/norm" para las tablas comparativas.
func pair2(raw, norm float64) string {
	return fmt.Sprintf("%.4f/%.4f", raw, norm)
}

// loadEmbedPairs lee y parsea la batería de pares.
func loadEmbedPairs(path string) ([]embedPair, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pairs []embedPair
	if err := json.Unmarshal(b, &pairs); err != nil {
		return nil, fmt.Errorf("parseando JSON: %w", err)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("batería vacía")
	}
	return pairs, nil
}

func countLabels(pairs []embedPair) (dup, noDup int) {
	for _, p := range pairs {
		if p.isDup() {
			dup++
		} else {
			noDup++
		}
	}
	return dup, noDup
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// sanitizeModel convierte un nombre de modelo en un slug apto para nombre de archivo
// (ej. "nomic-embed-text:latest" → "nomic-embed-text_latest").
func sanitizeModel(model string) string {
	repl := strings.NewReplacer(":", "_", "/", "_")
	return repl.Replace(model)
}
