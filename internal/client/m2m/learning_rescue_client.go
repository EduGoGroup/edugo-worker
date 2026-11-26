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

// Rutas del rescate de lo fallado en learning (HU-IA.6). Base = api_learning.base_url.
const (
	rescueReportPath = "/api/v1/internal/ai-failures/report"
	rescueSweepPath  = "/api/v1/internal/ai-failures/sweep"
)

// Vocabulario del aviso (contrato de la HU-IA.6, §3 de la ficha).
const (
	RescueTargetAttempt  = "attempt"
	RescueTargetQuestion = "question"

	RescueKindFailure       = "failure"
	RescueKindNotApplicable = "not_applicable"

	RescueCauseService = "service"
	RescueCauseData    = "data"
)

// ErrRescueTargetNotFound marca el 404 del aviso: el intento o la pregunta ya no existe.
// Envuelve ErrLearningPermanent: reintentar no lo arregla.
var ErrRescueTargetNotFound = fmt.Errorf("%w: el destino del aviso no existe", ErrLearningPermanent)

// FailureReportRequest es el cuerpo del aviso. En not_applicable, CauseClass y Error van
// vacíos y se omiten.
type FailureReportRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Kind       string `json:"kind"`
	CauseClass string `json:"cause_class,omitempty"`
	Error      string `json:"error,omitempty"`
}

// FailureReportResponse es la respuesta del aviso: el estado de la fila (retrying,
// failed, resolved, cancelled) o ignored.
type FailureReportResponse struct {
	Status string `json:"status"`
}

// SweepRequest lleva los números del barrido (P9): viajan en cada llamada, learning solo
// exige que la edad mínima supere el candado.
type SweepRequest struct {
	MaxRetries             int     `json:"max_retries"`
	RetryBackoffSeconds    []int64 `json:"retry_backoff_seconds"`
	MinAgeSeconds          int64   `json:"min_age_seconds"`
	JobPendingStaleSeconds int64   `json:"job_pending_stale_seconds"`
}

// SweepResponse son los conteos de un barrido.
type SweepResponse struct {
	Resolved    int `json:"resolved"`
	Exhausted   int `json:"exhausted"`
	Republished int `json:"republished"`
	Orphans     int `json:"orphans"`
	JobsFailed  int `json:"jobs_failed"`
}

// LearningRescueClient es el cliente M2M del rescate (HU-IA.6): grupo y scope propios
// (ai_failures.rescue), como los otros tres carriles. Seguro para uso concurrente.
type LearningRescueClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// LearningRescueClientConfig configura el cliente.
type LearningRescueClientConfig struct {
	// BaseURL de learning (ej. http://localhost:8065).
	BaseURL string
	// Timeout de la request HTTP (default 30s: el primer barrido es retroactivo).
	Timeout time.Duration
	// TokenProvider firma el service JWT (audience edugo-api-learning, scope
	// ai_failures.rescue).
	TokenProvider TokenProvider
}

// NewLearningRescueClient construye el cliente.
func NewLearningRescueClient(cfg LearningRescueClientConfig) *LearningRescueClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &LearningRescueClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
	}
}

// Report avisa un fallo o un «no aplica».
//   - 2xx → el estado de la fila.
//   - 404 → ErrRescueTargetNotFound (permanente).
//   - otros 4xx (salvo 408/429) → ErrLearningPermanent.
//   - 5xx / red / 408 / 429 → transitorio.
func (c *LearningRescueClient) Report(ctx context.Context, req FailureReportRequest) (FailureReportResponse, error) {
	if req.TargetID == "" {
		return FailureReportResponse{}, errors.New("target_id vacío")
	}
	var out FailureReportResponse
	if err := c.postJSON(ctx, rescueReportPath, req, &out); err != nil {
		return FailureReportResponse{}, err
	}
	return out, nil
}

// Sweep pide un barrido con los números dados. Clasifica el estado como Report (un 400
// por una edad mínima que no supera el candado es ErrLearningPermanent).
func (c *LearningRescueClient) Sweep(ctx context.Context, req SweepRequest) (SweepResponse, error) {
	var out SweepResponse
	if err := c.postJSON(ctx, rescueSweepPath, req, &out); err != nil {
		return SweepResponse{}, err
	}
	return out, nil
}

// postJSON ejecuta un POST M2M autenticado, clasifica el estado y decodifica la
// respuesta. NUNCA loguea el token.
func (c *LearningRescueClient) postJSON(ctx context.Context, path string, body, out any) error {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return fmt.Errorf("obtaining service token: %w", err)
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling learning request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating learning request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("learning request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading learning response: %w", err)
	}

	status := resp.StatusCode
	msg := fmt.Sprintf("%s returned status %d: %s", path, status, strings.TrimSpace(string(respBody)))
	switch {
	case status >= 200 && status < 300:
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrRescueTargetNotFound, msg)
	case status >= 400 && status < 500 &&
		status != http.StatusRequestTimeout && status != http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrLearningPermanent, msg)
	default:
		return errors.New(msg)
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parsing learning response: %w", err)
	}
	return nil
}
