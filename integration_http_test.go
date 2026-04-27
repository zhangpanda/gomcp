package gomcp_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// Integration tests that start a real HTTP server and test the full
// request/response cycle including sessions, metrics, batch, and SSE.
// Skip with: go test -short

func TestIntegration_HTTP_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HTTP integration test in short mode")
	}

	s := gomcp.New("http-int", "1.0.0", gomcp.WithMaxRequestSize(1<<20))
	s.Use(gomcp.Recovery())
	metricsMw, metrics := gomcp.PrometheusMetrics()
	s.Use(metricsMw)

	s.ToolFunc("echo", "Echo", func(ctx *gomcp.Context, in struct {
		Msg string `json:"msg" mcp:"required"`
	}) (string, error) {
		if sess := ctx.Session(); sess != nil {
			n, _ := sess.Get("n")
			count, _ := n.(int)
			count++
			sess.Set("n", count)
			return fmt.Sprintf("%s#%d", in.Msg, count), nil
		}
		return in.Msg, nil
	})

	s.Tool("boom", "Panic", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		panic("test panic")
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", s.Handler())
	mux.Handle("/metrics", metrics.Handler())

	srv := &http.Server{}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind local port (HTTP integration needs listen): %v", err)
	}
	srv.Handler = mux
	go srv.Serve(ln)
	defer srv.Close()

	base := "http://" + ln.Addr().String()

	post := func(body string, headers ...string) []byte {
		t.Helper()
		req, _ := http.NewRequest("POST", base+"/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST error: %v", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return data
	}

	rpc := func(method string, params any, headers ...string) map[string]any {
		t.Helper()
		p, _ := json.Marshal(params)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"%s","params":%s}`, method, p)
		data := post(body, headers...)
		var resp map[string]any
		json.Unmarshal(data, &resp)
		return resp
	}

	getText := func(resp map[string]any) string {
		result, _ := resp["result"].(map[string]any)
		if result == nil {
			return ""
		}
		content, _ := result["content"].([]any)
		if len(content) == 0 {
			return ""
		}
		return content[0].(map[string]any)["text"].(string)
	}

	// 1. Initialize
	resp := rpc("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})
	if resp["result"] == nil {
		t.Fatal("initialize failed")
	}

	// 2. Tool call
	text := getText(rpc("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"msg": "hi"}}))
	if !strings.Contains(text, "hi") {
		t.Errorf("echo: got %s", text)
	}

	// 3. Session persistence
	t1 := getText(rpc("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"msg": "x"}}, "Mcp-Session-Id", "s1"))
	t2 := getText(rpc("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"msg": "x"}}, "Mcp-Session-Id", "s1"))
	if !strings.Contains(t1, "#1") || !strings.Contains(t2, "#2") {
		t.Errorf("session: got %s, %s", t1, t2)
	}

	// 4. Different session
	t3 := getText(rpc("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"msg": "x"}}, "Mcp-Session-Id", "s2"))
	if !strings.Contains(t3, "#1") {
		t.Errorf("different session should start at 1: %s", t3)
	}

	// 5. Panic recovery
	resp = rpc("tools/call", map[string]any{"name": "boom"})
	result, _ := resp["result"].(map[string]any)
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Error("panic should return isError")
	}

	// 6. Batch
	batchData := post(`[{"jsonrpc":"2.0","id":1,"method":"ping","params":{}},{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}]`)
	var batch []map[string]any
	json.Unmarshal(batchData, &batch)
	if len(batch) != 2 {
		t.Errorf("batch: expected 2, got %d", len(batch))
	}

	// 7. SSE endpoint
	sseResp, err := http.Get(base + "/mcp")
	if err != nil {
		t.Fatalf("SSE GET: %v", err)
	}
	if sseResp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("GET should be SSE: %s", sseResp.Header.Get("Content-Type"))
	}
	sseResp.Body.Close()

	// 8. Metrics
	metricsResp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("metrics GET: %v", err)
	}
	metricsBody, err := io.ReadAll(metricsResp.Body)
	metricsResp.Body.Close()
	if err != nil {
		t.Fatalf("metrics body: %v", err)
	}
	if !strings.Contains(string(metricsBody), "gomcp_calls_total") {
		t.Error("metrics missing gomcp_calls_total")
	}

	// 9. Session count
	if s.Sessions().Count() < 2 {
		t.Errorf("expected >= 2 sessions, got %d", s.Sessions().Count())
	}
}
