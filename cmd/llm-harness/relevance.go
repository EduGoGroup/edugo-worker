package main

// relevance.go — modo "relevance" del harness (plan 044 F2a): calibra la pasada 2 del
// reduce (D-044.3, D-044.7 «medir antes de cablear») ANTES de fijar el umbral de
// producción RelevanceMin. Por cada caso de la batería llama ScoreRelevance con las ideas
// principales del job + la pregunta candidata (una llamada por caso, contexto fresco), y
// mide qué tan bien el modelo separa las tres familias etiquetadas a mano:
//
//   - central       → la pregunta se responde con una IDEA PRINCIPAL del material.
//   - peripheral    → se relaciona pero toca un detalle secundario.
//   - unanswerable  → NO se responde con estas ideas (fuera de tema / pide lo que no está).
//
// Política del diseño (§ D-044.3): la relevancia SOLO debe descartar las unanswerable; las
// peripheral DEBEN sobrevivir (el mix central/periférica lo decide la selección final, no
// esta pasada). Por eso el análisis de umbral reporta dos errores contrapuestos por cada
// candidato de RelevanceMin:
//
//   - falsos vivos    : unanswerable que SOBREVIVEN (score ≥ umbral) — basura que pasa.
//   - falsos descartes : central/peripheral que MUEREN (score < umbral) — señal que se pierde.
//
// Una salida malformada NUNCA descarta la candidata (conservador, ParseRelevanceResult): se
// reintenta una vez y, si persiste, se cuenta como "malformed" sin inventar score. En
// producción esa candidata sobrevive; se reporta el conteo aparte para no maquillar.
//
// No instala nada ni asume Ollama corriendo: si el backend no responde, el error de la
// llamada se propaga como malformed del caso (no aborta la corrida). Un caso por llamada,
// secuencial, temperatura 0 (la aplica el provider por default: greedy determinista).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// defaultRelevanceCases es la batería por defecto (relativa a la raíz del worker, donde se
// corre `go run ./cmd/llm-harness`).
const defaultRelevanceCases = "cmd/llm-harness/testdata/relevance/cases.json"

// relevanceThresholdCandidates son los valores de RelevanceMin a evaluar. Incluyen el
// default de producción (0.4) y sus vecinos, para ver si otro corte separa mejor.
var relevanceThresholdCandidates = []float64{0.3, 0.4, 0.5, 0.6}

// relevanceScorer es la interfaz mínima que el modo necesita del provider (ISP): puntuar la
// relevancia de una candidata. La satisfacen los providers concretos (ollama, api); no está
// en llm.LLMProvider porque la pasada la consume por su propia interfaz.
type relevanceScorer interface {
	ScoreRelevance(ctx context.Context, req llm.RelevanceRequest) (llm.RelevanceResult, error)
}

// relevanceCase es una fila de la batería: ideas del job + pregunta candidata + veredicto
// humano (central|peripheral|unanswerable).
type relevanceCase struct {
	ID       string   `json:"id"`
	Ideas    []string `json:"ideas"`
	Question string   `json:"question"`
	Label    string   `json:"label"`
	Source   string   `json:"source"`
	Note     string   `json:"note"`
}

// relevanceScore es el resultado medido de un caso.
type relevanceScore struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Score     float64 `json:"score"`
	Rationale string  `json:"rationale"`
	Malformed bool    `json:"malformed,omitempty"` // el modelo no devolvió un score válido tras 1 reintento
	Retried   bool    `json:"retried,omitempty"`   // hizo falta el reintento (aunque después saliera bien)
	Err       string  `json:"error,omitempty"`     // último error si quedó malformed
}

// labelDist es la distribución de scores de una familia (solo casos con score válido).
type labelDist struct {
	Label     string  `json:"label"`
	N         int     `json:"n"`         // casos con score válido
	Malformed int     `json:"malformed"` // casos de esta familia sin score válido
	Min       float64 `json:"min"`
	P25       float64 `json:"p25"`
	Median    float64 `json:"median"`
	P75       float64 `json:"p75"`
	Max       float64 `json:"max"`
	Mean      float64 `json:"mean"`
}

// thresholdEval es el saldo de un candidato de RelevanceMin: cuánta basura sobrevive vs
// cuánta señal se pierde. Solo cuenta casos con score válido (los malformed sobreviven en
// producción y se reportan aparte).
type thresholdEval struct {
	Threshold           float64 `json:"threshold"`
	FalseAlive          int     `json:"false_alive"`           // unanswerable con score ≥ umbral (basura que pasa)
	FalseDropCentral    int     `json:"false_drop_central"`    // central con score < umbral
	FalseDropPeripheral int     `json:"false_drop_peripheral"` // peripheral con score < umbral
	FalseDropTotal      int     `json:"false_drop_total"`      // central+peripheral que mueren
}

