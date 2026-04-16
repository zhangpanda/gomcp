package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

// HTTPServer serves MCP over Streamable HTTP (POST for requests, GET for SSE notifications).
type HTTPServer struct {
	handler MessageHandler
	mu      sync.Mutex
}

// NewHTTPServer creates a Streamable HTTP transport.
func NewHTTPServer(handler MessageHandler) *HTTPServer {
	return &HTTPServer{handler: handler}
}

// ServeHTTP implements http.Handler.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		// SSE endpoint for server-initiated notifications (placeholder)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// hold connection open until client disconnects
		<-r.Context().Done()
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *HTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp := s.handler(r.Context(), body)
	if resp == nil {
		// notification — no response body
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// ServeHTTPAddr starts an HTTP server on the given address.
func ServeHTTPAddr(ctx context.Context, addr string, handler MessageHandler) error {
	hs := NewHTTPServer(handler)
	mux := http.NewServeMux()
	mux.Handle("/mcp", hs)

	srv := &http.Server{Addr: addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return srv.Close()
	case err := <-errCh:
		return err
	}
}

// ServeHTTPHandler returns an http.Handler for embedding in existing routers.
func ServeHTTPHandler(handler MessageHandler) http.Handler {
	return NewHTTPServer(handler)
}

// Helpers for batch JSON-RPC (spec allows array of requests)
func ParseBatch(raw json.RawMessage) ([]json.RawMessage, bool) {
	raw = trimSpace(raw)
	if len(raw) > 0 && raw[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err == nil {
			return batch, true
		}
	}
	return []json.RawMessage{raw}, false
}

func trimSpace(b json.RawMessage) json.RawMessage {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r') {
		b = b[1:]
	}
	return b
}
