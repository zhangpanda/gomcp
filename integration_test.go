package gomcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/transport"
)

// === Middleware integration tests ===

func TestMiddleware_ExecutionOrder(t *testing.T) {
	var order []string
	mw := func(name string) gomcp.Middleware {
		return func(ctx *gomcp.Context, next func() error) error {
			order = append(order, name+":before")
			err := next()
			order = append(order, name+":after")
			return err
		}
	}

	s := gomcp.New("test", "1.0")
	s.Use(mw("global1"))
	s.Use(mw("global2"))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		order = append(order, "handler")
		return ctx.Text("ok"), nil
	})

	callRaw(t, s, "ping", nil)

	expected := "global1:before,global2:before,handler,global2:after,global1:after"
	got := strings.Join(order, ",")
	if got != expected {
		t.Errorf("execution order:\n  want: %s\n  got:  %s", expected, got)
	}
}

func TestMiddleware_Recovery_Panic(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.Recovery())
	s.Tool("boom", "boom", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		panic("something exploded")
	})

	text, _ := callRaw(t, s, "boom", nil)
	if !strings.Contains(text, "internal error") {
		t.Errorf("expected panic recovery message, got: %s", text)
	}
}

func TestMiddleware_Timeout_Expires(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.Timeout(50 * time.Millisecond))
	s.Tool("slow", "slow", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		select {
		case <-time.After(5 * time.Second):
			return ctx.Text("done"), nil
		case <-ctx.Context().Done():
			return nil, ctx.Context().Err()
		}
	})

	text, _ := callRaw(t, s, "slow", nil)
	if !strings.Contains(text, "timed out") {
		t.Errorf("expected timeout error, got: %s", text)
	}
}

func TestMiddleware_Timeout_Passes(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.Timeout(1 * time.Second))
	s.Tool("fast", "fast", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("quick"), nil
	})

	text, isErr := callRaw(t, s, "fast", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "quick" {
		t.Errorf("expected 'quick', got: %s", text)
	}
}

func TestMiddleware_RateLimit_Allows(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.RateLimit(600)) // 10/sec
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	text, isErr := callRaw(t, s, "ping", nil)
	if isErr {
		t.Fatalf("first call should pass: %s", text)
	}
}

func TestMiddleware_RateLimit_Blocks(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.RateLimit(1)) // 1/min — only 1 token
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	// first call consumes the token
	callRaw(t, s, "ping", nil)
	// second call should be rate limited
	text, _ := callRaw(t, s, "ping", nil)
	if !strings.Contains(text, "rate limit") {
		t.Errorf("expected rate limit error, got: %s", text)
	}
}

func TestMiddleware_MultiStack(t *testing.T) {
	// Recovery + Timeout + Logger all together
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.Recovery())
	s.Use(gomcp.Timeout(1 * time.Second))
	s.Use(gomcp.Logger())
	s.Use(gomcp.RequestID())
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		if _, ok := ctx.Get("request_id"); !ok {
			t.Error("expected request_id in context")
		}
		return ctx.Text("ok"), nil
	})

	text, isErr := callRaw(t, s, "ping", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
}

// === Enterprise gateway integration test ===

func TestEnterprise_MultiGroupAuthChain(t *testing.T) {
	tokenValidator := func(token string) (map[string]any, error) {
		switch token {
		case "admin-token":
			return map[string]any{"sub": "admin", "roles": []string{"admin", "user"}, "permissions": []string{"read", "write", "delete"}}, nil
		case "user-token":
			return map[string]any{"sub": "user1", "roles": []string{"user"}, "permissions": []string{"read"}}, nil
		}
		return nil, fmt.Errorf("invalid")
	}

	s := gomcp.New("gateway", "1.0")
	s.Use(gomcp.Recovery())
	s.Use(gomcp.Logger())
	s.Use(gomcp.BearerAuth(tokenValidator))

	// user group — any authenticated user
	user := s.Group("user")
	user.Tool("profile", "Get profile", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		claims, _ := ctx.Get("_auth_claims")
		return ctx.Text("profile:" + claims.(map[string]any)["sub"].(string)), nil
	})

	// admin group — requires admin role
	admin := s.Group("admin", gomcp.RequireRole("admin"))
	admin.Tool("delete", "Delete user", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("deleted:" + ctx.String("id")), nil
	})

	// nested group: admin.super — requires delete permission
	superAdmin := admin.Group("super", gomcp.RequirePermission("delete"))
	superAdmin.Tool("nuke", "Nuke everything", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("nuked"), nil
	})

	// Test 1: user can access user.profile
	ctx := context.WithValue(context.Background(), transport.CtxKey("auth_header"), "Bearer user-token")
	text, isErr := callWithCtx2(s, ctx, "user.profile", nil)
	if isErr {
		t.Fatalf("user should access profile: %s", text)
	}
	if text != "profile:user1" {
		t.Errorf("unexpected: %s", text)
	}

	// Test 2: user cannot access admin.delete (no admin role)
	text, _ = callWithCtx2(s, ctx, "admin.delete", map[string]any{"id": "42"})
	if !strings.Contains(text, "requires role admin") {
		t.Errorf("user should be denied admin: %s", text)
	}

	// Test 3: admin can access admin.delete
	ctx = context.WithValue(context.Background(), transport.CtxKey("auth_header"), "Bearer admin-token")
	text, isErr = callWithCtx2(s, ctx, "admin.delete", map[string]any{"id": "42"})
	if isErr {
		t.Fatalf("admin should access delete: %s", text)
	}
	if text != "deleted:42" {
		t.Errorf("unexpected: %s", text)
	}

	// Test 4: admin can access nested admin.super.nuke
	text, isErr = callWithCtx2(s, ctx, "admin.super.nuke", nil)
	if isErr {
		t.Fatalf("admin should access nuke: %s", text)
	}
	if text != "nuked" {
		t.Errorf("unexpected: %s", text)
	}

	// Test 5: no auth → rejected
	text, _ = callWithCtx2(s, context.Background(), "user.profile", nil)
	if !strings.Contains(text, "missing") {
		t.Errorf("no auth should be rejected: %s", text)
	}

	// Test 6: tools/list requires the same auth as tools/call when middleware is enabled
	ctx = context.WithValue(context.Background(), transport.CtxKey("auth_header"), "Bearer user-token")
	resp := callMethodCtx(t, s, ctx, "tools/list", map[string]any{})
	respStr := string(resp)
	for _, name := range []string{"user.profile", "admin.delete", "admin.super.nuke"} {
		if !strings.Contains(respStr, name) {
			t.Errorf("expected %s in tools list", name)
		}
	}
}

