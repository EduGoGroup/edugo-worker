package main

// material_v1.go — variante v1 LEGACY del digest (llamada A única) para regresión.
//
// Desde el experimento anti-degeneración (2026-07-19) la ruta productiva del provider
// ollama es la "tarea partida" v2 (internal/llm/digest_split.go): p.DigestChunk ya
// parte en dos llamadas. Este archivo conserva la llamada única original —el prompt
// productivo hasta ese día, BuildDigestChunkPrompt— detrás de -digest-prompt=v1, para
// poder re-medir la comparación v1 vs v2 en futuras regresiones sin tocar producción.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// digestChunkV1 corre la llamada A única legacy contra Ollama: el mismo prompt
// (BuildDigestChunkPrompt) y parseo (ParseDigestResult) que usaba la ruta productiva
// antes de la partición, con el request espejado del provider.
func digestChunkV1(opts materialOptions, chunkText string, prevSummary *string) (*llm.DigestChunkResult, error) {
	prompt := llm.BuildDigestChunkPrompt(llm.DigestChunkInput{
		ChunkText:   chunkText,
		PrevSummary: prevSummary,
		Language:    "es",
	})
	rawJSON, err := ollamaGenerateJSON(opts, prompt)
	if err != nil {
		return nil, err
	}
	artifacts, summary, err := llm.ParseDigestResult(rawJSON)
	if err != nil {
		return nil, err
	}
	return &llm.DigestChunkResult{Artifacts: artifacts, Summary: summary}, nil
}

// ollamaGenerateJSON ejecuta POST /api/generate espejando el request productivo del
// provider ollama (stream:false, format:"json", think:false, temperature 0) y aísla el
// objeto JSON de la respuesta con el mismo llm.ExtractJSON.
func ollamaGenerateJSON(opts materialOptions, prompt string) (json.RawMessage, error) {
	reqBody := map[string]any{
		"model":   opts.ollamaModel,
		"prompt":  prompt,
		"stream":  false,
		"format":  "json",
		"think":   false,
		"options": map[string]any{"temperature": 0.0},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}
	client := &http.Client{Timeout: opts.timeout}
	resp, err := client.Post(strings.TrimRight(opts.ollamaURL, "/")+"/api/generate", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ollama response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var gr struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parsing ollama response: %w", err)
	}
	return llm.ExtractJSON(gr.Response)
}
