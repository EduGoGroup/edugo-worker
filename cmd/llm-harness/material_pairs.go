package main

// material_pairs.go — modo "material-pairs" (AR-IA.3): la primera medida del juez de
// duplicadas. Pasa parejas ETIQUETADAS de preguntas por la pasada de duplicadas real
// (reduce.DedupePass), una pareja por vez, sobre un almacén en memoria con solo esas
// dos candidatas. Cada pareja se mide dos veces:
//
//   - "real": con el embedder de producción. Dice qué escalón decide (letras, coseno
//     o juez) y qué decide.
//   - "judge_only": con un embedder fijo en la zona gris. Toda pareja que las letras
//     no deciden llega al juez: mide al juez solo.
//
// Una corrida por pareja y por medida, sin repetir. Un error del juez es un dato.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
)

// defaultMaterialPairs es la batería de parejas, relativa a la raíz del worker.
const defaultMaterialPairs = defaultMaterialDir + "/pairs.json"

const (
	pairLabelDup   = "dup"
	pairLabelNoDup = "no_dup"

	pairTierText  = "text"
	pairTierEmbed = "embed"
	pairTierLLM   = "llm"

	pairModeReal      = "real"
	pairModeJudgeOnly = "judge_only"
)

// materialPair es una pareja etiquetada: dos candidatas completas y si son la misma
// pregunta.
type materialPair struct {
	ID       string          `json:"id"`
	Category string          `json:"category"`
	Label    string          `json:"label"`
	Note     string          `json:"note,omitempty"`
	A        json.RawMessage `json:"a"`
	B        json.RawMessage `json:"b"`
}

// pairsFile es el fichero de parejas.
type pairsFile struct {
	Categories []string       `json:"categories"`
	Pairs      []materialPair `json:"pairs"`
}

