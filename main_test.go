package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandleRoot(t *testing.T) {
	s := &server{hostname: "test-host"}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	s.handleRoot(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp rootResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Message != "hello" {
		t.Errorf("expected message %q, got %q", "hello", resp.Message)
	}
	if resp.Hostname != "test-host" {
		t.Errorf("expected hostname %q, got %q", "test-host", resp.Hostname)
	}
	if resp.Version != version {
		t.Errorf("expected version %q, got %q", version, resp.Version)
	}
}

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handleHealthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "ok" {
		t.Errorf("expected body %q, got %q", "ok", body)
	}
}

func TestHandleReadyz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	handleReadyz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "ready" {
		t.Errorf("expected body %q, got %q", "ready", body)
	}
}

func TestRoutes(t *testing.T) {
	s := &server{hostname: "test-host"}
	handler := s.routes()

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/", http.StatusOK},
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusOK},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != tt.wantStatus {
			t.Errorf("GET %s: expected status %d, got %d", tt.path, tt.wantStatus, rr.Code)
		}
	}
}

func TestPort(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		os.Unsetenv("PORT")
		if p := port(); p != "8080" {
			t.Errorf("expected default port 8080, got %q", p)
		}
	})

	t.Run("from env", func(t *testing.T) {
		os.Setenv("PORT", "9090")
		defer os.Unsetenv("PORT")
		if p := port(); p != "9090" {
			t.Errorf("expected port 9090, got %q", p)
		}
	})
}

func TestNewServer(t *testing.T) {
	s, err := newServer()
	if err != nil {
		t.Fatalf("newServer() error: %v", err)
	}
	if s.hostname == "" {
		t.Error("expected non-empty hostname")
	}
}
