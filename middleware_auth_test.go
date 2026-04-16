package gomcp_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/istarshine/gomcp"
	"github.com/istarshine/gomcp/transport"
)

func ctxWithAuth(scheme, cred string) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, transport.CtxKey("auth_header"), scheme+" "+cred)
}

func ctxWithHeaders(headers map[string]string) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, transport.CtxKey("http_headers"), headers)
}

func ctxWithBoth(authHeader string, headers map[string]string) context.Context {
	ctx := context.WithValue(context.Background(), transport.CtxKey("auth_header"), authHeader)
	return context.WithValue(ctx, transport.CtxKey("http_headers"), headers)
}

func callWithCtx(s *gomcp.Server, ctx context.Context, tool string, args map[string]any) (string, bool) {
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
	if msg.Result != nil {
		text := ""
		if len(msg.Result.Content) > 0 {
			text = msg.Result.Content[0].Text
		}
		return text, msg.Result.IsError
	}
	return "", true
}

// --- BearerAuth tests ---

func validTokenValidator(token string) (map[string]any, error) {
	if token == "valid-token" {
		return map[string]any{
			"sub":         "user123",
			"roles":       []string{"admin", "user"},
			"permissions": []string{"read", "write", "delete"},
		}, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func TestBearerAuth_Valid(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		claims, _ := ctx.Get("_auth_claims")
		m := claims.(map[string]any)
		return ctx.Text("hello " + m["sub"].(string)), nil
	})

	ctx := ctxWithAuth("Bearer", "valid-token")
	text, isErr := callWithCtx(s, ctx, "ping", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "hello user123" {
		t.Errorf("unexpected: %s", text)
	}
}

func TestBearerAuth_Invalid(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	ctx := ctxWithAuth("Bearer", "bad-token")
	text, _ := callWithCtx(s, ctx, "ping", nil)
	if !strings.Contains(text, "authentication failed") {
		t.Errorf("expected auth error, got: %s", text)
	}
}

func TestBearerAuth_Missing(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	text, _ := callWithCtx(s, context.Background(), "ping", nil)
	if !strings.Contains(text, "missing") {
		t.Errorf("expected missing token error, got: %s", text)
	}
}

// --- APIKeyAuth tests ---

func validKeyValidator(key string) (map[string]any, error) {
	if key == "secret-key-123" {
		return map[string]any{"app": "myapp", "roles": []string{"reader"}}, nil
	}
	return nil, fmt.Errorf("unknown key")
}

func TestAPIKeyAuth_FromHeader(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.APIKeyAuth("X-Api-Key", validKeyValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	ctx := ctxWithHeaders(map[string]string{"X-Api-Key": "secret-key-123"})
	text, isErr := callWithCtx(s, ctx, "ping", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
}

func TestAPIKeyAuth_FromParam(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.APIKeyAuth("X-Api-Key", validKeyValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	text, isErr := callWithCtx(s, context.Background(), "ping", map[string]any{"api_key": "secret-key-123"})
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
}

func TestAPIKeyAuth_Invalid(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.APIKeyAuth("X-Api-Key", validKeyValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	ctx := ctxWithHeaders(map[string]string{"X-Api-Key": "wrong"})
	text, _ := callWithCtx(s, ctx, "ping", nil)
	if !strings.Contains(text, "invalid API key") {
		t.Errorf("expected invalid key error, got: %s", text)
	}
}

func TestAPIKeyAuth_Missing(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.APIKeyAuth("X-Api-Key", validKeyValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	text, _ := callWithCtx(s, context.Background(), "ping", nil)
	if !strings.Contains(text, "missing API key") {
		t.Errorf("expected missing key error, got: %s", text)
	}
}

// --- BasicAuth tests ---

func validBasicValidator(user, pass string) (map[string]any, error) {
	if user == "admin" && pass == "secret" {
		return map[string]any{"roles": []string{"admin"}}, nil
	}
	return nil, fmt.Errorf("bad credentials")
}

func TestBasicAuth_Valid(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BasicAuth(validBasicValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		user, _ := ctx.Get("_auth_user")
		return ctx.Text("hello " + user.(string)), nil
	})

	cred := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	ctx := ctxWithAuth("Basic", cred)
	text, isErr := callWithCtx(s, ctx, "ping", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "hello admin" {
		t.Errorf("unexpected: %s", text)
	}
}

func TestBasicAuth_Invalid(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BasicAuth(validBasicValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	cred := base64.StdEncoding.EncodeToString([]byte("admin:wrong"))
	ctx := ctxWithAuth("Basic", cred)
	text, _ := callWithCtx(s, ctx, "ping", nil)
	if !strings.Contains(text, "authentication failed") {
		t.Errorf("expected auth error, got: %s", text)
	}
}

func TestBasicAuth_BadEncoding(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BasicAuth(validBasicValidator))
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	ctx := ctxWithAuth("Basic", "not-valid-base64!!!")
	text, _ := callWithCtx(s, ctx, "ping", nil)
	if !strings.Contains(text, "invalid Basic auth") {
		t.Errorf("expected encoding error, got: %s", text)
	}
}

// --- RequireRole / RequirePermission tests ---

func TestRequireRole_Allowed(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))

	g := s.Group("admin", gomcp.RequireRole("admin"))
	g.Tool("action", "admin action", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("done"), nil
	})

	ctx := ctxWithAuth("Bearer", "valid-token")
	text, isErr := callWithCtx(s, ctx, "admin.action", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
	if text != "done" {
		t.Errorf("unexpected: %s", text)
	}
}

func TestRequireRole_Denied(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))

	g := s.Group("super", gomcp.RequireRole("superadmin"))
	g.Tool("action", "super action", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("done"), nil
	})

	ctx := ctxWithAuth("Bearer", "valid-token")
	text, _ := callWithCtx(s, ctx, "super.action", nil)
	if !strings.Contains(text, "requires role superadmin") {
		t.Errorf("expected role denied, got: %s", text)
	}
}

func TestRequirePermission_Allowed(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))

	g := s.Group("data", gomcp.RequirePermission("write"))
	g.Tool("save", "save data", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("saved"), nil
	})

	ctx := ctxWithAuth("Bearer", "valid-token")
	text, isErr := callWithCtx(s, ctx, "data.save", nil)
	if isErr {
		t.Fatalf("unexpected error: %s", text)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	s := gomcp.New("test", "1.0")
	s.Use(gomcp.BearerAuth(validTokenValidator))

	g := s.Group("data", gomcp.RequirePermission("superpower"))
	g.Tool("nuke", "nuke", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("boom"), nil
	})

	ctx := ctxWithAuth("Bearer", "valid-token")
	text, _ := callWithCtx(s, ctx, "data.nuke", nil)
	if !strings.Contains(text, "requires permission superpower") {
		t.Errorf("expected permission denied, got: %s", text)
	}
}

func TestRequireRole_NoAuth(t *testing.T) {
	s := gomcp.New("test", "1.0")
	// No auth middleware, just RequireRole
	g := s.Group("admin", gomcp.RequireRole("admin"))
	g.Tool("action", "action", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("done"), nil
	})

	text, _ := callWithCtx(s, context.Background(), "admin.action", nil)
	if !strings.Contains(text, "no roles") {
		t.Errorf("expected no roles error, got: %s", text)
	}
}
