package gomcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/istarshine/gomcp"
)

// --- Version tests ---

func TestVersion_RegisterAndCall(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("search", "v1", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("v1"), nil
	}, gomcp.Version("1.0"))
	s.Tool("search", "v2", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("v2"), nil
	}, gomcp.Version("2.0"))

	// call exact version
	text, _ := callRaw(t, s, "search@1.0", nil)
	if text != "v1" {
		t.Errorf("expected v1, got %s", text)
	}
	text, _ = callRaw(t, s, "search@2.0", nil)
	if text != "v2" {
		t.Errorf("expected v2, got %s", text)
	}

	// call without version → latest (2.0 > 1.0 lexicographically)
	text, _ = callRaw(t, s, "search", nil)
	if text != "v2" {
		t.Errorf("expected latest (v2), got %s", text)
	}
}

func TestVersion_ListShowsAll(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("calc", "v1", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	}, gomcp.Version("1.0"))
	s.Tool("calc", "v2", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	}, gomcp.Version("2.0"))

	resp := callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "calc@1.0") || !strings.Contains(string(resp), "calc@2.0") {
		t.Errorf("expected both versions in list, got: %s", string(resp))
	}
}

func TestDeprecated(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("old", "old tool", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	}, gomcp.Deprecated("use new_tool instead"))

	resp := callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "deprecated") {
		t.Errorf("expected deprecated annotation, got: %s", string(resp))
	}
}

// --- Async task tests ---

func TestAsyncTool_ReturnsTaskID(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.AsyncTool("slow", "slow task", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		time.Sleep(50 * time.Millisecond)
		return ctx.Text("done"), nil
	})

	text, isErr := callRaw(t, s, "slow", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "taskId") {
		t.Fatalf("expected taskId, got: %s", text)
	}

	// extract task ID
	var taskResp struct{ TaskID string `json:"taskId"` }
	json.Unmarshal([]byte(text), &taskResp)

	// poll for completion
	time.Sleep(100 * time.Millisecond)
	resp := callMethod(t, s, "tasks/get", map[string]any{"taskId": taskResp.TaskID})
	if !strings.Contains(string(resp), "completed") {
		t.Errorf("expected completed, got: %s", string(resp))
	}
}

func TestAsyncTool_Cancel(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.AsyncTool("long", "long task", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		select {
		case <-time.After(5 * time.Second):
			return ctx.Text("done"), nil
		case <-ctx.Context().Done():
			return nil, ctx.Context().Err()
		}
	})

	text, _ := callRaw(t, s, "long", nil)
	var taskResp struct{ TaskID string `json:"taskId"` }
	json.Unmarshal([]byte(text), &taskResp)

	// cancel
	resp := callMethod(t, s, "tasks/cancel", map[string]any{"taskId": taskResp.TaskID})
	if !strings.Contains(string(resp), "cancelled") {
		t.Errorf("expected cancelled, got: %s", string(resp))
	}

	// verify status
	time.Sleep(20 * time.Millisecond)
	resp = callMethod(t, s, "tasks/get", map[string]any{"taskId": taskResp.TaskID})
	if !strings.Contains(string(resp), "cancelled") {
		t.Errorf("expected cancelled status, got: %s", string(resp))
	}
}

func TestAsyncTool_NotFound(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.AsyncTool("x", "x", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})
	resp := callMethod(t, s, "tasks/get", map[string]any{"taskId": "nonexistent"})
	if !strings.Contains(string(resp), "not found") {
		t.Errorf("expected not found, got: %s", string(resp))
	}
}

// --- Provider (LoadDir) tests ---

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: greet
description: Say hello
params:
  - name: name
    type: string
    required: true
    description: Who to greet
handler: http://localhost:9999/greet
method: GET
`
	os.WriteFile(filepath.Join(dir, "greet.tool.yaml"), []byte(yaml), 0o644)

	s := gomcp.New("test", "1.0")
	err := s.LoadDir(dir, gomcp.DirOptions{})
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}

	resp := callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "greet") {
		t.Errorf("expected greet tool, got: %s", string(resp))
	}
}

func TestLoadDir_WithVersion(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: calc
description: Calculator
version: "2.0"
params:
  - name: expr
    type: string
    required: true
handler: http://localhost:9999/calc
method: POST
`
	os.WriteFile(filepath.Join(dir, "calc.tool.yaml"), []byte(yaml), 0o644)

	s := gomcp.New("test", "1.0")
	s.LoadDir(dir, gomcp.DirOptions{})

	resp := callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "calc@2.0") {
		t.Errorf("expected calc@2.0, got: %s", string(resp))
	}
}

func TestLoadDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := gomcp.New("test", "1.0")
	err := s.LoadDir(dir, gomcp.DirOptions{})
	if err != nil {
		t.Fatalf("LoadDir on empty dir should not error: %v", err)
	}
}

// --- helpers ---

func callRaw(t *testing.T, s *gomcp.Server, tool string, args map[string]any) (string, bool) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	resp := s.HandleRaw(context.Background(), req)

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

func callMethod(t *testing.T, s *gomcp.Server, method string, params map[string]any) json.RawMessage {
	t.Helper()
	paramsJSON, _ := json.Marshal(params)
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": json.RawMessage(paramsJSON)})
	return s.HandleRaw(context.Background(), req)
}
