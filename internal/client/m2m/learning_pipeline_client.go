package m2m

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrPipelineConflict marca un 409 de cualquier ruta del carril de materiales
// (plan 043 F1): el recurso ya cambió de estado y la operación no aplica —chunks
// ya cerrados, job en otra fase, o un redelivery pisando un update ya hecho—. NO
// es un fallo transitorio ni permanente genérico: es un guard de estado/idempotencia
// y el CALLER decide qué hacer (abstenerse, releer el job, ACKear). Por eso NO se
// envuelve con ErrLearningPermanent: el clasificador de retry no debe verlo.
var ErrPipelineConflict = errors.New("conflicto de estado en el carril de materiales (409; el caller decide)")

// Rutas M2M del carril de materiales en learning (plan 043 F1). Base = api_learning.base_url.
const (
	pipelineJobPathFmt          = "/api/v1/internal/pipeline/jobs/%s"
	pipelineFileURLPathFmt      = "/api/v1/internal/pipeline/jobs/%s/file-url"
	pipelineChunksPathFmt       = "/api/v1/internal/pipeline/jobs/%s/chunks"
	pipelinePendingChunkPathFmt = "/api/v1/internal/pipeline/jobs/%s/chunks/pending"
	pipelineJobStatusPathFmt    = "/api/v1/internal/pipeline/jobs/%s/status"
	pipelineArtifactsPathFmt    = "/api/v1/internal/pipeline/chunks/%s/artifacts"
	pipelineChunkStatusPathFmt  = "/api/v1/internal/pipeline/chunks/%s/status"
	pipelineCandidatesPathFmt   = "/api/v1/internal/pipeline/jobs/%s/candidates"
	pipelineCandidatesPath      = "/api/v1/internal/pipeline/candidates"
	pipelineJobIdeasPathFmt     = "/api/v1/internal/pipeline/jobs/%s/ideas"
	pipelineChunkTextPathFmt    = "/api/v1/internal/pipeline/chunks/%s/text"
)

