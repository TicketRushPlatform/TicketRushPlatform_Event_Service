package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPServerBasics(t *testing.T) {
	s := NewHTTPServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	})

	if s.Router() == nil {
		t.Fatalf("router must not be nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz status=%d want=%d", w.Code, http.StatusOK)
	}
}

func TestHTTPServerShutdownAndCloseWithoutStart(t *testing.T) {
	s := NewHTTPServer(ServerConfig{
		Host: "127.0.0.1",
		Port: 0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("healthz status=%d want=%d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHTTPServerStartError(t *testing.T) {
	s := NewHTTPServer(ServerConfig{
		Host: "bad host",
		Port: 8080,
	})
	if err := s.Start(); err == nil {
		t.Fatalf("expected start error")
	}
}
