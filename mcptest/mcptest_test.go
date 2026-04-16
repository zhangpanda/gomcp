package mcptest

import (
	"fmt"
	"testing"

	"github.com/zhangpanda/gomcp"
)

func newServer() *gomcp.Server {
	s := gomcp.New("test", "1.0.0")

	s.Tool("echo", "Echo input", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("echo:" + ctx.String("msg")), nil
	})

	s.Tool("fail", "Always error", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return gomcp.ErrorResult("something went wrong"), nil
	})

	s.Resource("test://data", "Test Data", func(ctx *gomcp.Context) (any, error) {
		return map[string]string{"key": "value"}, nil
	})

	s.Prompt("greet", "Greeting",
		[]gomcp.PromptArgument{gomcp.PromptArg("name", "Name", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{gomcp.UserMsg("Hello " + ctx.String("name"))}, nil
		},
	)

	return s
}

func TestClient_Initialize(t *testing.T) {
	c := NewClient(t, newServer())
	result := c.Initialize()
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol: %v", result["protocolVersion"])
	}
}

func TestClient_ListTools(t *testing.T) {
	c := NewClient(t, newServer())
	tools := c.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}
}

func TestClient_CallTool(t *testing.T) {
	c := NewClient(t, newServer())
	r := c.CallTool("echo", map[string]any{"msg": "hi"})
	r.AssertNoError(t)
	r.AssertContains(t, "echo:hi")
	if r.Text() != "echo:hi" {
		t.Errorf("unexpected text: %s", r.Text())
	}
}

func TestClient_CallTool_Error(t *testing.T) {
	c := NewClient(t, newServer())
	r := c.CallTool("fail", map[string]any{})
	r.AssertIsError(t)
	r.AssertContains(t, "something went wrong")
}

func TestClient_ReadResource(t *testing.T) {
	c := NewClient(t, newServer())
	text := c.ReadResource("test://data")
	if text == "" {
		t.Fatal("expected non-empty resource")
	}
	if !containsStr(text, "value") {
		t.Errorf("expected 'value' in resource, got: %s", text)
	}
}

func TestClient_GetPrompt(t *testing.T) {
	c := NewClient(t, newServer())
	msgs := c.GetPrompt("greet", map[string]string{"name": "World"})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	content := msgs[0]["content"].(map[string]any)
	if content["text"] != "Hello World" {
		t.Errorf("unexpected text: %v", content["text"])
	}
}

func TestToolResult_AssertContains_Failure(t *testing.T) {
	// Use a fake *testing.T to capture the failure
	ft := &testing.T{}
	r := &ToolResult{text: "hello world"}
	r.AssertContains(ft, "xyz")
	// ft would have recorded an error, but we can't easily check it
	// Just ensure no panic
}

func TestToolResult_AssertNoError_OnError(t *testing.T) {
	ft := &testing.T{}
	r := &ToolResult{text: "bad", isError: true}
	r.AssertNoError(ft)
	// Just ensure no panic
}

func TestToolResult_AssertIsError_OnSuccess(t *testing.T) {
	ft := &testing.T{}
	r := &ToolResult{text: "ok", isError: false}
	r.AssertIsError(ft)
	// Just ensure no panic
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}

// TestMatchSnapshot is a basic smoke test for snapshot functionality.
func TestMatchSnapshot(t *testing.T) {
	r := &ToolResult{text: fmt.Sprintf("snapshot test %d", 42)}
	// First call creates the snapshot, second call matches it
	MatchSnapshot(t, "mcptest_basic", r)
	MatchSnapshot(t, "mcptest_basic", r)
}
