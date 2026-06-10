package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) Token() (string, error) { return s.token, s.err }

func TestNotificationDispatchClient_Success(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody DispatchRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := NewNotificationDispatchClient(NotificationDispatchClientConfig{
		BaseURL:       srv.URL,
		TokenProvider: stubTokenProvider{token: "svc-jwt-abc"},
	})

	req := DispatchRequest{
		IdempotencyKey: "assessment.assigned:1",
		Recipients:     []DispatchRecipient{{UserID: "u1"}, {UserID: "u2"}},
		Notification:   DispatchNotification{Type: "assessment_assigned", Title: "x"},
		Channels:       &DispatchChannels{InApp: true, Push: true},
	}
	if err := c.Dispatch(context.Background(), req); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if gotAuth != "Bearer svc-jwt-abc" {
		t.Errorf("Authorization=%q", gotAuth)
	}
	if gotPath != dispatchEndpoint {
		t.Errorf("path=%q, quiero %q", gotPath, dispatchEndpoint)
	}
	if len(gotBody.Recipients) != 2 {
		t.Errorf("recipients=%d, quiero 2", len(gotBody.Recipients))
	}
	if gotBody.Notification.Type != "assessment_assigned" {
		t.Errorf("type=%q", gotBody.Notification.Type)
	}
}

func TestNotificationDispatchClient_ServerErrorIsPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer srv.Close()

	c := NewNotificationDispatchClient(NotificationDispatchClientConfig{
		BaseURL:       srv.URL,
		TokenProvider: stubTokenProvider{token: "t"},
	})

	err := c.Dispatch(context.Background(), DispatchRequest{Recipients: []DispatchRecipient{{UserID: "u"}}})
	if err == nil {
		t.Fatal("5xx debe propagar error (Rabbit reintenta)")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error debe incluir status 503: %v", err)
	}
}

func TestNotificationDispatchClient_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewNotificationDispatchClient(NotificationDispatchClientConfig{
		BaseURL:       srv.URL,
		TokenProvider: stubTokenProvider{token: "t"},
	})

	err := c.Dispatch(context.Background(), DispatchRequest{})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("401 debe devolver error: %v", err)
	}
}

func TestNotificationDispatchClient_TokenError(t *testing.T) {
	c := NewNotificationDispatchClient(NotificationDispatchClientConfig{
		BaseURL:       "http://unused",
		TokenProvider: stubTokenProvider{err: fmt.Errorf("no secret")},
	})

	err := c.Dispatch(context.Background(), DispatchRequest{})
	if err == nil || !strings.Contains(err.Error(), "obtaining service token") {
		t.Errorf("error de token debe propagarse: %v", err)
	}
}
