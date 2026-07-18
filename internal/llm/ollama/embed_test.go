package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbed_OK_Batch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var req embedRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		if req.Model != "test-embed" {
			t.Errorf("modelo inesperado: %s", req.Model)
		}
		if len(req.Input) != 2 {
			t.Errorf("se esperaban 2 inputs, hubo %d", len(req.Input))
		}
		_ = json.NewEncoder(w).Encode(embedResponse{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
		})
	}))
	defer srv.Close()

	p := NewEmbedder(EmbedConfig{BaseURL: srv.URL, Model: "test-embed"})
	vecs, err := p.Embed(context.Background(), []string{"hola", "mundo"})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("se esperaban 2 vectores, hubo %d", len(vecs))
	}
	if len(vecs[0]) != 3 || vecs[1][2] != 0.6 {
		t.Fatalf("vectores inesperados: %+v", vecs)
	}
}

func TestEmbed_EmptyInput_NoCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer srv.Close()

	p := NewEmbedder(EmbedConfig{BaseURL: srv.URL, Model: "m"})
	vecs, err := p.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(vecs) != 0 {
		t.Fatalf("se esperaba slice vacío, hubo %d", len(vecs))
	}
	if called {
		t.Fatalf("un lote vacío no debe llamar al backend")
	}
}

func TestEmbed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	p := NewEmbedder(EmbedConfig{BaseURL: srv.URL, Model: "m"})
	if _, err := p.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("se esperaba error por status 500")
	}
}

func TestEmbed_LengthMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Se piden 2 textos pero el backend devuelve un solo vector.
		_ = json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1, 0.2}}})
	}))
	defer srv.Close()

	p := NewEmbedder(EmbedConfig{BaseURL: srv.URL, Model: "m"})
	if _, err := p.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("se esperaba error por mismatch de longitudes")
	}
}

func TestEmbed_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1}}})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewEmbedder(EmbedConfig{BaseURL: srv.URL, Model: "m"})
	if _, err := p.Embed(ctx, []string{"x"}); err == nil {
		t.Fatal("se esperaba error por contexto cancelado")
	}
}