// relevanceReport es todo lo medido, serializado a results-<modelo>.json.
type relevanceReport struct {
	Model      string           `json:"model"`
	Provider   string           `json:"provider"`
	Cases      int              `json:"cases"`
	Malformed  int              `json:"malformed"`
	ElapsedMS  int64            `json:"elapsed_ms"`
	Dists      []labelDist      `json:"distributions"`
	Thresholds []thresholdEval  `json:"thresholds"`
	Scores     []relevanceScore `json:"scores"`
}

// labelOrder fija el orden de reporte de las familias.
var labelOrder = []string{"central", "peripheral", "unanswerable"}

// runRelevance corre el modo relevancia: carga la batería, puntúa cada caso, reporta la
// distribución por familia + el análisis de umbral y escribe results-<modelo>.json. No sale
// con código != 0 por errores de un caso (se cuentan como malformed): solo aborta si la
// batería no se puede cargar o el provider no sabe puntuar relevancia.
func runRelevance(p llm.LLMProvider, model, casesPath, outPath string, timeout time.Duration) {
	scorer, ok := p.(relevanceScorer)
	if !ok {
		fatalf("el provider %s no implementa ScoreRelevance (modo relevance requiere ollama|api)", p.Name())
	}

	cases, err := loadRelevanceCases(casesPath)
	if err != nil {
		fatalf("cargando batería %q: %v", casesPath, err)
	}

	fmt.Printf("== llm-harness (relevance) ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("batería  : %s (%d casos)\n", casesPath, len(cases))
	nByLabel := map[string]int{}
	for _, c := range cases {
		nByLabel[c.Label]++
	}
	fmt.Printf("familias : %d central / %d peripheral / %d unanswerable\n\n",
		nByLabel["central"], nByLabel["peripheral"], nByLabel["unanswerable"])

	start := time.Now()
	scores := make([]relevanceScore, 0, len(cases))
	for i, c := range cases {
		s := scoreRelevanceCase(scorer, c, timeout)
		scores = append(scores, s)
		mark := fmt.Sprintf("%.2f", s.Score)
		if s.Malformed {
			mark = "MALFORMED"
		}
		fmt.Printf("  [%2d/%2d] %-5s %-13s %s\n", i+1, len(cases), c.ID, c.Label, mark)
	}
	elapsed := time.Since(start)

	report := buildRelevanceReport(p, model, scores, elapsed)
	printRelevanceReport(report)

	if strings.TrimSpace(outPath) == "" {
		outPath = filepath.Join(filepath.Dir(casesPath), "results-"+relevanceSlug(model)+".json")
	}
	if err := writeRelevanceReport(outPath, report); err != nil {
		fmt.Printf("  aviso: no se pudo escribir %s: %v\n", outPath, err)
	} else {
		fmt.Printf("\n→ %s\n", outPath)
	}
}

// scoreRelevanceCase puntúa un caso con 1 reintento ante error (malformed/timeout/conexión).
// No inventa score: si el segundo intento también falla, deja Malformed=true con el error.
func scoreRelevanceCase(scorer relevanceScorer, c relevanceCase, timeout time.Duration) relevanceScore {
	req := llm.RelevanceRequest{QuestionText: c.Question, MainIdeas: c.Ideas, Language: "es"}
	s := relevanceScore{ID: c.ID, Label: c.Label}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		res, err := scorer.ScoreRelevance(ctx, req)
		cancel()
		if err == nil {
			s.Score = res.Score
			s.Rationale = res.Rationale
			s.Retried = attempt > 0
			return s
		}
		lastErr = err
		if attempt == 0 {
			s.Retried = true
		}
	}
	s.Malformed = true
	s.Err = lastErr.Error()
	return s
}

// buildRelevanceReport agrega los scores en distribución por familia + análisis de umbral.
func buildRelevanceReport(p llm.LLMProvider, model string, scores []relevanceScore, elapsed time.Duration) relevanceReport {
	malformed := 0
	for _, s := range scores {
		if s.Malformed {
			malformed++
		}
	}
	return relevanceReport{
		Model:      model,
		Provider:   p.Name(),
		Cases:      len(scores),
		Malformed:  malformed,
		ElapsedMS:  elapsed.Milliseconds(),
		Dists:      relevanceDistributions(scores),
		Thresholds: relevanceThresholds(scores),
		Scores:     scores,
	}
}