// PipelineJob es el estado de un job del carril material→evaluación (GET job).
// Espeja el DTO de learning F1. AssessmentID/LastError/CompletedAt vienen nil
// mientras el pipeline no los produce. ChunkCounts agrega por status ("pending",
// "done", …). Los timestamps se transportan como string ISO-8601 (UTC 'Z').
type PipelineJob struct {
	JobID         string         `json:"job_id"`
	MaterialID    string         `json:"material_id"`
	MaterialTitle string         `json:"material_title"`
	Status        string         `json:"status"`
	Phase         int16          `json:"phase"`
	AssessmentID  *string        `json:"assessment_id,omitempty"`
	LastError     *string        `json:"last_error,omitempty"`
	ChunkCounts   map[string]int `json:"chunk_counts"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	CompletedAt   *string        `json:"completed_at,omitempty"`
}

// PresignedFile es la URL firmada (GET presignado) del PDF del material y su
// vencimiento (GET file-url). La URL ya lleva la firma: se descarga SIN Authorization.
type PresignedFile struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// ChunkInput es un porción a persistir (POST chunks). Seq es el orden 0-based dentro
// del material; ChunkText es el texto plano del trozo.
type ChunkInput struct {
	Seq       int    `json:"seq"`
	ChunkText string `json:"chunk_text"`
}

// NextChunk es el siguiente chunk pendiente de procesar por el LLM (GET
// chunks/pending). PrevSummary NO es parte del objeto chunk en el JSON: viene al
// lado en el sobre; se adjunta aquí por conveniencia del caller (F3) y por eso NO
// tiene tag de serialización.
type NextChunk struct {
	ChunkID     string  `json:"chunk_id"`
	JobID       string  `json:"job_id"`
	Seq         int     `json:"seq"`
	ChunkText   string  `json:"chunk_text"`
	Status      string  `json:"status"`
	PrevSummary *string `json:"-"`
}

// CandidatePayload envuelve un candidato de ítem generado por el LLM como JSON
// crudo (PUT artifacts). El shape del payload lo valida learning contra su contrato.
type CandidatePayload struct {
	Payload json.RawMessage `json:"payload"`
}

// CandidateRecord es una candidata del job con sus columnas de control del reduce
// (plan 044 §4), tal como las devuelve GET jobs/{id}/candidates. `Payload` y
// `Embedding` viajan crudos (JSON): el reduce los interpreta (el payload contra
// CandidatePayloadV1, el embedding como []float32). `DedupeGroup`/`Score`/`Embedding`
// vienen nil mientras la pasada correspondiente no los produce. El orden de la lista
// es chunk_sequence ASC, id ASC (determinista para la selección de representante).
type CandidateRecord struct {
	ID            string          `json:"id"`
	ChunkID       string          `json:"chunk_id"`
	ChunkSequence int             `json:"chunk_sequence"`
	Payload       json.RawMessage `json:"payload"`
	Status        string          `json:"status"`
	DedupeGroup   *string         `json:"dedupe_group"`
	Score         *float64        `json:"score"`
	Embedding     json.RawMessage `json:"embedding"`
}

// CandidateUpdate es un cambio PARCIAL a una candidata (PATCH candidates): solo los
// campos presentes (no-nil) se aplican; el resto queda como está. El batch es atómico
// en learning; si el nuevo `status` de una candidata YA terminal difiere del actual,
// learning responde 409 (→ ErrPipelineConflict) y no aplica nada. `Embedding` es JSON
// crudo (vector []float32 serializado).
type CandidateUpdate struct {
	ID          string          `json:"id"`
	Status      *string         `json:"status,omitempty"`
	DedupeGroup *string         `json:"dedupe_group,omitempty"`
	Score       *float64        `json:"score,omitempty"`
	Embedding   json.RawMessage `json:"embedding,omitempty"`
}

// listCandidatesResponse es el sobre de GET jobs/{id}/candidates.
type listCandidatesResponse struct {
	Candidates []CandidateRecord `json:"candidates"`
}

// updateCandidatesRequest es el body de PATCH candidates.
type updateCandidatesRequest struct {
	Updates []CandidateUpdate `json:"updates"`
}

// updateCandidatesResponse es el sobre de PATCH candidates ({"updated": n}).
type updateCandidatesResponse struct {
	Updated int `json:"updated"`
}

// jobIdeasResponse es el sobre de GET jobs/{id}/ideas: las main_ideas agregadas del
// material (unión de los ChunkArtifacts de los chunks ya procesados), que la selección
// final (pasada 4, D-044.5) usa como cobertura objetivo. Puede venir vacío si el job aún
// no produjo ideas; el caller cae al agregado de source_ideas (ver SelectionPass).
type jobIdeasResponse struct {
	MainIdeas []string `json:"main_ideas"`
}

// chunkTextResponse es el sobre de GET chunks/{id}/text: el texto plano de un chunk ya
// procesado. Lo consume el candado verbatim local_only (D-044.4) cuando la pasada corre en
// modo "api" — la ruta de lectura que en F2 quedó como gancho (chunkTextResolver nil).
type chunkTextResponse struct {
	Text string `json:"text"`
}

// nextChunkEnvelope es el sobre real de GET chunks/pending: chunk puede ser null
// (nada pendiente) y prev_summary viaja al lado del chunk.
type nextChunkEnvelope struct {
	Chunk       *NextChunk `json:"chunk"`
	PrevSummary *string    `json:"prev_summary"`
}

// saveChunksRequest es el body de POST chunks.
type saveChunksRequest struct {
	Chunks []ChunkInput `json:"chunks"`
}

// saveChunkArtifactsRequest es el body de PUT chunks/{id}/artifacts.
type saveChunkArtifactsRequest struct {
	Summary    *string            `json:"summary"`
	Artifacts  json.RawMessage    `json:"artifacts"`
	Candidates []CandidatePayload `json:"candidates"`
}

// updateJobStatusRequest es el body de PATCH jobs/{id}/status.
type updateJobStatusRequest struct {
	Status    string  `json:"status"`
	Phase     int16   `json:"phase"`
	LastError *string `json:"last_error"`
}

// markChunkStatusRequest es el body de PUT chunks/{id}/status. Hoy solo se usa para
// aislar un chunk envenenado (status="failed" + motivo corto), plan 043 resiliencia
// fase A.
type markChunkStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// LearningPipelineClient es el cliente M2M hacia edugo-api-learning para el carril
// de materiales (plan 043 F1/F2): descarga fuente, porción y artefactos LLM.
// Autentica con un service JWT propio (audience edugo-api-learning, scope
// materials.pipeline — un scope por riel, SOLID). Seguro para uso concurrente.
type LearningPipelineClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// LearningPipelineClientConfig configura el cliente.
type LearningPipelineClientConfig struct {
	// BaseURL de learning (ej. http://localhost:8065).
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// TokenProvider firma/obtiene el service JWT (audience edugo-api-learning, scope
	// materials.pipeline).
	TokenProvider TokenProvider
}

// NewLearningPipelineClient construye el cliente.
func NewLearningPipelineClient(cfg LearningPipelineClientConfig) *LearningPipelineClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LearningPipelineClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
	}
}

// GetJob lee el estado de un job del pipeline. 404 (job inexistente) → ErrLearningPermanent.
func (c *LearningPipelineClient) GetJob(ctx context.Context, jobID string) (*PipelineJob, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineJobPathFmt, jobID)

	var out PipelineJob
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFileURL obtiene la URL firmada (presignada) del PDF del material del job.
func (c *LearningPipelineClient) GetFileURL(ctx context.Context, jobID string) (*PresignedFile, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineFileURLPathFmt, jobID)

	var out PresignedFile
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveChunks persiste las porciones del material (fase 0). Semántica de estado:
//   - 2xx → persistido; nil.
//   - 409 → ErrPipelineConflict: los chunks ya están cerrados (un redelivery pisando
//     el porcionado ya hecho); el caller decide (típicamente ACK/abstención).
//   - 404 y otros 4xx (salvo 408/429) → ErrLearningPermanent.
//   - 5xx / red / timeout / 408 / 429 → transitorio.
func (c *LearningPipelineClient) SaveChunks(ctx context.Context, jobID string, chunks []ChunkInput) error {
	if jobID == "" {
		return fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineChunksPathFmt, jobID)
	return c.do(ctx, http.MethodPost, url, saveChunksRequest{Chunks: chunks}, nil)
}

// GetNextPendingChunk devuelve el siguiente chunk pendiente de procesar. Si no hay
// ninguno ({"chunk":null}), devuelve (nil, nil) sin error. prev_summary (el resumen
// del chunk anterior) se adjunta en NextChunk.PrevSummary.
func (c *LearningPipelineClient) GetNextPendingChunk(ctx context.Context, jobID string) (*NextChunk, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelinePendingChunkPathFmt, jobID)

	var env nextChunkEnvelope
	if err := c.do(ctx, http.MethodGet, url, nil, &env); err != nil {
		return nil, err
	}
	if env.Chunk == nil {
		return nil, nil
	}
	env.Chunk.PrevSummary = env.PrevSummary
	return env.Chunk, nil
}

// SaveChunkArtifacts persiste el resumen, los artefactos y los candidatos de ítems
// producidos por el LLM para un chunk (lo usa F3). 409 → ErrPipelineConflict.
func (c *LearningPipelineClient) SaveChunkArtifacts(ctx context.Context, chunkID string, summary *string, artifacts json.RawMessage, candidates []CandidatePayload) error {
	if chunkID == "" {
		return fmt.Errorf("chunk_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineArtifactsPathFmt, chunkID)
	return c.do(ctx, http.MethodPut, url, saveChunkArtifactsRequest{
		Summary:    summary,
		Artifacts:  artifacts,
		Candidates: candidates,
	}, nil)
}

// MarkChunkFailed marca un chunk como `failed` en learning para AISLARLO (resiliencia
// fase A del pipeline, plan 043): un trozo que el LLM no logra digerir con calidad ni al
// reintentar se saca de la cola de pendientes sin tumbar el job entero. Semántica de
// estado (idéntica a la de las demás rutas del carril):
//   - 2xx → marcado; nil.
//   - 409 → ErrPipelineConflict: el chunk ya está `done` (una carrera lo cerró antes);
//     el caller lo trata como benigno y continúa.
//   - 404 y otros 4xx → ErrLearningPermanent.
//   - 5xx / red / timeout → transitorio (el caller lo propaga: no se traga un fallo de
//     infraestructura al aislar).
func (c *LearningPipelineClient) MarkChunkFailed(ctx context.Context, chunkID, reason string) error {
	if chunkID == "" {
		return fmt.Errorf("chunk_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineChunkStatusPathFmt, chunkID)
	return c.do(ctx, http.MethodPut, url, markChunkStatusRequest{
		Status: "failed",
		Reason: reason,
	}, nil)
}

// UpdateJobStatus avanza el estado/fase del job (p.ej. a "processing" tras porcionar).
// 409 → ErrPipelineConflict (transición no válida desde el estado actual).
func (c *LearningPipelineClient) UpdateJobStatus(ctx context.Context, jobID, status string, phase int16, lastError *string) error {
	if jobID == "" {
		return fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineJobStatusPathFmt, jobID)
	return c.do(ctx, http.MethodPatch, url, updateJobStatusRequest{
		Status:    status,
		Phase:     phase,
		LastError: lastError,
	}, nil)
}

// ListCandidates lee todas las candidatas de un job con sus columnas de control del
// reduce (pasada 1 de dedupe, plan 044 D-044.2). El orden viene fijado por learning
// (chunk_sequence ASC, id ASC). Semántica de estado idéntica al resto del carril: 404
// (job inexistente) → ErrLearningPermanent; 5xx/red/timeout → transitorio.
func (c *LearningPipelineClient) ListCandidates(ctx context.Context, jobID string) ([]CandidateRecord, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineCandidatesPathFmt, jobID)

	var out listCandidatesResponse
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return out.Candidates, nil
}

// GetJobIdeas lee las main_ideas agregadas del material de un job — la cobertura objetivo
// de la selección final (pasada 4, D-044.5). Devuelve la lista tal cual la agrega learning
// (unión de los ChunkArtifacts de los chunks procesados). Puede venir vacía (job sin ideas
// aún): el caller (SelectionPass) cae entonces al agregado de source_ideas. Semántica de
// estado idéntica al resto del carril: 404 → ErrLearningPermanent; 5xx/red/timeout →
// transitorio.
func (c *LearningPipelineClient) GetJobIdeas(ctx context.Context, jobID string) ([]string, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineJobIdeasPathFmt, jobID)

	var out jobIdeasResponse
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return nil, err
	}
	return out.MainIdeas, nil
}

// GetChunkText lee el texto plano de un chunk ya procesado — la ruta de lectura que en F2
// quedó como gancho (chunkTextResolver nil) para el candado verbatim local_only (D-044.4).
// Semántica de estado idéntica al resto del carril: 404 → ErrLearningPermanent;
// 5xx/red/timeout → transitorio.
func (c *LearningPipelineClient) GetChunkText(ctx context.Context, chunkID string) (string, error) {
	if chunkID == "" {
		return "", fmt.Errorf("chunk_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pipelineChunkTextPathFmt, chunkID)

	var out chunkTextResponse
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// UpdateCandidates persiste cambios parciales a un lote de candidatas (embedding,
// score, status, dedupe_group) — el reduce lo usa para guardar embeddings calculados y
// el resultado del agrupado. Devuelve cuántas filas actualizó learning. Un lote vacío
// no llama a la red (0, nil). Semántica de estado: 409 → ErrPipelineConflict (una
// candidata ya terminal cuyo status se intentó cambiar a otro valor; el batch es
// atómico y no se aplicó nada — el caller decide); 4xx → ErrLearningPermanent;
// 5xx/red/timeout → transitorio.
func (c *LearningPipelineClient) UpdateCandidates(ctx context.Context, updates []CandidateUpdate) (int, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	url := c.baseURL + pipelineCandidatesPath

	var out updateCandidatesResponse
	if err := c.do(ctx, http.MethodPatch, url, updateCandidatesRequest{Updates: updates}, &out); err != nil {
		return 0, err
	}
	return out.Updated, nil
}

// do ejecuta una request M2M autenticada y (si out != nil) decodifica la respuesta.
// Clasifica el estado: 2xx OK; 409 → ErrPipelineConflict (guard de estado, el caller
// decide); 4xx salvo 408/429 → ErrLearningPermanent (permanente, lo trata retry.go);
// resto (5xx, 408, 429, red, timeout) → transitorio sin sentinel. NUNCA loguea el token.
func (c *LearningPipelineClient) do(ctx context.Context, method, url string, body any, out any) error {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return fmt.Errorf("obtaining service token: %w", err)
	}

	var reader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling learning request: %w", err)
		}
		reader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("creating learning request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("learning request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading learning response: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("parsing learning response: %w", err)
			}
		}
		return nil
	case resp.StatusCode == http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrPipelineConflict, strings.TrimSpace(string(respBody)))
	case resp.StatusCode >= 400 && resp.StatusCode < 500 &&
		resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests:
		return fmt.Errorf("%w: pipeline status %d: %s", ErrLearningPermanent, resp.StatusCode, strings.TrimSpace(string(respBody)))
	default:
		return fmt.Errorf("pipeline returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
}
