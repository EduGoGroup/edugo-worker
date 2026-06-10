package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Contrato del Notification Gateway (plan 020 D13, design §4). Es el MISMO
// contrato que valida edugo-api-platform en F2.2; se mantiene aquí como
// duplicado controlado (D17/D18: se evitó crear un módulo shared contracts).
// Cualquier cambio debe sincronizarse con el DTO de platform.

// DispatchRecipient identifica a un destinatario por user_id.
type DispatchRecipient struct {
	UserID string `json:"user_id"`
}

// DispatchNotification es el contenido de la notificación in-app.
type DispatchNotification struct {
	Type         string `json:"type"`
	Title        string `json:"title"`
	Body         string `json:"body,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
}

// DispatchChannels controla los canales de entrega.
type DispatchChannels struct {
	InApp bool `json:"in_app"`
	Push  bool `json:"push"`
}

// DispatchSource es metadata de trazabilidad del caller.
type DispatchSource struct {
	Caller        string `json:"caller,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// DispatchRequest es el cuerpo de POST /api/v1/internal/notifications/dispatch.
type DispatchRequest struct {
	IdempotencyKey string               `json:"idempotency_key,omitempty"`
	Recipients     []DispatchRecipient  `json:"recipients"`
	Notification   DispatchNotification `json:"notification"`
	Channels       *DispatchChannels    `json:"channels,omitempty"`
	PushData       map[string]string    `json:"push_data,omitempty"`
	Source         *DispatchSource      `json:"source,omitempty"`
}

// TokenProvider entrega un service JWT para autenticar la llamada M2M.
type TokenProvider interface {
	Token() (string, error)
}

// NotificationDispatchClient delega la entrega de notificaciones al Notification
// Gateway (edugo-api-platform). El worker ya NO escribe notifications.* ni tiene
// credenciales FCM/APNs: solo arma el request y lo envía con un service JWT (D13).
type NotificationDispatchClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// NotificationDispatchClientConfig configura el cliente.
type NotificationDispatchClientConfig struct {
	// BaseURL del gateway platform (ej. http://localhost:8075).
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// TokenProvider firma/obtiene el service JWT.
	TokenProvider TokenProvider
}

const dispatchEndpoint = "/api/v1/internal/notifications/dispatch"

// NewNotificationDispatchClient crea el cliente.
func NewNotificationDispatchClient(cfg NotificationDispatchClientConfig) *NotificationDispatchClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &NotificationDispatchClient{
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: timeout},
		tokenProvider: cfg.TokenProvider,
	}
}

// Dispatch envía el request al gateway. Devuelve error en fallo de red, timeout o
// status no-2xx, de modo que el processor propague el error y RabbitMQ reintente
// (la idempotencia en platform protege contra duplicados). NUNCA loguea el token.
func (c *NotificationDispatchClient) Dispatch(ctx context.Context, req DispatchRequest) error {
	token, err := c.tokenProvider.Token()
	if err != nil {
		return fmt.Errorf("obtaining service token: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling dispatch request: %w", err)
	}

	url := c.baseURL + dispatchEndpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating dispatch request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Fallo de red/timeout → reintentable.
		return fmt.Errorf("dispatch request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Leer un fragmento del cuerpo para diagnóstico (sin exponer el token).
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("dispatch returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}
