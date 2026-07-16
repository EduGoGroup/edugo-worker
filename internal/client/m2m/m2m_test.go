package m2m

import (
	"context"
	"encoding/json"
	"errors"
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

func TestLearningClient_GetPendingAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método esperado GET, hubo %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/internal/attempts/att-1/answers") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.URL.Query().Get("review") != "pending" {
			t.Errorf("query review=pending ausente: %s", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer t" {
			t.Errorf("Authorization esperado 'Bearer t', hubo %q", got)
		}
		_, _ = w.Write([]byte(`{"attempt_id":"att-1","answers":[{"answer_id":"a1","points":10}]}`))
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	resp, err := c.GetPendingAnswers(context.Background(), "att-1")
	if err != nil {
		t.Fatalf("GetPendingAnswers falló: %v", err)
	}
	if len(resp.Answers) != 1 || resp.Answers[0].AnswerID != "a1" || resp.Answers[0].Points != 10 {
		t.Fatalf("respuesta inesperada: %+v", resp)
	}
}

func TestLearningClient_PostAnswerReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método esperado POST, hubo %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/answers/a1/review") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body AnswerReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body no decodificable: %v", err)
		}
		if body.PointsAwarded != 7.5 || body.Feedback != "bien" {
			t.Errorf("body inesperado: %+v", body)
		}
		_, _ = w.Write([]byte(`{"answer_id":"a1","review_status":"reviewed"}`))
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	resp, err := c.PostAnswerReview(context.Background(), "att-1", "a1", AnswerReviewRequest{PointsAwarded: 7.5, Feedback: "bien"})
	if err != nil {
		t.Fatalf("PostAnswerReview falló: %v", err)
	}
	if resp.ReviewStatus != "reviewed" {
		t.Fatalf("review_status inesperado: %q", resp.ReviewStatus)
	}
}

func TestLearningClient_FinalizeAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/attempts/att-1/finalize") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"attempt_id":"att-1","status":"completed"}`))
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	resp, err := c.FinalizeAttempt(context.Background(), "att-1")
	if err != nil {
		t.Fatalf("FinalizeAttempt falló: %v", err)
	}
	if resp.Status != "completed" {
		t.Fatalf("status inesperado: %q", resp.Status)
	}
}

func TestLearningClient_Claim_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método esperado POST, hubo %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/attempts/att-1/claim") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer t" {
			t.Errorf("Authorization esperado 'Bearer t', hubo %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	if err := c.Claim(context.Background(), "att-1"); err != nil {
		t.Fatalf("Claim 200 no debe fallar: %v", err)
	}
}

func TestLearningClient_Claim_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"candado ajeno vigente"}`))
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	err := c.Claim(context.Background(), "att-1")
	if !errors.Is(err, ErrClaimConflict) {
		t.Fatalf("409 debe envolver ErrClaimConflict, hubo: %v", err)
	}
	// 409 NO es permanente: el processor lo convierte en abstención, no en DLQ.
	if errors.Is(err, ErrLearningPermanent) {
		t.Fatal("409 de claim no debe ser ErrLearningPermanent")
	}
}

func TestLearningClient_Claim_NotFound_Permanente(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	err := c.Claim(context.Background(), "att-1")
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("404 de claim debe ser ErrLearningPermanent, hubo: %v", err)
	}
}

func TestLearningClient_ReleaseClaim_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/attempts/att-1/release-claim") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	if err := c.ReleaseClaim(context.Background(), "att-1"); err != nil {
		t.Fatalf("ReleaseClaim 200 no debe fallar: %v", err)
	}
}

func TestLearningClient_ReleaseClaim_Idempotente404(t *testing.T) {
	// 404/409 en release = candado ya inexistente/expirado → no-op (sin error).
	for _, code := range []int{http.StatusNotFound, http.StatusConflict} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
		if err := c.ReleaseClaim(context.Background(), "att-1"); err != nil {
			t.Fatalf("release %d debe ser no-op idempotente, hubo: %v", code, err)
		}
		srv.Close()
	}
}

func TestLearningClient_Permanent4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	_, err := c.GetPendingAnswers(context.Background(), "att-1")
	if err == nil {
		t.Fatal("esperaba error por 404")
	}
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("404 debe envolver ErrLearningPermanent, hubo: %v", err)
	}
}

func TestLearningClient_Transient5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewLearningClient(LearningClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"t"}})
	_, err := c.FinalizeAttempt(context.Background(), "att-1")
	if err == nil {
		t.Fatal("esperaba error por 503")
	}
	if errors.Is(err, ErrLearningPermanent) {
		t.Fatal("503 NO debe ser permanente (es transitorio)")
	}
}