// === Async task failure test ===

func TestAsyncTool_Failure(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.AsyncTool("fail_task", "will fail", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return nil, fmt.Errorf("task exploded")
	})

	text, _ := callRaw(t, s, "fail_task", nil)
	var taskResp struct {
		TaskID string `json:"taskId"`
	}
	json.Unmarshal([]byte(text), &taskResp)

	time.Sleep(50 * time.Millisecond)
	resp := callMethod(t, s, "tasks/get", map[string]any{"taskId": taskResp.TaskID})
	respStr := string(resp)
	if !strings.Contains(respStr, "failed") {
		t.Errorf("expected failed status, got: %s", respStr)
	}
	if !strings.Contains(respStr, "task exploded") {
		t.Errorf("expected error message, got: %s", respStr)
	}
}

// helper — same as callWithCtx in middleware_auth_test but avoids redeclaration
func callWithCtx2(s *gomcp.Server, ctx context.Context, tool string, args map[string]any) (string, bool) {
	params, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	resp := s.HandleRaw(ctx, req)

	var msg struct {
		Result *struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	json.Unmarshal(resp, &msg)
	if msg.Error != nil {
		return msg.Error.Message, true
	}
	if msg.Result != nil && len(msg.Result.Content) > 0 {
		return msg.Result.Content[0].Text, msg.Result.IsError
	}
	return "", true
}

// === Inspector HTTP API tests ===

func newInspectorServer() *gomcp.Server {
	s := gomcp.New("inspector-test", "1.0")
	s.Tool("echo", "echo", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("echo:" + ctx.String("msg")), nil
	})
	s.Resource("test://data", "Test", func(ctx *gomcp.Context) (any, error) {
		return "hello", nil
	})
	s.Prompt("greet", "Greet", []gomcp.PromptArgument{gomcp.PromptArg("name", "Name", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{gomcp.UserMsg("Hi " + ctx.String("name"))}, nil
		})
	return s
}

func TestInspector_ToolsAPI(t *testing.T) {
	s := newInspectorServer()
	h := s.Handler()

	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "echo") {
		t.Errorf("expected echo tool in response: %s", w.Body.String())
	}
}

// === HTTP transport end-to-end tests ===

func TestHTTP_E2E_ToolCall(t *testing.T) {
	s := gomcp.New("e2e", "1.0")
	s.Tool("greet", "greet", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("hello " + ctx.String("name")), nil
	})

	h := s.Handler()

	// initialize
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("init: expected 200, got %d", w.Code)
	}

	// call tool
	req = httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"greet","arguments":{"name":"World"}}}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("call: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello World") {
		t.Errorf("expected 'hello World', got: %s", w.Body.String())
	}
}

func TestHTTP_E2E_WithAuth(t *testing.T) {
	s := gomcp.New("e2e-auth", "1.0")
	s.Use(gomcp.BearerAuth(func(token string) (map[string]any, error) {
		if token == "good" {
			return map[string]any{"sub": "user1"}, nil
		}
		return nil, fmt.Errorf("bad token")
	}))
	s.Tool("secret", "secret", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("secret data"), nil
	})

	h := s.Handler()

	// without auth → error
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"secret","arguments":{}}}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "missing") {
		t.Errorf("expected auth error without token: %s", w.Body.String())
	}

	// with auth → success
	req = httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"secret","arguments":{}}}`))
	req.Header.Set("Authorization", "Bearer good")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "secret data") {
		t.Errorf("expected secret data with valid token: %s", w.Body.String())
	}
}

func TestHTTP_E2E_Batch(t *testing.T) {
	s := gomcp.New("e2e-batch", "1.0")
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("pong"), nil
	})

	h := s.Handler()
	body := `[{"jsonrpc":"2.0","id":1,"method":"ping","params":{}},{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}]`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// should be a JSON array
	var batch []json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil {
		t.Fatalf("expected batch response: %v", err)
	}
	if len(batch) != 2 {
		t.Errorf("expected 2 responses, got %d", len(batch))
	}
}
