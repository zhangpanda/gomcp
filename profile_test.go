package gomcp_test

import (
	"context"
	"encoding/json"
	"os"
	"runtime/pprof"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// TestProfile_HeapDump runs 100k tool calls then dumps a heap profile
// to /tmp/gomcp_heap.prof for offline analysis with `go tool pprof`.
// Not a real test — just a profiling harness. Skipped in -short.
func TestProfile_HeapDump(t *testing.T) {
	if testing.Short() {
		t.Skip("profiling harness")
	}

	s := gomcp.New("prof", "1.0.0")
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Tool("echo", "", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "echo", "arguments": map[string]any{}},
	})
	ctx := context.Background()

	for i := 0; i < 100_000; i++ {
		_ = s.HandleRaw(ctx, req)
	}

	f, err := os.Create("/tmp/gomcp_heap.prof")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	t.Logf("heap profile written to /tmp/gomcp_heap.prof")
	t.Logf("analyze with: go tool pprof -top /tmp/gomcp_heap.prof")
}
