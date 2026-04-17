# GoMCP

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://gitee.com/rilegouasas/gomcp/releases)

**The fast, idiomatic way to build MCP servers in Go.**

[中文文档](README_CN.md) · [Gitee](https://gitee.com/rilegouasas/gomcp)

---

## What is GoMCP?

GoMCP is a **framework** for building [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers — not just an SDK. Think of it as **"Gin for MCP"**: it brings the developer experience Go engineers love to the world of AI tool integration.

MCP is the open protocol that lets AI applications (Claude Desktop, Cursor, Kiro, VS Code Copilot) call external tools, read data sources, and use prompt templates. GoMCP makes building those servers trivial.

### Why GoMCP over existing Go MCP libraries?

| | mcp-go (mark3labs) | Official Go SDK | **GoMCP** |
|---|---|---|---|
| Level | SDK | SDK | **Framework** |
| Schema generation | Manual (`mcp.WithString(...)`) | `jsonschema` tag | **`mcp` tag with validation** |
| Middleware | Basic hooks | None | **Full chain (Logger, Auth, RateLimit, OTel...)** |
| Tool groups | No | No | **Yes (`user.get`, `admin.delete`)** |
| Import existing Gin routes | No | No | **Yes, one line** |
| Import OpenAPI/Swagger | No | No | **Yes, one line** |
| Import gRPC services | No | No | **Yes** |
| Built-in auth | No | No | **Bearer, API Key, Basic + RBAC** |
| Inspector UI | No | No | **Yes** |
| Test utilities | Basic | No | **mcptest package** |

---

## Installation

```bash
go get github.com/zhangpanda/gomcp
```

Requires Go 1.25+.

---

## Quick Start

### 5 lines to a working MCP server

```go
package main

import (
    "fmt"
    "github.com/zhangpanda/gomcp"
)

type SearchInput struct {
    Query string `json:"query" mcp:"required,desc=Search keyword"`
    Limit int    `json:"limit" mcp:"default=10,min=1,max=100"`
}

type SearchResult struct {
    Items []string `json:"items"`
    Total int      `json:"total"`
}

func main() {
    s := gomcp.New("my-server", "1.0.0")

    s.ToolFunc("search", "Search documents by keyword", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
        items := []string{fmt.Sprintf("Result for %q", in.Query)}
        return SearchResult{Items: items, Total: len(items)}, nil
    })

    s.Stdio()
}
```

The `SearchInput` struct **automatically generates** this JSON Schema — no manual definition needed:

```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "Search keyword" },
    "limit": { "type": "integer", "default": 10, "minimum": 1, "maximum": 100 }
  },
  "required": ["query"]
}
```

Parameters are **validated before your handler runs**. If a client sends `{"limit": 200}` without `query`, they get:

```
validation failed: query: required; limit: must be <= 100
```

---

## Core Concepts

### Tools

Tools are functions that AI models can call. GoMCP supports two registration styles:

**Simple handler** — full control via Context:

```go
s.Tool("hello", "Say hello", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    name := ctx.String("name")
    return ctx.Text(fmt.Sprintf("Hello, %s!", name)), nil
})
```

**Typed handler** — struct input with auto schema (recommended):

```go
type CreateUserInput struct {
    Name  string `json:"name"  mcp:"required,desc=User's full name"`
    Email string `json:"email" mcp:"required,pattern=^[^@]+@[^@]+$"`
    Age   int    `json:"age"   mcp:"min=0,max=150"`
}

s.ToolFunc("create_user", "Create a new user", func(ctx *gomcp.Context, in CreateUserInput) (User, error) {
    return db.CreateUser(in.Name, in.Email, in.Age)
})
```

### Resources

Resources expose data to AI models (like GET endpoints):

```go
// Static resource
s.Resource("config://app", "Application config", func(ctx *gomcp.Context) (any, error) {
    return map[string]any{"version": "1.0", "env": "production"}, nil
})

// Dynamic resource with URI template
s.ResourceTemplate("db://{table}/{id}", "Database record", func(ctx *gomcp.Context) (any, error) {
    table := ctx.String("table")
    id := ctx.String("id")
    return db.Find(table, id), nil
})
```

### Prompts

Prompts are reusable message templates:

```go
s.Prompt("code_review", "Code review assistant",
    []gomcp.PromptArgument{
        gomcp.PromptArg("language", "Programming language", true),
        gomcp.PromptArg("focus", "Review focus area", false),
    },
    func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
        lang := ctx.String("language")
        focus := ctx.String("focus")
        if focus == "" {
            focus = "bugs, performance, and security"
        }
        return []gomcp.PromptMessage{
            gomcp.UserMsg(fmt.Sprintf("Review this %s code. Focus on: %s", lang, focus)),
        }, nil
    },
)
```

---

## Struct Tag Reference

The `mcp` tag controls schema generation and validation:

| Tag | Type | Description | Example |
|-----|------|-------------|---------|
| `required` | flag | Field must be provided | `mcp:"required"` |
| `desc` | string | Human-readable description | `mcp:"desc=Search keyword"` |
| `default` | any | Default value if not provided | `mcp:"default=10"` |
| `min` | number | Minimum value (inclusive) | `mcp:"min=0"` |
| `max` | number | Maximum value (inclusive) | `mcp:"max=100"` |
| `enum` | string | Pipe-separated allowed values | `mcp:"enum=asc\|desc\|random"` |
| `pattern` | string | Regex validation pattern | `mcp:"pattern=^[a-z]+$"` |

Combine multiple directives: `mcp:"required,desc=User email,pattern=^[^@]+@[^@]+$"`

Supported Go types: `string`, `int`, `float64`, `bool`, `[]T`, nested structs.

---

## Middleware

GoMCP uses a Gin-style middleware chain. Middleware runs in order, wrapping the handler:

```
Request → Logger → Recovery → Auth → RateLimit → Handler → RateLimit → Auth → Recovery → Logger → Response
```

### Built-in middleware

```go
s.Use(gomcp.Logger())                              // Log tool name + duration
s.Use(gomcp.Recovery())                            // Recover from panics gracefully
s.Use(gomcp.RequestID())                           // Inject unique ID into every request
s.Use(gomcp.Timeout(10 * time.Second))             // Kill slow handlers
s.Use(gomcp.RateLimit(100))                        // Token bucket: 100 calls/minute
s.Use(gomcp.OpenTelemetry())                       // Auto-trace every tool call
```

### Auth middleware

```go
// Authentication — verify identity
s.Use(gomcp.BearerAuth(func(token string) (*gomcp.AuthInfo, error) {
    claims, err := jwt.Verify(token)
    if err != nil {
        return nil, err
    }
    return &gomcp.AuthInfo{UserID: claims.Sub, Roles: claims.Roles}, nil
}))

s.Use(gomcp.APIKeyAuth("X-API-Key", func(key string) (*gomcp.AuthInfo, error) {
    // validate API key
}))

s.Use(gomcp.BasicAuth(func(user, pass string) (*gomcp.AuthInfo, error) {
    // validate credentials
}))

// Authorization — check permissions (apply to groups)
admin := s.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete_user", "Delete a user", deleteHandler)

writer := s.Group("data", gomcp.RequirePermission("data:write"))
writer.Tool("import", "Import data", importHandler)
```

### Custom middleware

```go
func AuditLog() gomcp.Middleware {
    return func(ctx *gomcp.Context, next func() error) error {
        start := time.Now()
        err := next()
        audit.Log(ctx.String("_tool_name"), time.Since(start), err)
        return err
    }
}

s.Use(AuditLog())
```

---

## Tool Groups

Organize tools by domain with shared middleware — like Gin's `RouterGroup`:

```go
s := gomcp.New("platform", "1.0.0")
s.Use(gomcp.Logger())

// Public tools
s.Tool("health", "Health check", healthHandler)

// User tools — require authentication
user := s.Group("user", authMiddleware)
user.Tool("get", "Get user profile", getUser)         // → user.get
user.Tool("update", "Update user profile", updateUser) // → user.update

// Admin tools — require admin role
admin := user.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "Delete user", deleteUser)        // → user.admin.delete
admin.Tool("ban", "Ban user", banUser)                 // → user.admin.ban
```

---

## Adapters

The killer feature. Turn existing services into MCP tools without rewriting anything.

### Gin Adapter

Already have a Gin API? One line to expose it via MCP:

```go
ginRouter := setupYourExistingGinApp() // your 100+ route Gin app

s := gomcp.New("my-api", "1.0.0")
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v1/"},
    ExcludePaths: []string{"/api/v1/internal/"},
})
s.Stdio()
```

**What happens:**
- `GET /api/v1/users` → Tool `get_api_v1_users`
- `GET /api/v1/users/:id` → Tool `get_api_v1_users_by_id` (id = required string param)
- `POST /api/v1/users` → Tool `post_api_v1_users` (body = JSON string param)
- `DELETE /api/v1/users/:id` → Tool `delete_api_v1_users_by_id`

Path parameters become required Tool parameters. Your existing Gin middleware (auth, logging) still runs.

### OpenAPI Adapter

Have a Swagger/OpenAPI spec? Generate MCP tools from it:

```go
s := gomcp.New("petstore", "1.0.0")
adapter.ImportOpenAPI(s, "./openapi.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"pets", "users"},  // only these tags
    ServerURL: "https://api.example.com",
    AuthToken: os.Getenv("API_TOKEN"),
})
s.Stdio()
```

Supports OpenAPI 3.0/3.1, `$ref` resolution, and `requestBody` schemas.

### gRPC Adapter

```go
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService", "order.OrderService"},
})
```

---

## Component Versioning

Register multiple versions of the same tool for gradual API evolution:

```go
s.ToolFunc("search", "Full-text search", searchV1, gomcp.Version("1.0"))
s.ToolFunc("search", "Semantic search with embeddings", searchV2, gomcp.Version("2.0"))

// Clients call:
//   "search"     → latest version (2.0)
//   "search@1.0" → exact version
```

Mark deprecated tools:

```go
s.Tool("old_search", "Legacy search", handler, gomcp.Version("0.9"), gomcp.Deprecated("Use search@2.0"))
```

---

## Async Tasks

For long-running operations that would otherwise time out:

```go
s.AsyncTool("generate_report", "Generate analytics report", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    data := fetchLargeDataset()
    report := analyze(data) // takes minutes
    return ctx.Text(report.Summary()), nil
})
```

The client receives a task ID immediately. Then:
- `tasks/get` — poll for status and result
- `tasks/cancel` — cancel a running task

---

## Hot-Reload Provider

Load tool definitions from YAML files. Changes are picked up automatically:

```go
s.LoadDir("./tools/", gomcp.DirOptions{
    Watch:   true,
    Pattern: "*.tool.yaml",
})
```

```yaml
# tools/search.tool.yaml
name: search_user
description: Search users by name
version: "1.0"
params:
  - name: query
    type: string
    required: true
    description: Search keyword
handler: http://localhost:8080/api/users/search
method: GET
```

---

## MCP Inspector

Built-in web UI for development and debugging:

```go
s.Dev(":9090") // visit http://localhost:9090
```

Browse all registered tools, resources, and prompts. Execute tools inline and inspect responses.

---

## Testing

The `mcptest` package provides an in-memory client for unit testing — no transport needed:

```go
func TestSearch(t *testing.T) {
    s := setupServer()
    c := mcptest.NewClient(t, s)
    c.Initialize()

    // Call a tool
    result := c.CallTool("search", map[string]any{"query": "golang", "limit": 5})
    result.AssertNoError(t)
    result.AssertContains(t, "golang")

    // Snapshot testing
    mcptest.MatchSnapshot(t, "search_result", result)

    // Test validation
    bad := c.CallTool("search", map[string]any{"limit": 999})
    bad.AssertIsError(t)
    bad.AssertContains(t, "query: required")

    // Test resources
    config := c.ReadResource("config://app")
    // ... assertions

    // Test prompts
    msgs := c.GetPrompt("code_review", map[string]string{"language": "Go"})
    // ... assertions
}
```

---

## Transports

```go
s.Stdio()          // stdin/stdout — Claude Desktop, Cursor, Kiro, etc.
s.HTTP(":8080")    // Streamable HTTP with SSE — remote deployment
s.Handler()        // http.Handler — embed in existing HTTP servers
```

### Claude Desktop / Cursor / Kiro

Add to your MCP client config:

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

### Remote deployment

```go
s.HTTP(":8080") // POST /mcp for requests, GET /mcp for SSE notifications
```

---

## Project Structure

```
gomcp/
├── server.go              # Server core, tool/resource/prompt registration
├── context.go             # Request context with typed accessors
├── group.go               # Tool groups with prefix naming
├── middleware.go           # Middleware interface and chain execution
├── middleware_builtin.go   # Logger, Recovery, RequestID, Timeout, RateLimit
├── middleware_auth.go      # BearerAuth, APIKeyAuth, BasicAuth, RBAC
├── middleware_otel.go      # OpenTelemetry tracing
├── schema/                # Struct tag → JSON Schema generator + validator
├── transport/             # stdio + Streamable HTTP
├── adapter/               # Gin, OpenAPI, gRPC adapters
├── mcptest/               # Testing utilities
├── task.go                # Async task support
├── completion.go          # Auto-completions
├── inspector.go           # Web debug UI
├── provider.go            # Hot-reload from YAML
└── examples/              # Working examples
```

---

## Roadmap

- [x] Core: Tool, Resource, Prompt with full MCP protocol support
- [x] Struct-tag auto schema generation + parameter validation
- [x] Middleware chain (Logger, Recovery, RateLimit, Timeout, RequestID)
- [x] Auth middleware (Bearer / API Key / Basic) + Role/Permission authorization
- [x] Tool groups with prefix naming and nested groups
- [x] stdio + Streamable HTTP transports with SSE notifications
- [x] Gin adapter — import existing Gin routes as MCP tools
- [x] OpenAPI adapter — generate tools from Swagger/OpenAPI docs
- [x] gRPC adapter — import gRPC services as MCP tools
- [x] OpenTelemetry integration
- [x] mcptest package with snapshot testing
- [x] Component versioning + deprecation
- [x] Async tasks with polling and cancellation
- [x] MCP Inspector web debug UI
- [x] Hot-reload provider from YAML
- [x] Auto-completions for prompt/resource arguments
- [ ] MCP Client support (server-to-server calls)

---

## Contributing

Contributions are welcome! Please open an issue or pull request.

## License

[Apache 2.0](LICENSE)
