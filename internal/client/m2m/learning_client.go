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

// ErrLearningPermanent marca una respuesta 4xx de learning (salvo 408/429): la
// request está malformada o no aplica (p.ej. answer/attempt inexistente, scope
// insuficiente). Reintentar no la arregla; el clasificador de retry (retry.go) la
// trata como permanente. El resto de fallos (5xx, red, timeout, 408/429) son
// transitorios y se devuelven sin envolver con este sentinel.
var ErrLearningPermanent = errors.New("learning respondió error permanente (4xx)")

// Rutas de los endpoints M2M en learning (plan 040 F2). Base = api_learning.base_url.
const (
	pendingAnswersPathFmt  = "/api/v1/internal/attempts/%s/answers?review=pending"
	answerReviewPathFmt    = "/api/v1/internal/attempts/%s/answers/%s/review"
	finalizeAttemptPathFmt = "/api/v1/internal/attempts/%s/finalize"
)

// PendingAnswer es una respuesta pendiente de revisión asistida (solo open_ended,
// según el contrato M2M) con todo lo que el LLM necesita para corregir.
type PendingAnswer struct {
	AnswerID       string  `json:"answer_id"`
	QuestionID     string  `json:"question_id"`
	QuestionType   string  `json:"question_type"`
	QuestionText   string  `json:"question_text"`
	Rubric         string  `json:"rubric"`
	ExpectedAnswer string  `json:"expected_answer"`
	StudentAnswer  string  `json:"student_answer"`
	Points         float64 `json:"points"`
}

// PendingAnswersResponse es la respuesta de GET answers?review=pending. Answers
// puede venir vacío (nada pendiente o revisión ya aplicada por un redelivery).
type PendingAnswersResponse struct {
	AttemptID    string          `json:"attempt_id"`
	AssessmentID string          `json:"assessment_id"`
	SchoolID     string          `json:"school_id"`
	Status       string          `json:"status"`
	Answers      []PendingAnswer `json:"answers"`
}

// AnswerReviewRequest es el body de POST answers/{answerID}/review. Idempotente
// (upsert) del lado de learning: reintentar es seguro.
type AnswerReviewRequest struct {
	PointsAwarded float64 `json:"points_awarded"`
	Feedback      string  `json:"feedback"`
}

// AnswerReviewResponse es la respuesta de POST review.
type AnswerReviewResponse struct {
	AnswerID     string `json:"answer_id"`
	ReviewStatus string `json:"review_status"`
}

// FinalizeResponse es la respuesta de POST finalize. Si el attempt ya estaba
// completed, learning devuelve 200 no-op (idempotente).
type FinalizeResponse struct {
	AttemptID string `json:"attempt_id"`
	Status    string `json:"status"`
}

// LearningClient es el cliente M2M hacia edugo-api-learning para el carril de
// revisión asistida (plan 040 F2). Autentica con un service JWT propio (audience
// edugo-api-learning, scope attempts.review). Seguro para uso concurrente (el
// http.Client lo es).
type LearningClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// LearningClientConfig configura el cliente.
type LearningClientConfig struct {
	// BaseURL de learning (ej. http://localhost:8065).
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// TokenProvider firma/obtiene el service JWT (audience edugo-api-learning,
	// scope attempts.review).
	TokenProvider TokenProvider
}

// NewLearningClient construye el cliente.
func NewLearningClient(cfg LearningClientConfig) *LearningClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &LearningClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
	}
}

// GetPendingAnswers lee las respuestas pendientes de revisión de un intento.
func (c *LearningClient) GetPendingAnswers(ctx context.Context, attemptID string) (PendingAnswersResponse, error) {
	if attemptID == "" {
		return PendingAnswersResponse{}, fmt.Errorf("attempt_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(pendingAnswersPathFmt, attemptID)

	var out PendingAnswersResponse
	if err := c.do(ctx, http.MethodGet, url, nil, &out); err != nil {
		return PendingAnswersResponse{}, err
	}
	return out, nil
}

// PostAnswerReview escribe la corrección (puntaje + feedback) de una respuesta.
// Idempotente del lado de learning: reintentar tras un fallo posterior es seguro.
func (c *LearningClient) PostAnswerReview(ctx context.Context, attemptID, answerID string, review AnswerReviewRequest) (AnswerReviewResponse, error) {
	if attemptID == "" || answerID == "" {
		return AnswerReviewResponse{}, fmt.Errorf("attempt_id/answer_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(answerReviewPathFmt, attemptID, answerID)

	var out AnswerReviewResponse
	if err := c.do(ctx, http.MethodPost, url, review, &out); err != nil {
		return AnswerReviewResponse{}, err
	}
	return out, nil
}

// FinalizeAttempt cierra la revisión del intento (flow direct). Idempotente: si ya
// estaba completed, learning devuelve 200 no-op.
func (c *LearningClient) FinalizeAttempt(ctx context.Context, attemptID string) (FinalizeResponse, error) {
	if attemptID == "" {
		return FinalizeResponse{}, fmt.Errorf("attempt_id vacío")
	}
	url := c.baseURL + fmt.Sprintf(finalizeAttemptPathFmt, attemptID)

	var out FinalizeResponse
	if err := c.do(ctx, http.MethodPost, url, struct{}{}, &out); err != nil {
		return FinalizeResponse{}, err
	}
	return out, nil
}

// do ejecuta una request M2M autenticada y decodifica la respuesta JSON. Clasifica
// el estado: 2xx OK; 4xx (salvo 408/429) → ErrLearningPermanent; resto → error
// transitorio (sin sentinel). NUNCA loguea el token.
func (c *LearningClient) do(ctx context.Context, method, url string, body any, out any) error {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("learning returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		// 4xx salvo 408 (timeout) y 429 (rate limit) es permanente: reintentar no lo
		// arregla. El resto (5xx, 408, 429) es transitorio.
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
