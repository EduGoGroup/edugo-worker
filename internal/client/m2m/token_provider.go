// Package m2m agrupa los clientes máquina-a-máquina que el worker estrena en el
// plan 039 (D-039.7): un provider de service JWT y el cliente de lectura de
// settings hacia academic. Hasta ahora el worker solo hablaba con identity
// (AuthClient). Cero SQL: el worker sigue siendo orquestador puro.
package m2m

import (
	"fmt"
	"sync"
	"time"

	"github.com/EduGoGroup/edugo-shared/auth"
)

// ServiceTokenConfig configura el ServiceTokenProvider. Espeja el patrón de
// edugo-api-learning (plan 032): misma firma HS256 con auth.ServiceJWTManager de
// edugo-shared. Los valores se inyectan desde bootstrap/config (env
// SERVICE_JWT_SECRET, issuer/audience por convención del ecosistema).
type ServiceTokenConfig struct {
	// Secret es el SERVICE_JWT_SECRET (HS256), distinto del secret de usuarios.
	Secret string
	// Issuer emisor del token (convención del ecosistema: "edugo-identity").
	Issuer string
	// Audience servicio destino esperado por el receptor (ej. "edugo-api-academic").
	Audience string
	// ClientID identificador del cliente M2M (ej. "edugo-worker").
	ClientID string
	// Scopes concedidos al token (ej. ["schools.settings.read"]).
	Scopes []string
	// TTL duración del token antes de expirar (default 15 min).
	TTL time.Duration
}

// ServiceTokenProvider firma localmente y cachea un service JWT M2M. Es seguro
// para uso concurrente. Renueva el token antes de que expire (leeway).
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

// NewServiceTokenProvider crea el provider. Falla si falta ClientID; un Secret
// vacío se permite (el receptor rechazará el token) para no impedir el arranque
// del worker en entornos sin el secret provisionado aún.
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

// Token devuelve un service JWT válido, renovándolo si está por expirar. NUNCA
// lo loguea el caller (patrón dispatch_client de learning).
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
