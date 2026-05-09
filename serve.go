package gomcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhangpanda/gomcp/transport"
)

// Stdio starts the server over stdin/stdout.
func (s *Server) Stdio() error {
	s.logger.Info("starting MCP server", "name", s.name, "version", s.version, "transport", "stdio")
	return transport.ServeStdio(s.rawHandler)
}

// HTTP starts the server over Streamable HTTP on the given address.
// For browser clients using a custom mux, wrap the MCP handler with transport.WrapCORS from package transport.
func (s *Server) HTTP(addr string) error {
	s.logger.Info("starting MCP server", "name", s.name, "version", s.version, "transport", "http", "addr", addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hs := transport.NewHTTPServer(s.rawHandler)
	hs.MaxRequestSize = s.maxRequestSize
	hs.ValidateSSE = s.sseValidate
	s.mu.Lock()
	s.notifyFn = append(s.notifyFn, hs.Notify)
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.Handle("/mcp", hs)
	return transport.ServeHTTPAddrWithHandler(ctx, addr, mux)
}

// Handler returns an http.Handler for embedding in existing HTTP servers.
// When browsers call POST /mcp cross-origin, wrap this handler with transport.WrapCORS from package transport.
func (s *Server) Handler() http.Handler {
	hs := transport.NewHTTPServer(s.rawHandler)
	hs.MaxRequestSize = s.maxRequestSize
	hs.ValidateSSE = s.sseValidate
	s.mu.Lock()
	s.notifyFn = append(s.notifyFn, hs.Notify)
	s.mu.Unlock()
	return hs
}

// rawHandler adapts the transport.MessageHandler signature.
func (s *Server) rawHandler(ctx context.Context, raw json.RawMessage) json.RawMessage {
	var msg jsonrpcMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		resp := newErrorResponse(nil, -32700, "parse error: "+err.Error())
		data, _ := json.Marshal(resp)
		return data
	}

	resp := s.handleRequestInternal(ctx, &msg)
	if resp == nil {
		return nil
	}

	data, err := json.Marshal(resp)
	if err != nil {
		// A handler returned something that cannot be serialised (e.g.
		// a Result containing a channel or a cyclic pointer). Surface
		// the problem as a standard JSON-RPC internal error rather than
		// silently writing an empty byte slice to stdio/HTTP.
		s.logger.Error("marshal JSON-RPC response failed", "method", msg.Method, "error", err)
		fallback := newErrorResponse(msg.ID, -32603, "internal error: failed to marshal response: "+err.Error())
		data, _ = json.Marshal(fallback)
	}
	return data
}
