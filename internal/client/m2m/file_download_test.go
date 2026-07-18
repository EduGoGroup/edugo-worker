package m2m

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadFile_OK(t *testing.T) {
	payload := []byte("%PDF-1.7 contenido del material")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método inesperado: %s", r.Method)
		}
		// La URL va firmada: NO debe mandarse Authorization.
		if r.Header.Get("Authorization") != "" {
			t.Errorf("no debe llevar Authorization, tenía: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	got, err := DownloadFile(context.Background(), srv.URL, 1<<20)
	if err != nil {
		t.Fatalf("DownloadFile falló: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("contenido inesperado: %q", got)
	}
}

func TestDownloadFile_ExceedsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Cuerpo mayor que el límite: el corte es por bytes reales leídos (LimitReader),
		// no por Content-Length.
		_, _ = w.Write(bytes.Repeat([]byte("x"), 100))
	}))
	defer srv.Close()

	_, err := DownloadFile(context.Background(), srv.URL, 10)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("esperaba ErrFileTooLarge, got: %v", err)
	}
	// Un archivo demasiado grande es permanente: reintentar no lo cura.
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatal("ErrFileTooLarge debe envolver ErrLearningPermanent")
	}
}

func TestDownloadFile_403Permanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<Error>expired</Error>"))
	}))
	defer srv.Close()

	_, err := DownloadFile(context.Background(), srv.URL, 1<<20)
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("403 (firma expirada) debe ser permanente, got: %v", err)
	}
}

func TestDownloadFile_500Transient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := DownloadFile(context.Background(), srv.URL, 1<<20)
	if err == nil {
		t.Fatal("esperaba error por 500")
	}
	if errors.Is(err, ErrLearningPermanent) {
		t.Fatal("500 NO debe ser permanente (es transitorio)")
	}
}
