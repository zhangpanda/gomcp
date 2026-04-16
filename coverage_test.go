package gomcp_test

import (
	"testing"

	"github.com/istarshine/gomcp"
	"github.com/istarshine/gomcp/mcptest"
)

// --- Group.ToolFunc ---

type MathInput struct {
	A int `json:"a" mcp:"required"`
	B int `json:"b" mcp:"required"`
}

func TestGroup_ToolFunc(t *testing.T) {
	s := gomcp.New("test", "1.0")
	g := s.Group("math")
	g.ToolFunc("add", "Add numbers", func(ctx *gomcp.Context, in MathInput) (int, error) {
		return in.A + in.B, nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("math.add", map[string]any{"a": 3, "b": 4})
	r.AssertNoError(t)
	r.AssertContains(t, "7")
}

func TestGroup_ToolFunc_WithVersion(t *testing.T) {
	s := gomcp.New("test", "1.0")
	g := s.Group("math")
	g.ToolFunc("add", "Add v2", func(ctx *gomcp.Context, in MathInput) (int, error) {
		return in.A + in.B, nil
	}, gomcp.Version("2.0"))

	c := mcptest.NewClient(t, s)
	tools := c.ListTools()
	found := false
	for _, name := range tools {
		if name == "math.add@2.0" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected math.add@2.0, got: %v", tools)
	}
}

func TestGroup_ToolFunc_WithMiddleware(t *testing.T) {
	var called bool
	mw := func(ctx *gomcp.Context, next func() error) error {
		called = true
		return next()
	}

	s := gomcp.New("test", "1.0")
	g := s.Group("api", mw)
	g.ToolFunc("calc", "calc", func(ctx *gomcp.Context, in MathInput) (int, error) {
		return in.A * in.B, nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("api.calc", map[string]any{"a": 5, "b": 6})
	r.AssertNoError(t)
	r.AssertContains(t, "30")
	if !called {
		t.Error("group middleware was not called")
	}
}

func TestGroup_Use(t *testing.T) {
	var order []string
	s := gomcp.New("test", "1.0")
	g := s.Group("x")
	g.Use(func(ctx *gomcp.Context, next func() error) error {
		order = append(order, "mw1")
		return next()
	})
	g.Use(func(ctx *gomcp.Context, next func() error) error {
		order = append(order, "mw2")
		return next()
	})
	g.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		order = append(order, "handler")
		return ctx.Text("ok"), nil
	})

	c := mcptest.NewClient(t, s)
	c.CallTool("x.ping", nil)
	if len(order) != 3 || order[0] != "mw1" || order[1] != "mw2" {
		t.Errorf("expected [mw1 mw2 handler], got: %v", order)
	}
}

// --- toResult branches ---

func TestToolFunc_ReturnsString(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.ToolFunc("greet", "greet", func(ctx *gomcp.Context, in struct {
		Name string `json:"name"`
	}) (string, error) {
		return "hi " + in.Name, nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("greet", map[string]any{"name": "bob"})
	r.AssertNoError(t)
	r.AssertContains(t, "hi bob")
}

func TestToolFunc_ReturnsStruct(t *testing.T) {
	type Out struct {
		Sum int `json:"sum"`
	}
	s := gomcp.New("test", "1.0")
	s.ToolFunc("add", "add", func(ctx *gomcp.Context, in MathInput) (Out, error) {
		return Out{Sum: in.A + in.B}, nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("add", map[string]any{"a": 1, "b": 2})
	r.AssertNoError(t)
	r.AssertContains(t, `"sum": 3`)
}

func TestToolFunc_ReturnsCallToolResult(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.ToolFunc("custom", "custom", func(ctx *gomcp.Context, in struct{}) (*gomcp.CallToolResult, error) {
		return ctx.Text("custom result"), nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("custom", nil)
	r.AssertNoError(t)
	r.AssertContains(t, "custom result")
}

// --- Context.JSON and Context.Error ---

func TestContext_JSON(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("json", "json", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.JSON(map[string]int{"x": 42}), nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("json", nil)
	r.AssertNoError(t)
	r.AssertContains(t, `"x": 42`)
}

func TestContext_Error(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Tool("err", "err", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Error("something bad"), nil
	})

	c := mcptest.NewClient(t, s)
	r := c.CallTool("err", nil)
	r.AssertIsError(t)
	r.AssertContains(t, "something bad")
}

// --- WithDescription / WithLogger ---

func TestWithDescription(t *testing.T) {
	s := gomcp.New("test", "1.0", gomcp.WithDescription("my desc"))
	_ = s // just ensure no panic
}

func TestWithLogger(t *testing.T) {
	s := gomcp.New("test", "1.0", gomcp.WithLogger(nil))
	_ = s
}

// --- AssistantMsg ---

func TestAssistantMsg(t *testing.T) {
	msg := gomcp.AssistantMsg("hello")
	if msg.Role != "assistant" {
		t.Errorf("expected assistant role, got: %s", msg.Role)
	}
}
