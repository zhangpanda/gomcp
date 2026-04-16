package gomcp_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// --- Edge cases: malformed requests ---

func TestEdge_InvalidJSON(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`not json`))
	if !strings.Contains(string(resp), "parse error") {
		t.Errorf("expected parse error, got: %s", string(resp))
	}
}

func TestEdge_UnknownMethod(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"foo/bar","params":{}}`))
	if !strings.Contains(string(resp), "method not found") {
		t.Errorf("expected method not found, got: %s", string(resp))
	}
}

func TestEdge_ToolNotFound(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent"}}`))
	if !strings.Contains(string(resp), "tool not found") {
		t.Errorf("expected tool not found, got: %s", string(resp))
	}
}

func TestEdge_ToolCallBadParams(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"bad"}`))
	if !strings.Contains(string(resp), "invalid params") {
		t.Errorf("expected invalid params, got: %s", string(resp))
	}
}

func TestEdge_ResourceNotFound(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Resource("test://x", "X", func(ctx *gomcp.Context) (any, error) { return "ok", nil })
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"test://missing"}}`))
	if !strings.Contains(string(resp), "not found") {
		t.Errorf("expected not found, got: %s", string(resp))
	}
}

func TestEdge_PromptNotFound(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"missing"}}`))
	if !strings.Contains(string(resp), "not found") {
		t.Errorf("expected not found, got: %s", string(resp))
	}
}

func TestEdge_Notification_NoResponse(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if resp != nil {
		t.Errorf("notification should return nil, got: %s", string(resp))
	}
}

func TestEdge_Ping(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`))
	if resp == nil {
		t.Fatal("ping should return response")
	}
}

// --- Edge cases: nil/empty arguments ---

func TestEdge_ToolCallNilArgs(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("pong"), nil
	})
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ping"}}`))
	if !strings.Contains(string(resp), "pong") {
		t.Errorf("expected pong, got: %s", string(resp))
	}
}

func TestEdge_ContextGetMissing(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("test", "test", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		v, ok := ctx.Get("nonexistent")
		if ok || v != nil {
			return ctx.Error("should be nil"), nil
		}
		return ctx.Text(ctx.String("missing_key")), nil // should return ""
	})
	text, isErr := callRaw(t, s, "test", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "" {
		t.Errorf("expected empty string, got: %q", text)
	}
}

func TestEdge_ContextIntFloat_NonNumeric(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("test", "test", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		i := ctx.Int("name")    // "hello" → 0
		f := ctx.Float("name")  // "hello" → 0
		b := ctx.Bool("name")   // "hello" → false
		if i != 0 || f != 0 || b != false {
			return ctx.Error("wrong defaults"), nil
		}
		return ctx.Text("ok"), nil
	})
	text, isErr := callRaw(t, s, "test", map[string]any{"name": "hello"})
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
}

// --- Edge cases: handler returns nil result ---

func TestEdge_HandlerReturnsNil(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("nil_tool", "nil", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return nil, nil
	})
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nil_tool"}}`))
	// should not panic, should return something
	if resp == nil {
		t.Fatal("expected response even for nil result")
	}
}

// --- Edge cases: version resolution ---

func TestEdge_VersionCallNonexistent(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("calc", "v1", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("v1"), nil
	}, gomcp.Version("1.0"))

	// call nonexistent version
	text, _ := callRaw(t, s, "calc@9.9", nil)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found for bad version, got: %s", text)
	}
}

func TestEdge_VersionUnversioned(t *testing.T) {
	// register without version, call by name
	s := gomcp.New("test", "1.0")
	s.Tool("simple", "simple", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})
	text, isErr := callRaw(t, s, "simple", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "ok" {
		t.Errorf("expected ok, got: %s", text)
	}
}

// --- Edge cases: tasks ---

func TestEdge_TasksGetWithoutAsyncTool(t *testing.T) {
	s := gomcp.New("test", "1.0")
	// no async tools registered, taskMgr is nil
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"taskId":"x"}}`))
	if !strings.Contains(string(resp), "not enabled") {
		t.Errorf("expected tasks not enabled, got: %s", string(resp))
	}
}

func TestEdge_TasksCancelWithoutAsyncTool(t *testing.T) {
	s := gomcp.New("test", "1.0")
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tasks/cancel","params":{"taskId":"x"}}`))
	if !strings.Contains(string(resp), "not enabled") {
		t.Errorf("expected tasks not enabled, got: %s", string(resp))
	}
}

// --- Edge cases: resource handler error ---

func TestEdge_ResourceHandlerError(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Resource("err://x", "X", func(ctx *gomcp.Context) (any, error) {
		return nil, fmt.Errorf("resource error")
	})
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"err://x"}}`))
	if !strings.Contains(string(resp), "error") {
		t.Errorf("expected error, got: %s", string(resp))
	}
}

// --- Edge cases: prompt handler error ---

func TestEdge_PromptHandlerError(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Prompt("bad", "bad", nil, func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
		return nil, fmt.Errorf("prompt error")
	})
	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"bad"}}`))
	if !strings.Contains(string(resp), "error") {
		t.Errorf("expected error, got: %s", string(resp))
	}
}

// --- Edge cases: empty tools/list, resources/list, prompts/list ---

func TestEdge_EmptyLists(t *testing.T) {
	s := gomcp.New("test", "1.0")

	resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	if !strings.Contains(string(resp), `"tools":[]`) {
		t.Errorf("expected empty tools, got: %s", string(resp))
	}

	resp = s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}`))
	if !strings.Contains(string(resp), `"resources":[]`) {
		t.Errorf("expected empty resources, got: %s", string(resp))
	}

	resp = s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"prompts/list","params":{}}`))
	if !strings.Contains(string(resp), `"prompts":[]`) {
		t.Errorf("expected empty prompts, got: %s", string(resp))
	}
}

// --- Edge cases: ToolFunc with wrong signature ---

func TestEdge_ToolFuncBadSignature(t *testing.T) {
	s := gomcp.New("test", "1.0")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for bad ToolFunc signature")
		}
	}()
	s.ToolFunc("bad", "bad", func() {}) // wrong signature → should panic
}
