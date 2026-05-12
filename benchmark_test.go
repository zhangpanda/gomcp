package gomcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// --- Benchmark: HandleRaw dispatch (tools/call hot path) ---

func BenchmarkHandleRaw_ToolCall(b *testing.B) {
	s := gomcp.New("bench", "1.0")
	s.Tool("noop", "", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "noop", "arguments": map[string]any{}},
	})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.HandleRaw(ctx, req)
	}
}

// --- Benchmark: Schema validation ---

type benchInput struct {
	Query string `json:"query" mcp:"required,desc=q"`
	Limit int    `json:"limit" mcp:"min=1,max=100"`
	Mode  string `json:"mode" mcp:"enum=fast|deep"`
}

func BenchmarkHandleRaw_SchemaValidation(b *testing.B) {
	s := gomcp.New("bench", "1.0")
	s.ToolFunc("strict", "", func(ctx *gomcp.Context, in benchInput) (string, error) {
		return "ok", nil
	})
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "strict", "arguments": map[string]any{
			"query": "hello", "limit": 10, "mode": "fast",
		}},
	})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.HandleRaw(ctx, req)
	}
}

// --- Benchmark: Middleware chain (5 middlewares stacked) ---

func BenchmarkHandleRaw_MiddlewareChain(b *testing.B) {
	s := gomcp.New("bench", "1.0")
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Use(gomcp.Logger())
	s.Use(gomcp.RateLimit(1_000_000)) // effectively unlimited
	s.Use(gomcp.OpenTelemetry())
	s.Tool("noop", "", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "noop", "arguments": map[string]any{}},
	})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.HandleRaw(ctx, req)
	}
}

// --- Benchmark: tools/list (no handler execution, pure routing) ---

func BenchmarkHandleRaw_ToolsList(b *testing.B) {
	s := gomcp.New("bench", "1.0")
	for i := 0; i < 50; i++ {
		name := "tool_" + benchItoa(i)
		s.Tool(name, "", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return ctx.Text("ok"), nil
		})
	}
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{},
	})
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = s.HandleRaw(ctx, req)
	}
}

func benchItoa(n int) string {
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if i == len(buf) {
		return "0"
	}
	return string(buf[i:])
}
