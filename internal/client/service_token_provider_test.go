package client

import (
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/auth"
)

func TestServiceTokenProvider_GeneratesValidToken(t *testing.T) {
	const secret = "service-jwt-test-secret-min-32-characters!"
	provider, err := NewServiceTokenProvider(ServiceTokenConfig{
		Secret:   secret,
		Issuer:   "edugo-identity",
		Audience: "edugo-api-platform",
		ClientID: "edugo-worker",
		Scopes:   []string{"notifications.dispatch"},
		TTL:      15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServiceTokenProvider: %v", err)
	}

	token, err := provider.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}

	// El gateway valida con el mismo secret/iss/aud.
	mgr := auth.NewServiceJWTManager(secret, "edugo-identity", "edugo-api-platform")
	claims, err := mgr.ValidateServiceToken(token)
	if err != nil {
		t.Fatalf("ValidateServiceToken: %v", err)
	}
	if claims.ClientID != "edugo-worker" {
		t.Errorf("client_id=%q", claims.ClientID)
	}
	if !claims.HasScope("notifications.dispatch") {
		t.Errorf("falta scope notifications.dispatch: %v", claims.Scopes)
	}
}

func TestServiceTokenProvider_CachesToken(t *testing.T) {
	provider, err := NewServiceTokenProvider(ServiceTokenConfig{
		Secret:   "service-jwt-test-secret-min-32-characters!",
		Issuer:   "edugo-identity",
		Audience: "edugo-api-platform",
		ClientID: "edugo-worker",
		Scopes:   []string{"notifications.dispatch"},
		TTL:      15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServiceTokenProvider: %v", err)
	}

	current := time.Now()
	provider.now = func() time.Time { return current }

	t1, err := provider.Token()
	if err != nil {
		t.Fatalf("Token #1: %v", err)
	}
	t2, err := provider.Token()
	if err != nil {
		t.Fatalf("Token #2: %v", err)
	}
	if t1 != t2 {
		t.Error("el token debe cachearse mientras es válido")
	}

	// Tras superar TTL - leeway, se regenera (nuevo jti → token distinto).
	current = current.Add(16 * time.Minute)
	t3, err := provider.Token()
	if err != nil {
		t.Fatalf("Token #3: %v", err)
	}
	if t3 == t1 {
		t.Error("el token debe regenerarse tras expirar")
	}
}

func TestNewServiceTokenProvider_RequiresClientID(t *testing.T) {
	if _, err := NewServiceTokenProvider(ServiceTokenConfig{Secret: "x"}); err == nil {
		t.Error("ClientID vacío debe fallar")
	}
}
