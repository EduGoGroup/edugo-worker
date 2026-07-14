// Command llm-harness es el entregable D-039.8: toma un material de muestra,
// corre GenerateAssessment con el provider elegido por flag, valida el JSON
// resultante contra las reglas replicadas del contrato 038 y reporta pass/fail y
// tiempos. Sirve para (a) smoke de la infra LLM, (b) elegir el modelo local
// midiendo (no en papel) y (c) regresión de prompts en 041.
//
// Uso:
//
//	go run ./cmd/llm-harness -provider ollama -model llama3.1 -questions 3
//	go run ./cmd/llm-harness -provider api -api-provider anthropic \
//	    -api-key "$LLM_API_KEY" -api-model claude-sonnet-5 -material ./material.txt
//
// NO instala nada ni asume que hay un Ollama corriendo: si el provider local no
// responde, reporta el error de conexión y termina con código != 0.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/assessmentimport"
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
	provider := flag.String("provider", "local", "provider LLM: local (alias de ollama) | ollama | api. 'local'/'api' espejan el vocabulario de la política por escuela (D-039.2; 'off' no aplica al harness)")
	materialPath := flag.String("material", "", "ruta a un archivo de texto con el material (vacío = muestra interna)")
	title := flag.String("title", "Fotosíntesis — capítulo 3", "título del material")
	subjectHint := flag.String("subject", "Biología", "pista de materia")
	numQuestions := flag.Int("questions", 3, "número de preguntas a generar")
	difficulty := flag.String("difficulty", "", "dificultad objetivo: easy|medium|hard (vacío = mezcla)")
	timeout := flag.Duration("timeout", 120*time.Second, "timeout de la generación")

	ollamaURL := flag.String("ollama-url", "http://localhost:11434", "base URL de Ollama")
	ollamaModel := flag.String("model", "llama3.1", "modelo de Ollama")

	apiProvider := flag.String("api-provider", "anthropic", "backend del provider api: anthropic|gemini")
	apiKey := flag.String("api-key", os.Getenv("LLM_API_KEY"), "API key (default env LLM_API_KEY)")
	apiModel := flag.String("api-model", "claude-sonnet-5", "modelo del provider api")
	apiBaseURL := flag.String("api-base-url", "", "base URL del provider api (vacío = default)")

	flag.Parse()

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

	fmt.Printf("== llm-harness ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("preguntas: %d\n", *numQuestions)
	fmt.Printf("material : %q (%d bytes)\n\n", *title, len(content))

	material := llm.MaterialInput{Title: *title, Content: content, SubjectHint: *subjectHint}
	params := llm.GenerationParams{NumQuestions: *numQuestions, Language: "es", Difficulty: *difficulty}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	start := time.Now()
	rawJSON, genErr := p.GenerateAssessment(ctx, material, params)
	elapsed := time.Since(start)

	fmt.Printf("generación: %s\n", elapsed.Round(time.Millisecond))
	if genErr != nil {
		fmt.Printf("RESULTADO : FAIL (generación)\n")
		fatalf("GenerateAssessment falló: %v", genErr)
	}
	fmt.Printf("JSON      : %d bytes\n\n", len(rawJSON))

	contract, valErr := assessmentimport.Validate(rawJSON, assessmentimport.DefaultLimits())
	if valErr != nil {
		fmt.Printf("RESULTADO : FAIL (validación contrato 038)\n\n")
		if ve, ok := valErr.(*assessmentimport.ValidationError); ok {
			for _, iss := range ve.Issues {
				fmt.Printf("  - %s\n", iss.String())
			}
		} else {
			fmt.Printf("  - %v\n", valErr)
		}
		fmt.Printf("\n--- JSON recibido ---\n%s\n", prettyOrRaw(rawJSON))
		os.Exit(1)
	}

	fmt.Printf("RESULTADO : PASS\n")
	fmt.Printf("  título          : %s\n", contract.Assessment.Title)
	fmt.Printf("  preguntas       : %d\n", len(contract.Questions))
	fmt.Printf("  tiempo total    : %s\n", elapsed.Round(time.Millisecond))
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
	var tmp interface{}
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

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
