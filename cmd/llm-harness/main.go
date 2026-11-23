// Command llm-harness es el entregable D-039.8 (extendido en 040 T2c y en la
// AR-INF.1): con el provider elegido por flag corre un modo y reporta lo medido y
// los tiempos:
//
//   - mode=generate (default, D-039.8): toma un material de muestra, corre
//     GenerateAssessment y valida el JSON contra las reglas del contrato 038.
//
//   - mode=review (040 T2c): corre ReviewAnswer contra una batería de casos de
//     muestra en español (correcto, incorrecto, parcial, vacío/sin sentido,
//     parafraseo, prompt-injection) y cuenta si verdict/score coinciden con lo
//     esperado.
//
//   - mode=embed (044 F1b): calibra el dedupe por embeddings. Vectoriza una batería
//     de pares dup/no_dup en español, calcula el coseno de cada par y reporta los
//     umbrales dup_high/dup_low, la zona gris y la separación por modelo candidato.
//
//   - mode=bank (AR-INF.1): pasa el banco etiquetado de learning por el carril de
//     corrección real (processor.ReviewAnswerLane) y saca una tabla por categoría.
//
// Sirve para (a) smoke de la infra LLM, (b) elegir el modelo local midiendo (no
// en papel) y (c) regresión de prompts.
//
// MIDE, NO JUZGA (ADR 0048): una corrida por caso, sin reintentos ni casos
// apartados; una salida vacía o inválida es un dato. No hay PASS/FAIL por lo que
// escribe el modelo y el código de salida solo es != 0 si el arnés no pudo correr
// (sin modelo, sin banco, sin poder guardar). Cada corrida se guarda como fichero
// nuevo en cmd/llm-harness/testdata/runs/ (runlog.go), sin pisar las anteriores.
//
// El modelo local sale de -model o, si no se dice, de LLM_LOCAL_MODEL (el de
// producción). Sin modelo dicho no arranca: no hay modelo por defecto.
//
// Uso:
//
//	go run ./cmd/llm-harness -mode generate -provider ollama -model qwen3:1.7b -questions 3
//	go run ./cmd/llm-harness -mode review   -provider ollama -model qwen3:1.7b
//	LLM_LOCAL_MODEL=gemma4:e2b-it-qat go run ./cmd/llm-harness -mode bank \
//	    -bank ../edugo-api-learning/internal/core/domain/testdata/answer_bank.json \
//	    -deterministic /ruta/al/informe.json
//	go run ./cmd/llm-harness -mode generate -provider api -api-provider anthropic \
//	    -api-key "$LLM_API_KEY" -api-model claude-sonnet-5 -material ./material.txt
//
// NO instala nada ni asume que hay un Ollama corriendo: antes de medir comprueba que
// el servidor responde y que el modelo está instalado (nunca lo descarga); si no,
// termina con código != 0.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/assessmentimport"
	"github.com/EduGoGroup/edugo-worker/internal/chunking"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	llmapi "github.com/EduGoGroup/edugo-worker/internal/llm/api"
	"github.com/EduGoGroup/edugo-worker/internal/llm/ollama"
)

// sampleMaterial es el material por defecto si no se pasa -material.
const sampleMaterial = `La fotosíntesis es el proceso por el cual las plantas, algas y algunas
bacterias convierten la energía luminosa en energía química. Ocurre principalmente en los
cloroplastos, que contienen clorofila, el pigmento verde que absorbe la luz. En la fase
luminosa, la energía del sol se usa para dividir moléculas de agua (fotólisis), liberando
oxígeno. En la fase oscura (ciclo de Calvin), el dióxido de carbono se fija para formar
glucosa. La ecuación general es: 6 CO2 + 6 H2O + luz -> C6H12O6 + 6 O2.`

