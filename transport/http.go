package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

type transportCtxKey = CtxKey

// CtxKey is the context key type used by transport to inject HTTP metadata.
type CtxKey string

// HTTPServer serves MCP over Streamable HTTP (POST for requests, GET for SSE notifications).
type HTTPServer struct {
	handler MessageHandler
	mu      sync.Mutex
	clients map[chan []byte]struct{} // SSE client channels
}

// NewHTTPServer creates a Streamable HTTP transport.
func NewHTTPServer(handler MessageHandler) *HTTPServer {
	return &HTTPServer{
		handler: handler,
		clients: make(map[chan []byte]struct{}),
	}
}

// Notify sends a JSON-RPC notification to all connected SSE clients.
func (s *HTTPServer) Notify(method string, params any) {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		msg["params"] = params
	}
	data, _ := json.Marshal(msg)
	event := []byte("data: " + string(data) + "\n\n")

	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- event:
		default: // drop if client is slow
		}
	}
}

func (s *HTTPServer) addClient(ch chan []byte) {
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
}

func (s *HTTPServer) removeClient(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	s.mu.Unlock()
}

// ServeHTTP implements http.Handler.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleSSE(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *HTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan []byte, 32)
	s.addClient(ch)
	defer s.removeClient(ch)

	for {
		select {
		case event := <-ch:
			w.Write(event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *HTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// inject HTTP headers into context for auth middleware
	ctx := r.Context()
	if auth := r.Header.Get("Authorization"); auth != "" {
		ctx = context.WithValue(ctx, transportCtxKey("auth_header"), auth)
	}
	headers := make(map[string]string)
	for k := range r.Header {
		headers[k] = r.Header.Get(k)
	}
	ctx = context.WithValue(ctx, transportCtxKey("http_headers"), headers)

	msgs, isBatch := ParseBatch(body)

	responses := make([]json.RawMessage, 0, len(msgs))
	for _, msg := range msgs {
		resp := s.handler(ctx, msg)
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if isBatch {
		batch, _ := json.Marshal(responses)
		w.Write(batch)
	} else {
		w.Write(responses[0])
	}
}

// ServeHTTPAddr starts an HTTP server on the given address.
func ServeHTTPAddr(ctx context.Context, addr string, handler MessageHandler) error {
	hs := NewHTTPServer(handler)
	mux := http.NewServeMux()
	mux.Handle("/mcp", hs)
	return ServeHTTPAddrWithHandler(ctx, addr, mux)
}

// ServeHTTPAddrWithHandler starts an HTTP server with a custom handler.
func ServeHTTPAddrWithHandler(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
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
