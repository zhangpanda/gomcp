package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhangpanda/gomcp/transport"
)

func TestWrapCORS_DisabledWhenEmpty(t *testing.T) {
	called := false
	h := transport.WrapCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}), nil)

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !called {
		t.Fatal("handler not invoked")
	}
	if w.Code != http.StatusTeapot {
		t.Fatalf("code=%d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected CORS header: %v", w.Header())
	}
}

func TestWrapCORS_OPTIONSAllowedOrigin(t *testing.T) {
	h := transport.WrapCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not run for OPTIONS")
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://app.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("code=%d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("Allow-Origin=%q", got)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("missing Allow-Methods")
	}
}

func TestWrapCORS_POSTAllowedOrigin(t *testing.T) {
	h := transport.WrapCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []string{"https://app.example"})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Origin", "https://app.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("Allow-Origin=%q", got)
	}
}
