package gomcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/istarshine/gomcp/transport"
)

// Stdio starts the server over stdin/stdout.
func (s *Server) Stdio() error {
	s.logger.Info("starting MCP server", "name", s.name, "version", s.version, "transport", "stdio")
	return transport.ServeStdio(s.rawHandler)
}

// HTTP starts the server over Streamable HTTP on the given address.
func (s *Server) HTTP(addr string) error {
	s.logger.Info("starting MCP server", "name", s.name, "version", s.version, "transport", "http", "addr", addr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hs := transport.NewHTTPServer(s.rawHandler)
	s.notifyFn = hs.Notify

	mux := http.NewServeMux()
	mux.Handle("/mcp", hs)
	return transport.ServeHTTPAddrWithHandler(ctx, addr, mux)
}

// Handler returns an http.Handler for embedding in existing HTTP servers.
func (s *Server) Handler() http.Handler {
	hs := transport.NewHTTPServer(s.rawHandler)
	s.notifyFn = hs.Notify
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

	data, _ := json.Marshal(resp)
	return data
}