// loadMaterialPairs lee y valida la batería: ids únicos, etiqueta dup|no_dup,
// categoría declarada y los dos miembros válidos contra el contrato de candidata
// (la pasada de duplicadas omite en silencio lo que no parsea).
func loadMaterialPairs(path string) (pairsFile, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return pairsFile{}, "", fmt.Errorf("leyendo las parejas %q: %w", path, err)
	}
	var f pairsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return pairsFile{}, "", fmt.Errorf("las parejas %q no parsean: %w", path, err)
	}
	if len(f.Pairs) == 0 {
		return pairsFile{}, "", fmt.Errorf("las parejas %q están vacías", path)
	}
	categories := map[string]struct{}{}
	for _, c := range f.Categories {
		categories[c] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, p := range f.Pairs {
		if p.ID == "" {
			return pairsFile{}, "", fmt.Errorf("hay una pareja sin id en %q", path)
		}
		if _, dup := seen[p.ID]; dup {
			return pairsFile{}, "", fmt.Errorf("id de pareja repetido: %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		if p.Label != pairLabelDup && p.Label != pairLabelNoDup {
			return pairsFile{}, "", fmt.Errorf("pareja %q: etiqueta %q (se espera dup o no_dup)", p.ID, p.Label)
		}
		if _, ok := categories[p.Category]; !ok {
			return pairsFile{}, "", fmt.Errorf("pareja %q: categoría %q no declarada", p.ID, p.Category)
		}
		for name, member := range map[string]json.RawMessage{"a": p.A, "b": p.B} {
			if _, verr := materialpipeline.ValidateCandidatePayload(member); verr != nil {
				return pairsFile{}, "", fmt.Errorf("pareja %q, miembro %s: %w", p.ID, name, verr)
			}
		}
	}
	return f, fileSHA256(raw), nil
}

// pairMeasure es UNA medida de una pareja.
type pairMeasure struct {
	// Tier es el escalón que decidió: text, embed o llm. Vacío si la medida no llegó
	// a decidir (el embedder falló).
	Tier string `json:"tier,omitempty"`
	// Verdict es lo que decidió la pasada: dup o no_dup. Vacío si hubo error.
	Verdict   string `json:"verdict,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
	Error     string `json:"error,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// pairRun es una pareja con sus dos medidas.
type pairRun struct {
	ID        string      `json:"id"`
	Category  string      `json:"category"`
	Label     string      `json:"label"`
	Real      pairMeasure `json:"real"`
	JudgeOnly pairMeasure `json:"judge_only"`
}

// pairJudge es lo que la pasada de duplicadas pide al modelo.
type pairJudge interface {
	JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error)
}

// measurePair pasa la pareja por la pasada de duplicadas real con el embedder dado.
// Devuelve error solo si el arnés no pudo medir (la pasada no contó ningún escalón
// sin haber fallado): eso no es un dato del modelo.
func measurePair(ctx context.Context, p materialPair, embedder llm.Embedder, judge pairJudge, cfg reduce.Config, log logger.Logger) (pairMeasure, error) {
	store := newMemoryStore("pair-"+p.ID, "")
	store.seedCandidates(p.A, p.B)
	pass := reduce.NewDedupePass(store, embedder, judge, cfg, log)

	start := time.Now()
	rep, err := pass.Run(ctx, store.job.JobID)
	m := pairMeasure{ElapsedMS: time.Since(start).Milliseconds()}
	if err != nil {
		m.ErrorKind = classifyModelError(err)
		m.Error = err.Error()
		// El informe llega con lo contado hasta el fallo: si el embedding salió, el par
		// murió en el juez (la pasada no cuenta el par que falla).
		if rep.EmbeddingsComputed > 0 {
			m.Tier = pairTierLLM
		}
		return m, nil
	}
	switch {
	case rep.PairsText == 1:
		m.Tier = pairTierText
	case rep.PairsEmbed == 1:
		m.Tier = pairTierEmbed
	case rep.PairsLLM == 1:
		m.Tier = pairTierLLM
	default:
		return m, fmt.Errorf("pareja %q: la pasada de duplicadas no comparó el par (procesadas: %d)", p.ID, rep.Processed)
	}
	m.Verdict = pairLabelNoDup
	if rep.DroppedDup > 0 {
		m.Verdict = pairLabelDup
	}
	return m, nil
}

// pairCounts son los contadores de una categoría en una de las dos medidas.
type pairCounts struct {
	Pairs          int `json:"pairs"`
	TierText       int `json:"tier_text"`
	TierEmbed      int `json:"tier_embed"`
	TierLLM        int `json:"tier_llm"`
	Hits           int `json:"hits"`            // la pasada dijo lo mismo que la etiqueta
	MissedDups     int `json:"missed_dups"`     // etiqueta dup, la pasada dijo no_dup
	FalseDups      int `json:"false_dups"`      // etiqueta no_dup, la pasada dijo dup
	InvalidOutputs int `json:"invalid_outputs"` // el juez devolvió algo que no se pudo leer
	ProviderErrors int `json:"provider_errors"` // red, timeout, 5xx
}

func (c *pairCounts) add(label string, m pairMeasure) {
	c.Pairs++
	switch m.Tier {
	case pairTierText:
		c.TierText++
	case pairTierEmbed:
		c.TierEmbed++
	case pairTierLLM:
		c.TierLLM++
	}
	switch m.ErrorKind {
	case errKindInvalidOutput:
		c.InvalidOutputs++
		return
	case errKindProvider:
		c.ProviderErrors++
		return
	}
	switch {
	case m.Verdict == label:
		c.Hits++
	case label == pairLabelDup:
		c.MissedDups++
	default:
		c.FalseDups++
	}
}

// pairCategoryRow es una fila de la tabla: una categoría con sus dos medidas.
type pairCategoryRow struct {
	Category  string     `json:"category"`
	Label     string     `json:"label"`
	Real      pairCounts `json:"real"`
	JudgeOnly pairCounts `json:"judge_only"`
}

// pairsTable cuenta las parejas por categoría, en el orden en que el fichero las
// declara.
func pairsTable(categories []string, runs []pairRun) []pairCategoryRow {
	rows := make([]pairCategoryRow, len(categories))
	index := make(map[string]int, len(categories))
	for i, c := range categories {
		rows[i].Category = c
		index[c] = i
	}
	for _, r := range runs {
		i, ok := index[r.Category]
		if !ok {
			continue
		}
		rows[i].Label = r.Label
		rows[i].Real.add(r.Label, r.Real)
		rows[i].JudgeOnly.add(r.Label, r.JudgeOnly)
	}
	return rows
}

// pairsRunData es lo que el modo guarda en el historial.
type pairsRunData struct {
	PairsSHA256 string            `json:"pairs_sha256"`
	Params      materialParams    `json:"params"`
	Table       []pairCategoryRow `json:"table"`
	Judge       callStats         `json:"judge"`
	Embed       embedStats        `json:"embed"`
	ElapsedMS   int64             `json:"elapsed_ms"`
	Pairs       []pairRun         `json:"pairs"`
}

// runPairs mide todas las parejas. Devuelve error solo si el arnés no pudo medir.
func runPairs(ctx context.Context, f pairsFile, model railModel, embedder llm.Embedder, cfg config.MaterialPipelineConfig, log logger.Logger, progress func(pairRun)) ([]pairRun, callStats, embedStats, error) {
	probe := newProbeModel(model)
	realEmbedder := &probeEmbedder{inner: embedder}
	dedupeCfg := reduce.Config{DupHigh: cfg.DedupeHigh, DupLow: cfg.DedupeLow}

	runs := make([]pairRun, 0, len(f.Pairs))
	for _, p := range f.Pairs {
		run := pairRun{ID: p.ID, Category: p.Category, Label: p.Label}
		var err error
		if run.Real, err = measurePair(ctx, p, realEmbedder, probe, dedupeCfg, log); err != nil {
			return nil, callStats{}, embedStats{}, err
		}
		if run.JudgeOnly, err = measurePair(ctx, p, greyZoneEmbedder{}, probe, dedupeCfg, log); err != nil {
			return nil, callStats{}, embedStats{}, err
		}
		runs = append(runs, run)
		if progress != nil {
			progress(run)
		}
	}
	return runs, probe.judge, realEmbedder.stats, nil
}

// printPairsTable imprime la tabla por categoría. Sin veredicto: cuenta.
func printPairsTable(rows []pairCategoryRow) {
	fmt.Printf("\n%-26s %-7s %-10s %5s | %5s %5s %5s | %5s %7s %7s %8s %6s\n",
		"categoría", "etiq.", "medida", "pares", "letra", "cos", "juez", "igual", "dup-esc", "dup-fal", "inválida", "prov.")
	for _, r := range rows {
		for _, mc := range []struct {
			name string
			c    pairCounts
		}{{pairModeReal, r.Real}, {pairModeJudgeOnly, r.JudgeOnly}} {
			fmt.Printf("%-26s %-7s %-10s %5d | %5d %5d %5d | %5d %7d %7d %8d %6d\n",
				r.Category, r.Label, mc.name, mc.c.Pairs, mc.c.TierText, mc.c.TierEmbed, mc.c.TierLLM,
				mc.c.Hits, mc.c.MissedDups, mc.c.FalseDups, mc.c.InvalidOutputs, mc.c.ProviderErrors)
		}
	}
	fmt.Println("\nigual = la pasada dijo lo mismo que la etiqueta; dup-esc = duplicada que se escapa; dup-fal = falsa duplicada.")
}