func main() {
	mode := flag.String("mode", "generate", "modo del harness: generate (contrato 038) | review (corrección, 040 T2c) | prep (preparación, 042 F2d) | review-prep (carril triturado short_answer, 042 F3d) | material (pipeline A/B material→evaluación, 043 F3b) | embed (calibración dedupe por embeddings, 044 F1b) | relevance (calibración umbral relevancia, 044 F2a) | bank (banco etiquetado por el carril de corrección, AR-INF.1)")
	provider := flag.String("provider", "local", "provider LLM: local (alias de ollama) | ollama | api. 'local'/'api' espejan el vocabulario de la política por escuela (D-039.2; 'off' no aplica al harness)")
	materialPath := flag.String("material", "", "ruta a un archivo de texto con el material (vacío = muestra interna)")
	title := flag.String("title", "Fotosíntesis — capítulo 3", "título del material")
	subjectHint := flag.String("subject", "Biología", "pista de materia")
	numQuestions := flag.Int("questions", 3, "número de preguntas a generar")
	difficulty := flag.String("difficulty", "", "dificultad objetivo: easy|medium|hard (vacío = mezcla)")
	timeout := flag.Duration("timeout", 120*time.Second, "timeout de la generación")

	ollamaURL := flag.String("ollama-url", "http://localhost:11434", "base URL de Ollama")
	ollamaModel := flag.String("model", os.Getenv("LLM_LOCAL_MODEL"), "modelo de Ollama (default env LLM_LOCAL_MODEL; sin modelo dicho, el arnés no arranca)")

	apiProvider := flag.String("api-provider", "anthropic", "backend del provider api: anthropic|gemini")
	apiKey := flag.String("api-key", os.Getenv("LLM_API_KEY"), "API key (default env LLM_API_KEY)")
	apiModel := flag.String("api-model", "claude-sonnet-5", "modelo del provider api")
	apiBaseURL := flag.String("api-base-url", "", "base URL del provider api (vacío = default)")

	materialInputsCSV := flag.String("material-inputs", "", "modo material: rutas coma-separadas a documentos (.pdf → pdf.Extractor, .txt → texto plano). Vacío = todos los .txt de "+defaultMaterialDir)
	chunkTarget := flag.Int("chunk-target", 300, "modo material: TargetWords del porcionado (default = config productiva)")
	chunkMax := flag.Int("chunk-max", 400, "modo material: MaxWords del porcionado (default = config productiva)")
	chunkMin := flag.Int("chunk-min", 200, "modo material: MinWords del porcionado (default = config productiva)")
	chunkMerge := flag.Int("chunk-merge", 80, "modo material: MergeThresholdWords del porcionado (default = config productiva)")
	skipPropose := flag.Bool("skip-propose", false, "modo material: omite la Batería B (solo mide el digest A)")
	digestPrompt := flag.String("digest-prompt", "v2", "modo material: variante del prompt A: v2 (tarea partida, ruta productiva del provider local; default) | v1 (llamada única legacy, para regresión)")

	embedModelsCSV := flag.String("embed-models", "nomic-embed-text,embeddinggemma", "modo embed: modelos de embeddings coma-separados a comparar (secuencial)")
	embedPairsPath := flag.String("embed-pairs", defaultEmbedPairs, "modo embed: ruta a la batería de pares dup/no_dup")
	embedOutDir := flag.String("embed-out-dir", "", "modo embed: carpeta donde escribir results-<modelo>.json, sin pisar (vacío = una corrida nueva en -runs-dir)")

	relevanceCasesPath := flag.String("relevance-cases", defaultRelevanceCases, "modo relevance: ruta a la batería de casos central/peripheral/unanswerable")
	relevanceOutPath := flag.String("relevance-out", "", "modo relevance: ruta del resultado, sin pisar (vacío = una corrida nueva en -runs-dir)")

	bankPath := flag.String("bank", "", "modo bank: ruta al banco de learning (internal/core/domain/testdata/answer_bank.json)")
	deterministicPath := flag.String("deterministic", "", "modo bank: ruta al informe que escribe la prueba de learning con ANSWER_BANK_REPORT")
	runsDir := flag.String("runs-dir", defaultRunsDir, "carpeta del historial de corridas (un fichero nuevo por corrida, nunca se pisa)")

	flag.Parse()

	// El modo embed no genera texto: usa el puerto Embedder (no LLMProvider), así que
	// no construye el provider LLM ni necesita material. Se resuelve y retorna aquí.
	if *mode == "embed" {
		runEmbed(*ollamaURL, *embedModelsCSV, *embedPairsPath, *embedOutDir, *runsDir, *timeout)
		return
	}

	content := sampleMaterial
	if *materialPath != "" {
		raw, err := os.ReadFile(*materialPath)
		if err != nil {
			fatalf("no se pudo leer el material %q: %v", *materialPath, err)
		}
		content = string(raw)
	}

	p, err := buildProvider(*provider, providerFlags{
		ollamaURL:   *ollamaURL,
		ollamaModel: *ollamaModel,
		timeout:     *timeout,
		apiProvider: *apiProvider,
		apiKey:      *apiKey,
		apiModel:    *apiModel,
		apiBaseURL:  *apiBaseURL,
	})
	if err != nil {
		fatalf("construyendo provider: %v", err)
	}

	// Sin modelo dicho no hay medida (AR-INF.1, decisión 8): no se cae a un modelo
	// por defecto. Con el provider local, antes de medir se comprueba que Ollama
	// responde y que el modelo está instalado.
	modelName := *apiModel
	var mi modelInfo
	if *mode == "bank" && *provider == "api" && !flagSet("api-model") {
		// El banco no sale a una API externa con un modelo que nadie eligió.
		fatalf("el modo bank con -provider api pide -api-model dicho")
	}
	if *provider == "local" || *provider == "ollama" {
		modelName = *ollamaModel
		if strings.TrimSpace(modelName) == "" {
			fatalf("sin modelo: pasa -model o define LLM_LOCAL_MODEL")
		}
		mi, err = preflightOllama(*ollamaURL, modelName, 15*time.Second)
		if err != nil {
			fatalf("comprobación previa: %v", err)
		}
	}
	info := newRunInfo(*mode, *provider, modelName, mi)

	switch *mode {
	case "generate":
		material := llm.MaterialInput{Title: *title, Content: content, SubjectHint: *subjectHint}
		params := llm.GenerationParams{NumQuestions: *numQuestions, Language: "es", Difficulty: *difficulty}
		runGenerate(p, info, *runsDir, material, params, len(content), *timeout)
	case "review":
		runReview(p, info, *runsDir, *timeout)
	case "prep":
		runPrep(p, info, *runsDir, *timeout)
	case "review-prep":
		runReviewPrep(p, info, *runsDir, *timeout)
	case "bank":
		runBank(p, info, *bankPath, *deterministicPath, *runsDir, *timeout)
	case "material":
		runMaterial(p, materialOptions{
			timeout:   *timeout,
			inputsCSV: *materialInputsCSV,
			chunkCfg: chunking.Config{
				TargetWords:         *chunkTarget,
				MaxWords:            *chunkMax,
				MinWords:            *chunkMin,
				MergeThresholdWords: *chunkMerge,
			},
			skipPropose:   *skipPropose,
			digestVariant: *digestPrompt,
			provider:      *provider,
			ollamaURL:     *ollamaURL,
			ollamaModel:   *ollamaModel,
		})
	case "relevance":
		runRelevance(p, info, *runsDir, *relevanceCasesPath, *relevanceOutPath, *timeout)
	default:
		fatalf("modo desconocido %q (usa generate|review|prep|review-prep|material|embed|relevance|bank)", *mode)
	}
}