// relevanceDistributions calcula min/p25/mediana/p75/max/media por familia (solo scores
// válidos), en el orden central → peripheral → unanswerable.
func relevanceDistributions(scores []relevanceScore) []labelDist {
	valid := map[string][]float64{}
	malformed := map[string]int{}
	for _, s := range scores {
		if s.Malformed {
			malformed[s.Label]++
			continue
		}
		valid[s.Label] = append(valid[s.Label], s.Score)
	}
	var out []labelDist
	for _, label := range labelOrder {
		vals := valid[label]
		d := labelDist{Label: label, N: len(vals), Malformed: malformed[label]}
		if len(vals) > 0 {
			sort.Float64s(vals)
			d.Min = vals[0]
			d.P25 = quantile(vals, 0.25)
			d.Median = quantile(vals, 0.50)
			d.P75 = quantile(vals, 0.75)
			d.Max = vals[len(vals)-1]
			d.Mean = mean(vals)
		}
		out = append(out, d)
	}
	return out
}

// relevanceThresholds evalúa cada candidato de RelevanceMin: falsos vivos (unanswerable que
// sobreviven) y falsos descartes (central/peripheral que mueren). Solo scores válidos.
func relevanceThresholds(scores []relevanceScore) []thresholdEval {
	out := make([]thresholdEval, 0, len(relevanceThresholdCandidates))
	for _, thr := range relevanceThresholdCandidates {
		e := thresholdEval{Threshold: thr}
		for _, s := range scores {
			if s.Malformed {
				continue
			}
			switch s.Label {
			case "unanswerable":
				if s.Score >= thr {
					e.FalseAlive++ // basura que sobrevive
				}
			case "central":
				if s.Score < thr {
					e.FalseDropCentral++
				}
			case "peripheral":
				if s.Score < thr {
					e.FalseDropPeripheral++
				}
			}
		}
		e.FalseDropTotal = e.FalseDropCentral + e.FalseDropPeripheral
		out = append(out, e)
	}
	return out
}

// --- salida ---

func printRelevanceReport(r relevanceReport) {
	fmt.Printf("\n── distribución de scores por familia (%d casos, %d malformed, %d ms) ──\n",
		r.Cases, r.Malformed, r.ElapsedMS)
	fmt.Printf("  %-13s %-4s %-6s %-6s %-6s %-6s %-6s %-6s %s\n",
		"FAMILIA", "N", "MIN", "P25", "MEDIANA", "P75", "MAX", "MEDIA", "MALF")
	for _, d := range r.Dists {
		if d.N == 0 {
			fmt.Printf("  %-13s %-4d (sin scores válidos)  malf=%d\n", d.Label, d.N, d.Malformed)
			continue
		}
		fmt.Printf("  %-13s %-4d %-6.2f %-6.2f %-6.2f %-6.2f %-6.2f %-6.2f %d\n",
			d.Label, d.N, d.Min, d.P25, d.Median, d.P75, d.Max, d.Mean, d.Malformed)
	}

	fmt.Printf("\n── análisis de umbral RelevanceMin (falsos vivos = unanswerable que pasan; falsos descartes = central/peripheral que mueren) ──\n")
	fmt.Printf("  %-8s %-13s %-16s %-11s %-14s %s\n",
		"UMBRAL", "FALSOS VIVOS", "FALSOS DESCARTES", "· central", "· peripheral", "")
	for _, e := range r.Thresholds {
		flag := ""
		if e.FalseDropPeripheral > 0 {
			flag = "⚠ mata peripheral"
		}
		fmt.Printf("  %-8.2f %-13d %-16d %-11d %-14d %s\n",
			e.Threshold, e.FalseAlive, e.FalseDropTotal, e.FalseDropCentral, e.FalseDropPeripheral, flag)
	}
}

// writeRelevanceReport escribe el reporte a JSON indentado.
func writeRelevanceReport(path string, r relevanceReport) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// --- helpers ---

// loadRelevanceCases lee y parsea la batería de casos de relevancia.
func loadRelevanceCases(path string) ([]relevanceCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []relevanceCase
	if err := json.Unmarshal(b, &cases); err != nil {
		return nil, fmt.Errorf("parseando JSON: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("batería vacía")
	}
	return cases, nil
}

// mean promedia un slice no vacío (el caller garantiza len > 0).
func mean(vals []float64) float64 {
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// relevanceSlug convierte el nombre de modelo en un slug con guiones para el nombre de
// archivo (ej. "gemma4:e4b" → "gemma4-e4b"), consistente con el results-gemma4-e4b.json
// esperado por el plan.
func relevanceSlug(model string) string {
	repl := strings.NewReplacer(":", "-", "/", "-", "_", "-")
	return repl.Replace(model)
}
