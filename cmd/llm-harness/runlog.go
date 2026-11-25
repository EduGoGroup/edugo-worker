package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// El historial de corridas (AR-INF.1, decisión 7; ADR 0048, punto 4). Cada corrida
// de un modo que llama al modelo escribe UN fichero nuevo en defaultRunsDir, con la
// fecha en UTC en el nombre, y nunca pisa uno que ya exista. Dentro van el modelo y
// su versión, el commit del worker (la huella de los prompts, que son código) y si
// el árbol estaba sucio. Los modos del riel de material añaden el modelo de embeddings
// y su digest.

// defaultRunsDir es la carpeta del historial, relativa a la raíz del worker (donde
// corre `make llm-harness`). Está en git: el «antes» de un cambio de prompt no se
// pierde con la máquina.
const defaultRunsDir = "cmd/llm-harness/testdata/runs"

// runInfo es la cabecera de toda corrida guardada.
type runInfo struct {
	Mode          string `json:"mode"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	ModelDigest   string `json:"model_digest,omitempty"`
	OllamaVersion string `json:"ollama_version,omitempty"`
	// EmbedModel y su digest: solo en los modos que además usan embeddings (el riel de
	// material). La comprobación previa de ese modelo la hace el modo.
	EmbedModel       string  `json:"embed_model,omitempty"`
	EmbedModelDigest string  `json:"embed_model_digest,omitempty"`
	Temperature      float64 `json:"temperature"`
	WorkerCommit     string  `json:"worker_commit"`
	WorkerDirty      bool    `json:"worker_dirty"`
}

// modelInfo es lo que la comprobación previa sabe del modelo local.
type modelInfo struct {
	Digest        string
	OllamaVersion string
}

// newRunInfo arma la cabecera al empezar la corrida.
func newRunInfo(mode, provider, model string, mi modelInfo) runInfo {
	commit, dirty := workerGitState()
	return runInfo{
		Mode:          mode,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Provider:      provider,
		Model:         model,
		ModelDigest:   mi.Digest,
		OllamaVersion: mi.OllamaVersion,
		Temperature:   0, // la de producción (config.yaml); el provider local la manda explícita
		WorkerCommit:  commit,
		WorkerDirty:   dirty,
	}
}

// workerGitState devuelve el commit del worker y si el árbol tiene cambios, sin
// contar el propio historial (si no, la segunda corrida saldría «sucia» por el
// fichero de la primera). Fuera de un repo git devuelve «desconocido».
func workerGitState() (string, bool) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "desconocido", false
	}
	commit := strings.TrimSpace(string(out))
	status, err := exec.Command("git", "status", "--porcelain", "--", ".",
		":(exclude)"+defaultRunsDir).Output()
	if err != nil {
		return commit, false
	}
	return commit, strings.TrimSpace(string(status)) != ""
}

// writeRun guarda una corrida como fichero NUEVO en dir. El nombre lleva la fecha
// en UTC, el modo y el modelo; si ya existe (dos corridas en el mismo segundo), no
// lo pisa: devuelve error.
func writeRun(dir string, info runInfo, payload any) (string, error) {
	info.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	doc := struct {
		Run  runInfo `json:"run"`
		Data any     `json:"data"`
	}{Run: info, Data: payload}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializando la corrida: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creando %s: %w", dir, err)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.json", stamp, info.Mode, relevanceSlug(info.Model)))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("no se pisa una corrida que ya existe (%s): %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return "", fmt.Errorf("escribiendo %s: %w", path, err)
	}
	return path, nil
}

// saveRun escribe la corrida y lo dice; un fallo al guardar es de infraestructura
// (la medida no quedó), así que sí termina con código != 0.
func saveRun(dir string, info runInfo, payload any) {
	path, err := writeRun(dir, info, payload)
	if err != nil {
		fatalf("guardando la corrida: %v", err)
	}
	fmt.Printf("\ncorrida guardada: %s\n", path)
}

// preflightOllama comprueba, antes de medir, que el servidor responde y que el
// modelo pedido está instalado, y devuelve su digest y la versión de Ollama. Solo
// consulta: nunca descarga un modelo. Si algo falla, el arnés no pudo correr y
// termina con código != 0 (ADR 0048: sin modelo no hay medida).
func preflightOllama(baseURL, model string, timeout time.Duration) (modelInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	base := strings.TrimRight(baseURL, "/")

	var version struct {
		Version string `json:"version"`
	}
	if err := getJSON(ctx, client, base+"/api/version", &version); err != nil {
		return modelInfo{}, fmt.Errorf("el servidor de Ollama no responde en %s: %w", baseURL, err)
	}

	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if err := getJSON(ctx, client, base+"/api/tags", &tags); err != nil {
		return modelInfo{}, fmt.Errorf("listando los modelos de Ollama: %w", err)
	}
	names := make([]string, 0, len(tags.Models))
	for _, m := range tags.Models {
		if m.Name == model || m.Name == model+":latest" {
			return modelInfo{Digest: m.Digest, OllamaVersion: version.Version}, nil
		}
		names = append(names, m.Name)
	}
	return modelInfo{}, fmt.Errorf("el modelo %q no está instalado (hay: %s); el arnés no descarga modelos",
		model, strings.Join(names, ", "))
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return errors.New("respuesta no es JSON: " + err.Error())
	}
	return nil
}

// fileSHA256 es la huella de un fichero (el banco, el informe determinista).
func fileSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
