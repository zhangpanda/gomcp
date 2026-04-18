// Regression tests for every bug found during development.
// Each test documents the original bug and ensures it never returns.
package gomcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/transport"
)

// BUG: AsyncTool handler panic crashed entire process (no recover in goroutine)
func TestRegression_AsyncPanicNoCrash(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.AsyncTool("boom", "boom", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		panic("async panic")
	})
	text, _ := callRaw(t, s, "boom", nil)
	var tr struct{ TaskID string `json:"taskId"` }
	json.Unmarshal([]byte(text), &tr)
	time.Sleep(100 * time.Millisecond)

	resp := callMethod(t, s, "tasks/get", map[string]any{"taskId": tr.TaskID})
	if !strings.Contains(string(resp), "failed") {
		t.Errorf("async panic should set status=failed, got: %s", string(resp))
	}
	if !strings.Contains(string(resp), "panic") {
		t.Errorf("async panic should include panic message, got: %s", string(resp))
	}
}

// BUG: Handler panic without Recovery middleware crashed server
func TestRegression_PanicWithoutRecovery(t *testing.T) {
	s := gomcp.New("t", "1.0")
	// deliberately NO Recovery middleware
	s.Tool("boom", "boom", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		panic("no recovery")
	})
	text, isErr := callRaw(t, s, "boom", nil)
	if !isErr {
		t.Error("panic should return error")
	}
	if !strings.Contains(text, "internal error") {
		t.Errorf("expected internal error, got: %s", text)
	}
}

// BUG: Auth/middleware errors returned JSON-RPC error instead of MCP tool error
func TestRegression_MiddlewareErrorIsMCPError(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(func(ctx *gomcp.Context, next func() error) error {
		return fmt.Errorf("blocked")
	})
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	params, _ := json.Marshal(map[string]any{"name": "ping"})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	resp := s.HandleRaw(context.Background(), req)

	var msg struct {
		Result *struct {
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct{} `json:"error"`
	}
	json.Unmarshal(resp, &msg)

	if msg.Error != nil {
		t.Error("middleware error should NOT be JSON-RPC error")
	}
	if msg.Result == nil || !msg.Result.IsError {
		t.Error("middleware error should be MCP tool error with isError=true")
	}
}

// BUG: Version resolution used lexicographic sort ("9.0" > "20.0")
func TestRegression_SemverResolution(t *testing.T) {
	s := gomcp.New("t", "1.0")
	for i := 1; i <= 20; i++ {
		v := fmt.Sprintf("%d.0", i)
		val := v
		s.Tool("calc", "v"+v, func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return ctx.Text("v" + val), nil
		}, gomcp.Version(v))
	}
	text, _ := callRaw(t, s, "calc", nil)
	if text != "v20.0" {
		t.Errorf("expected v20.0 (semver), got: %s", text)
	}
}

// BUG: RateLimit used background goroutine that leaked
func TestRegression_RateLimitNoGoroutineLeak(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.RateLimit(60))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})
	text, isErr := callRaw(t, s, "ping", nil)
	if isErr || text != "ok" {
		t.Errorf("unexpected: %s, isErr=%v", text, isErr)
	}
	// no way to directly test goroutine leak, but RateLimit now uses lazy refill
	// so this test just ensures it works without hanging
}

// BUG: Timeout middleware had race condition on handlerErr variable
func TestRegression_TimeoutNoRace(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.Timeout(50 * time.Millisecond))
	s.Tool("slow", "slow", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		select {
		case <-time.After(5 * time.Second):
			return ctx.Text("done"), nil
		case <-ctx.Context().Done():
			return nil, ctx.Context().Err()
		}
	})
	text, isErr := callRaw(t, s, "slow", nil)
	if !isErr || !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout, got: %s isErr=%v", text, isErr)
	}
}

// BUG: AsyncTool mutated shared Context.ctx field (race condition)
func TestRegression_AsyncToolNoSharedContext(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.AsyncTool("work", "work", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		time.Sleep(10 * time.Millisecond)
		return ctx.Text("done"), nil
	})
	// call multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callRaw(t, s, "work", nil)
		}()
	}
	wg.Wait()
	// if there was a race, -race would catch it
}

