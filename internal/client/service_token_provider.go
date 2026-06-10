// Package client proporciona clientes HTTP para comunicación con otros servicios.
package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/EduGoGroup/edugo-shared/auth"
)

// ServiceTokenConfig configura el ServiceTokenProvider (auth M2M, plan 020 D14/D15).
type ServiceTokenConfig struct {
	// Secret es el SERVICE_JWT_SECRET (HS256) compartido con el gateway platform.
	// Distinto del secret de usuarios. En alpha el worker firma el token con este
	// secret (modelo de secret compartido del design §5); el flujo client_secret →
	// identity /oauth/token es la evolución de mediano plazo.
	Secret string
	// Issuer emisor del token (ej. "edugo-identity").
	Issuer string
	// Audience servicio destino esperado por el gateway (ej. "edugo-api-platform").
	Audience string
	// ClientID identificador del cliente M2M (ej. "edugo-worker"); debe existir
	// en auth.service_clients.
	ClientID string
	// Scopes concedidos al token (ej. ["notifications.dispatch"]).
	Scopes []string
	// TTL duración del token antes de expirar (recomendado ~15 min).
	TTL time.Duration
}

// ServiceTokenProvider obtiene (firmando localmente) y cachea un service JWT M2M
// para autenticar las llamadas del worker al Notification Gateway. Reutiliza
// auth.ServiceJWTManager de edugo-shared (HS256). Es seguro para uso concurrente.
type ServiceTokenProvider struct {
	manager  *auth.ServiceJWTManager
	clientID string
	scopes   []string
	ttl      time.Duration
	leeway   time.Duration
	now      func() time.Time

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// NewServiceTokenProvider crea el provider. Falla si falta el ClientID; un Secret
// vacío se permite (el gateway rechazará el token), para no impedir el arranque
// del worker en entornos sin el secret aún provisionado.
func NewServiceTokenProvider(cfg ServiceTokenConfig) (*ServiceTokenProvider, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("service token: ClientID es obligatorio")
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	leeway := 60 * time.Second
	if leeway > ttl/2 {
		leeway = ttl / 2
	}

	return &ServiceTokenProvider{
		manager:  auth.NewServiceJWTManager(cfg.Secret, cfg.Issuer, cfg.Audience),
		clientID: cfg.ClientID,
		scopes:   cfg.Scopes,
		ttl:      ttl,
		leeway:   leeway,
		now:      time.Now,
	}, nil
}

// Token devuelve un service JWT válido, renovándolo si está por expirar.
func (p *ServiceTokenProvider) Token() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && p.now().Before(p.expiresAt.Add(-p.leeway)) {
		return p.token, nil
	}

	token, expiresAt, err := p.manager.GenerateServiceToken(p.clientID, p.scopes, p.ttl)
	if err != nil {
		return "", fmt.Errorf("firmando service token: %w", err)
	}

	p.token = token
	p.expiresAt = expiresAt
	return token, nil
}
