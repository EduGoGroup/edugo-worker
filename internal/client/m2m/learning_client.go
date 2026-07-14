package m2m

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// LearningClient es el STUB del cliente M2M hacia edugo-api-learning. El plan 039
// solo estrena el andamiaje (D-039.7): el cliente concreto —qué endpoints de
// learning consume el worker para el carril de corrección/generación— lo define
// y llena el plan 040. Se deja aquí, construible y con el token provider ya
// cableado, para que 040 no tenga que crear el paquete desde cero.
type LearningClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// LearningClientConfig configura el stub.
type LearningClientConfig struct {
	// BaseURL de learning (ej. http://localhost:8065).
	BaseURL string
	// Timeout de la request HTTP (default 5s).
	Timeout time.Duration
	// TokenProvider firma/obtiene el service JWT (scopes del carril 040).
	TokenProvider TokenProvider
}

// NewLearningClient construye el stub. No expone operaciones todavía: las agrega
// el plan 040 cuando se defina el contrato del carril.
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

// Ping es un placeholder no-op para que el stub no quede sin métodos y el wiring
// pueda referenciarlo. El plan 040 lo reemplaza por las operaciones reales.
func (c *LearningClient) Ping(_ context.Context) error { return nil }
