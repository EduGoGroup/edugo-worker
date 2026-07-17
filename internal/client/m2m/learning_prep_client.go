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

// ErrPrepHashConflict marca un 409 del PUT llm-prep: el source_hash con el que el
// worker trabajó ya NO coincide con el de la pregunta (el profesor editó en medio,
// D-042.5). No es un fallo: el update ya re-encoló con datos frescos, así que el
// worker DEBE ABSTENERSE (descartar el prep viejo y ACKear). El processor lo
// convierte en ACK; el clasificador de retry no lo ve.
var ErrPrepHashConflict = errors.New("source_hash del prep ya no coincide (la pregunta se editó; el update re-encoló)")

// Rutas del carril de preparación en learning (plan 042 F1b). Base = api_learning.base_url.
const (
	prepSourcePathFmt = "/api/v1/internal/questions/%s/prep-source"
	savePrepPathFmt   = "/api/v1/internal/questions/%s/llm-prep"
)

// PrepSourceResponse es la fuente que el worker lee para preparar una pregunta
// (GET prep-source). Espeja el DTO de learning. Los punteros vienen nil cuando la
// canónica/explicación/feedback no existen.
type PrepSourceResponse struct {
	QuestionID      string  `json:"question_id"`
	AssessmentID    string  `json:"assessment_id"`
	SchoolID        string  `json:"school_id"`
	QuestionType    string  `json:"question_type"`
	QuestionText    string  `json:"question_text"`
	CorrectAnswer   *string `json:"correct_answer,omitempty"`
	Explanation     *string `json:"explanation,omitempty"`
	LLMPrepFeedback *string `json:"llm_prep_feedback,omitempty"`
	SourceHash      string  `json:"source_hash"`
}

// SavePrepRequest es el cuerpo del PUT llm-prep. LLMPrep es el JSON crudo YA validado
// contra el contrato v1; SourceHash es el hash con el que trabajó el worker (409 si no
// coincide); ConsumedFeedback marca que el prompt usó el comentario del profesor y
// learning debe limpiarlo (D-042.7).
type SavePrepRequest struct {
	LLMPrep          json.RawMessage `json:"llm_prep"`
	SourceHash       string          `json:"source_hash"`
	ConsumedFeedback bool            `json:"consumed_feedback"`
}

// LearningPrepClient es el cliente M2M hacia edugo-api-learning para el carril de
// preparación (plan 042 F2). Canal y scope propios (questions.prep), independientes
// del carril de revisión (SOLID: un scope por riel). Seguro para uso concurrente.
type LearningPrepClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// LearningPrepClientConfig configura el cliente.
type LearningPrepClientConfig struct {
	// BaseURL de learning (ej. http://localhost:8065).
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// TokenProvider firma/obtiene el service JWT (audience edugo-api-learning, scope
	// questions.prep — distinto del de revisión).
	TokenProvider TokenProvider
}

// NewLearningPrepClient construye el cliente.
func NewLearningPrepClient(cfg LearningPrepClientConfig) *LearningPrepClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LearningPrepClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
	}
}

// GetPrepSource lee la fuente para preparar una pregunta. Un 404 (pregunta borrada)
// llega como ErrLearningPermanent (no reintentar).
func (c *LearningPrepClient) GetPrepSource(ctx context.Context, questionID string) (PrepSourceResponse, error) {
	if questionID == "" {
		return PrepSourceResponse{}, fmt.Errorf("question_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(prepSourcePathFmt, questionID)

	var out PrepSourceResponse
	if err := c.getJSON(ctx, url, &out); err != nil {
		return PrepSourceResponse{}, err
	}
	return out, nil
}

// SavePrep persiste el artefacto preparado con concurrencia optimista por hash.
// Semántica de estado (D-042.5):
//   - 200/2xx → persistido; nil.
//   - 409     → ErrPrepHashConflict: el hash ya no coincide; el caller ACKea (el
//     update ya re-encoló). No es fallo.
//   - 404 y otros 4xx (salvo 408/429) → ErrLearningPermanent (permanente).
//   - 5xx / red / timeout / 408 / 429 → error transitorio (reintentable).
func (c *LearningPrepClient) SavePrep(ctx context.Context, questionID string, req SavePrepRequest) error {
	if questionID == "" {
		return fmt.Errorf("question_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(savePrepPathFmt, questionID)

	status, body, err := c.putJSON(ctx, url, req)
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusConflict:
		return fmt.Errorf("%w: %s", ErrPrepHashConflict, strings.TrimSpace(string(body)))
	case status >= 400 && status < 500 &&
		status != http.StatusRequestTimeout && status != http.StatusTooManyRequests:
		return fmt.Errorf("%w: save-prep status %d: %s", ErrLearningPermanent, status, strings.TrimSpace(string(body)))
	default:
		return fmt.Errorf("save-prep returned status %d: %s", status, strings.TrimSpace(string(body)))
	}
}

// getJSON ejecuta un GET M2M autenticado y decodifica la respuesta. Clasifica el
// estado como do(): 2xx OK; 4xx (salvo 408/429) → ErrLearningPermanent; resto →
// transitorio. NUNCA loguea el token.
func (c *LearningPrepClient) getJSON(ctx context.Context, url string, out any) error {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return fmt.Errorf("obtaining service token: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating learning request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("learning request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading learning response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("learning returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 &&
			resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests {
			return fmt.Errorf("%w: %s", ErrLearningPermanent, msg)
		}
		return errors.New(msg)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parsing learning response: %w", err)
		}
	}
	return nil
}

// putJSON ejecuta un PUT M2M autenticado con cuerpo JSON y devuelve el código de
// estado y el cuerpo crudo SIN clasificar: SavePrep tiene semántica de estado propia
// (409 = conflicto de hash, no error genérico). NUNCA loguea el token.
func (c *LearningPrepClient) putJSON(ctx context.Context, url string, body any) (int, []byte, error) {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("obtaining service token: %w", err)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return 0, nil, fmt.Errorf("marshaling learning request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("creating learning request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("learning request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("reading learning response: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
