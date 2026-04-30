// Package inspector provides a development UI for GoMCP servers.
package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// Server interface matches the subset of gomcp.Server needed by the inspector.
type Server interface {
	Handler() http.Handler
	HandleRaw(ctx context.Context, raw json.RawMessage) json.RawMessage
}

// Dev starts a development server with Inspector UI on the given address.
func Dev(s Server, addr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.Handle("/mcp", s.Handler())

	// Inspector API — all data fetched via JSON-RPC
	mux.HandleFunc("/api/tools", func(w http.ResponseWriter, r *http.Request) {
		writeRPC(w, r, s, "tools/list", map[string]any{})
	})
	mux.HandleFunc("/api/resources", func(w http.ResponseWriter, r *http.Request) {
		res := callRPC(r.Context(), s, "resources/list", map[string]any{})
		tmpl := callRPC(r.Context(), s, "resources/templates/list", map[string]any{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"resources": res, "templates": tmpl})
	})
	mux.HandleFunc("/api/prompts", func(w http.ResponseWriter, r *http.Request) {
		writeRPC(w, r, s, "prompts/list", map[string]any{})
	})
	mux.HandleFunc("/api/call", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		paramsJSON, _ := json.Marshal(req.Params)
		rpcReq, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": req.Method, "params": json.RawMessage(paramsJSON),
		})
		resp := s.HandleRaw(r.Context(), rpcReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, inspectorHTML)
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func callRPC(ctx context.Context, s Server, method string, params any) any {
	paramsJSON, _ := json.Marshal(params)
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": json.RawMessage(paramsJSON),
	})
	resp := s.HandleRaw(ctx, req)
	var msg struct {
		Result any `json:"result"`
	}
	_ = json.Unmarshal(resp, &msg)
	return msg.Result
}

func writeRPC(w http.ResponseWriter, r *http.Request, s Server, method string, params any) {
	result := callRPC(r.Context(), s, method, params)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
