package m2m

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/auth"
)

func TestServiceTokenProvider_GeneratesValidToken(t *testing.T) {
	const secret = "test-secret"
	p, err := NewServiceTokenProvider(ServiceTokenConfig{
		Secret:   secret,
		Issuer:   "edugo-identity",
		Audience: "edugo-api-academic",
		ClientID: "edugo-worker",
		Scopes:   []string{"schools.settings.read"},
		TTL:      15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServiceTokenProvider falló: %v", err)
	}

	tok, err := p.Token()
	if err != nil {
		t.Fatalf("Token falló: %v", err)
	}

	// El token debe validar con un manager que espere el mismo iss/aud/secret.
	mgr := auth.NewServiceJWTManager(secret, "edugo-identity", "edugo-api-academic")
	claims, err := mgr.ValidateServiceToken(tok)
	if err != nil {
		t.Fatalf("token no valida: %v", err)
	}
	if claims.ClientID != "edugo-worker" {
		t.Fatalf("client_id inesperado: %s", claims.ClientID)
	}
	if !claims.HasScope("schools.settings.read") {
		t.Fatalf("falta el scope esperado; scopes: %v", claims.Scopes)
	}
}

func TestServiceTokenProvider_CachesToken(t *testing.T) {
	p, _ := NewServiceTokenProvider(ServiceTokenConfig{Secret: "s", ClientID: "edugo-worker"})
	a, _ := p.Token()
	b, _ := p.Token()
	if a != b {
		t.Fatal("esperaba el mismo token cacheado en llamadas consecutivas")
	}
}

func TestServiceTokenProvider_RequiresClientID(t *testing.T) {
	if _, err := NewServiceTokenProvider(ServiceTokenConfig{Secret: "s"}); err == nil {
		t.Fatal("esperaba error por ClientID vacío")
	}
}

type staticToken struct{ v string }

func (s staticToken) Token() (string, error) { return s.v, nil }

func TestSettingsClient_GetSettings_OK(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Method != http.MethodGet {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/internal/schools/") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok-123" {
			t.Errorf("Authorization inesperada: %q", r.Header.Get("Authorization"))
		}
		resp := SchoolSettings{
			SchoolID: "school-1",
			Settings: []ResolvedSetting{
				{Key: "llm.review.mode", Value: "api", Source: "school"},
				{Key: "llm.review.flow", Value: "teacher", Source: "default"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewSettingsClient(SettingsClientConfig{
		BaseURL:       srv.URL,
		TokenProvider: staticToken{"tok-123"},
		CacheTTL:      time.Minute,
	})

	got, err := c.GetSettings(context.Background(), "school-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if v, ok := got.Get("llm.review.mode"); !ok || v != "api" {
		t.Fatalf("valor resuelto inesperado: %q ok=%v", v, ok)
	}

	// Segunda llamada: debe servirse de caché (sin golpear academic).
	if _, err := c.GetSettings(context.Background(), "school-1"); err != nil {
		t.Fatalf("error inesperado (cache): %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("esperaba 1 llamada HTTP (resto cache), hubo %d", n)
	}
}

func TestSettingsClient_CacheExpires(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_ = json.NewEncoder(w).Encode(SchoolSettings{SchoolID: "s"})
	}))
	defer srv.Close()

	c := NewSettingsClient(SettingsClientConfig{
		BaseURL:       srv.URL,
		TokenProvider: staticToken{"t"},
		CacheTTL:      1 * time.Millisecond,
	})
	_, _ = c.GetSettings(context.Background(), "s")
	time.Sleep(5 * time.Millisecond)
	_, _ = c.GetSettings(context.Background(), "s")
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Fatalf("esperaba 2 llamadas (cache expirada), hubo %d", n)
	}
}

func TestSettingsClient_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewSettingsClient(SettingsClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	if _, err := c.GetSettings(context.Background(), "s"); err == nil {
		t.Fatal("esperaba error por status 403")
	}
}

func TestSettingsClient_EmptySchoolID(t *testing.T) {
	c := NewSettingsClient(SettingsClientConfig{BaseURL: "http://x", TokenProvider: staticToken{"t"}})
	if _, err := c.GetSettings(context.Background(), ""); err == nil {
		t.Fatal("esperaba error por school_id vacío")
	}
}

func TestLearningClient_Stub(t *testing.T) {
	c := NewLearningClient(LearningClientConfig{BaseURL: "http://x", TokenProvider: staticToken{"t"}})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping stub no debe fallar: %v", err)
	}
}