// BUG: Context.store had no mutex (race with Timeout middleware)
func TestRegression_ContextStoreConcurrent(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.Timeout(5 * time.Second))
	s.Use(func(ctx *gomcp.Context, next func() error) error {
		ctx.Set("mw_key", "mw_val")
		return next()
	})
	s.Tool("test", "test", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		ctx.Set("handler_key", "handler_val")
		v, _ := ctx.Get("mw_key")
		return ctx.Text(fmt.Sprintf("%v", v)), nil
	})
	text, isErr := callRaw(t, s, "test", nil)
	if isErr || text != "mw_val" {
		t.Errorf("expected mw_val, got: %s", text)
	}
}

// BUG: task struct fields had data race (no mutex)
func TestRegression_TaskFieldsRace(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.AsyncTool("fast", "fast", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("done"), nil
	})
	// submit and immediately poll — races on Status/Result fields
	text, _ := callRaw(t, s, "fast", nil)
	var tr struct{ TaskID string `json:"taskId"` }
	json.Unmarshal([]byte(text), &tr)
	// poll immediately (task may still be running)
	callMethod(t, s, "tasks/get", map[string]any{"taskId": tr.TaskID})
	// poll again after completion
	time.Sleep(50 * time.Millisecond)
	resp := callMethod(t, s, "tasks/get", map[string]any{"taskId": tr.TaskID})
	if !strings.Contains(string(resp), "completed") {
		t.Errorf("expected completed: %s", string(resp))
	}
}

// BUG: Group middleware isolation — one group's error should not affect another
func TestRegression_GroupMiddlewareIsolation(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Group("blocked", func(ctx *gomcp.Context, next func() error) error {
		return fmt.Errorf("nope")
	}).Tool("a", "a", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("a"), nil
	})
	s.Group("open").Tool("b", "b", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("b"), nil
	})

	_, isErr := callRaw(t, s, "blocked.a", nil)
	if !isErr {
		t.Error("blocked.a should fail")
	}
	text, isErr := callRaw(t, s, "open.b", nil)
	if isErr || text != "b" {
		t.Errorf("open.b should work, got: %s isErr=%v", text, isErr)
	}
}

// BUG: Session persistence across calls with same Mcp-Session-Id
func TestRegression_SessionPersistence(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Tool("count", "count", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		sess := ctx.Session()
		n, _ := sess.Get("n")
		count, _ := n.(int)
		count++
		sess.Set("n", count)
		return ctx.Text(fmt.Sprintf("%d", count)), nil
	})

	sessCtx := context.WithValue(context.Background(), transport.CtxKey("http_headers"),
		map[string]string{"Mcp-Session-Id": "test-session"})

	text1, _ := callWithCtx2(s, sessCtx, "count", nil)
	text2, _ := callWithCtx2(s, sessCtx, "count", nil)

	if text1 != "1" || text2 != "2" {
		t.Errorf("session should persist: got %s, %s", text1, text2)
	}
}

// BUG: Bearer auth with edge case tokens
func TestRegression_BearerAuthEdgeCases(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.BearerAuth(func(token string) (map[string]any, error) {
		if token == "good" {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("bad")
	}))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	cases := []struct {
		name   string
		header string
		reject bool
	}{
		{"empty Bearer", "Bearer ", true},
		{"no space", "Bearer", true},
		{"wrong scheme", "Token good", true},
		{"valid", "Bearer good", false},
		{"no header", "", true},
	}
	for _, tc := range cases {
		ctx := context.Background()
		if tc.header != "" {
			ctx = context.WithValue(ctx, transport.CtxKey("auth_header"), tc.header)
		}
		_, isErr := callWithCtx2(s, ctx, "ping", nil)
		if isErr != tc.reject {
			t.Errorf("%s: expected reject=%v, got isErr=%v", tc.name, tc.reject, isErr)
		}
	}
}
