package gomcp_test

import (
	"fmt"
	"testing"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/mcptest"
)

type SearchInput struct {
	Query string `json:"query" mcp:"required,desc=keyword"`
	Limit int    `json:"limit" mcp:"default=10,min=1,max=100"`
}

type SearchResult struct {
	Items []string `json:"items"`
	Total int      `json:"total"`
}

func newTestServer() *gomcp.Server {
	s := gomcp.New("test-server", "0.1.0")

	s.Tool("hello", "Say hello", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		name := ctx.String("name")
		if name == "" {
			name = "World"
		}
		return ctx.Text(fmt.Sprintf("Hello, %s!", name)), nil
	})

	s.ToolFunc("search", "Search", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		return SearchResult{Items: []string{"r1"}, Total: 1}, nil
	})

	s.Resource("config://app", "Config", func(ctx *gomcp.Context) (any, error) {
		return map[string]string{"env": "test"}, nil
	})

	s.ResourceTemplate("users://{id}/profile", "User", func(ctx *gomcp.Context) (any, error) {
		return map[string]string{"id": ctx.String("id")}, nil
	})

	s.Prompt("review", "Code review",
		[]gomcp.PromptArgument{gomcp.PromptArg("lang", "Language", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{gomcp.UserMsg("Review " + ctx.String("lang"))}, nil
		},
	)

	return s
}

func TestInitialize(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	result := c.Initialize()
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol version: %v", result["protocolVersion"])
	}
}

func TestListTools(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()
	tools := c.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(tools), tools)
	}
}

func TestCallTool_Simple(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	r := c.CallTool("hello", map[string]any{"name": "GoMCP"})
	r.AssertNoError(t)
	r.AssertContains(t, "Hello, GoMCP!")
}

func TestCallTool_Typed(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	r := c.CallTool("search", map[string]any{"query": "test", "limit": 5})
	r.AssertNoError(t)
	r.AssertContains(t, "r1")
}

func TestCallTool_ValidationFails(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	// missing required "query", limit out of range
	r := c.CallTool("search", map[string]any{"limit": 200})
	r.AssertIsError(t)
	r.AssertContains(t, "query: required")
	r.AssertContains(t, "must be <= 100")
}

func TestReadResource_Static(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	text := c.ReadResource("config://app")
	if text == "" {
		t.Fatal("expected non-empty resource")
	}
	if !contains(text, "test") {
		t.Errorf("expected resource to contain 'test', got: %s", text)
	}
}

func TestReadResource_Template(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	text := c.ReadResource("users://42/profile")
	if !contains(text, "42") {
		t.Errorf("expected resource to contain '42', got: %s", text)
	}
}

func TestGetPrompt(t *testing.T) {
	c := mcptest.NewClient(t, newTestServer())
	c.Initialize()

	msgs := c.GetPrompt("review", map[string]string{"lang": "Go"})
	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}
	content := msgs[0]["content"].(map[string]any)
	if text := content["text"].(string); text != "Review Go" {
		t.Errorf("unexpected prompt text: %s", text)
	}
}

func TestGroup(t *testing.T) {
	s := gomcp.New("test", "1.0.0")
	g := s.Group("admin")
	g.Tool("delete", "Delete", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("deleted"), nil
	})

	c := mcptest.NewClient(t, s)
	c.Initialize()

	tools := c.ListTools()
	found := false
	for _, name := range tools {
		if name == "admin.delete" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tool 'admin.delete', got: %v", tools)
	}

	r := c.CallTool("admin.delete", map[string]any{})
	r.AssertNoError(t)
	r.AssertContains(t, "deleted")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && fmt.Sprintf("%s", s) != "" && // avoid import strings
		func() bool {
			for i := 0; i+len(substr) <= len(s); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}