// generateRun es lo que el modo generate guarda en el historial.
type generateRun struct {
	ElapsedMS     int64    `json:"elapsed_ms"`
	Error         string   `json:"error,omitempty"`
	JSONBytes     int      `json:"json_bytes"`
	ValidContract bool     `json:"valid_contract"`
	Issues        []string `json:"issues,omitempty"`
	Title         string   `json:"title,omitempty"`
	Questions     int      `json:"questions"`
	RequestedQ    int      `json:"requested_questions"`
}

// runGenerate corre el modo generación (D-039.8): genera una evaluación y la
// valida contra el contrato 038. Una generación fallida o que no cumple el
// contrato es un dato de la corrida, no un código de salida (ADR 0048).
func runGenerate(p llm.LLMProvider, info runInfo, runsDir string, material llm.MaterialInput, params llm.GenerationParams, contentBytes int, timeout time.Duration) {
	fmt.Printf("== llm-harness (generate) ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("preguntas: %d\n", params.NumQuestions)
	fmt.Printf("material : %q (%d bytes)\n\n", material.Title, contentBytes)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	rawJSON, genErr := p.GenerateAssessment(ctx, material, params)
	elapsed := time.Since(start)
	run := generateRun{ElapsedMS: elapsed.Milliseconds(), JSONBytes: len(rawJSON), RequestedQ: params.NumQuestions}

	fmt.Printf("generación: %s\n", elapsed.Round(time.Millisecond))
	if genErr != nil {
		fmt.Printf("resultado : la generación falló: %v\n", genErr)
		run.Error = genErr.Error()
		saveRun(runsDir, info, run)
		return
	}
	fmt.Printf("JSON      : %d bytes\n\n", len(rawJSON))

	contract, valErr := assessmentimport.Validate(rawJSON, assessmentimport.DefaultLimits())
	if valErr != nil {
		fmt.Printf("resultado : no cumple el contrato 038\n\n")
		if ve, ok := valErr.(*assessmentimport.ValidationError); ok {
			for _, iss := range ve.Issues {
				fmt.Printf("  - %s\n", iss.String())
				run.Issues = append(run.Issues, iss.String())
			}
		} else {
			fmt.Printf("  - %v\n", valErr)
			run.Issues = append(run.Issues, valErr.Error())
		}
		fmt.Printf("\n--- JSON recibido ---\n%s\n", prettyOrRaw(rawJSON))
		saveRun(runsDir, info, run)
		return
	}

	run.ValidContract = true
	run.Title = contract.Assessment.Title
	run.Questions = len(contract.Questions)
	fmt.Printf("resultado : cumple el contrato 038\n")
	fmt.Printf("  título          : %s\n", contract.Assessment.Title)
	fmt.Printf("  preguntas       : %d\n", len(contract.Questions))
	fmt.Printf("  tiempo total    : %s\n", elapsed.Round(time.Millisecond))
	saveRun(runsDir, info, run)
}

type providerFlags struct {
	ollamaURL   string
	ollamaModel string
	timeout     time.Duration
	apiProvider string
	apiKey      string
	apiModel    string
	apiBaseURL  string
}

func buildProvider(kind string, f providerFlags) (llm.LLMProvider, error) {
	switch kind {
	// "local" es alias de "ollama": así el harness habla el mismo vocabulario que
	// la política por escuela (D-039.2: local|api|off). "off" no aplica al harness
	// (no hay generación que medir sin provider).
	case "local", "ollama":
		return ollama.New(ollama.Config{
			BaseURL: f.ollamaURL,
			Model:   f.ollamaModel,
			Timeout: f.timeout,
		}), nil
	case "api":
		return llmapi.New(llmapi.Config{
			Provider: f.apiProvider,
			APIKey:   f.apiKey,
			Model:    f.apiModel,
			BaseURL:  f.apiBaseURL,
			Timeout:  f.timeout,
		})
	default:
		return nil, fmt.Errorf("provider desconocido %q (usa local|ollama|api)", kind)
	}
}

func prettyOrRaw(raw json.RawMessage) string {
	var buf []byte
	var tmp any
	if err := json.Unmarshal(raw, &tmp); err == nil {
		if b, err := json.MarshalIndent(tmp, "", "  "); err == nil {
			buf = b
		}
	}
	if buf == nil {
		return string(raw)
	}
	return string(buf)
}

// flagSet dice si un flag se pasó en la línea de comandos (no su valor por defecto).
func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
