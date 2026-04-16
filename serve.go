package gomcp

import (
	"context"
	"encoding/json"
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
	return transport.ServeHTTPAddr(ctx, addr, s.rawHandler)
}

// Handler returns an http.Handler for embedding in existing HTTP servers.
func (s *Server) Handler() interface{ ServeHTTP(interface{}, interface{}) } {
	// Return the raw handler for advanced use; users should use HTTP() for simple cases.
	return nil
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
