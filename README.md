# GoMCP

The fast, idiomatic way to build MCP servers in Go.

[中文文档](README_CN.md)

GoMCP is a framework — not just an SDK — for building [Model Context Protocol](https://modelcontextprotocol.io) servers. It brings a Gin-like developer experience to MCP: struct-tag schema generation, middleware chains, tool groups, and adapters that turn existing services into MCP tools with minimal code.

## Features

- **Struct-tag auto schema** — define tool parameters with Go structs and `mcp` tags, JSON Schema is generated automatically
- **Typed handlers** — `func(*Context, Input) (Output, error)` — no manual parameter parsing
- **Middleware chain** — Logger, Recovery, RateLimit, Timeout, RequestID, OpenTelemetry — or write your own
- **Auth middleware** — BearerAuth, APIKeyAuth, BasicAuth + RequireRole / RequirePermission authorization
- **Tool groups** — organize tools with prefixes and group-level middleware (like Gin's `RouterGroup`)
- **Resource & Prompt** — full MCP support including URI templates and parameterized prompts
- **Parameter validation** — required, min/max, enum, pattern — checked before your handler runs
- **Component versioning** — register multiple versions of a tool, clients can call `name@version`
- **Async tasks** — long-running tools return a task ID, with status polling and cancellation
- **Multiple transports** — stdio (Claude Desktop, Cursor, Kiro) and Streamable HTTP with SSE notifications
- **Adapters** — import existing Gin routes, OpenAPI specs, or gRPC services as MCP tools
- **MCP Inspector** — built-in web debug UI for browsing and testing tools
- **Hot-reload** — load tool definitions from YAML files with file watching
- **Zero-dependency core** — the core framework uses only the Go standard library; adapters and OpenTelemetry middleware bring their own dependencies

## Quick Start

```bash
go get github.com/istarshine/gomcp
```

```go
package main

import (
    "fmt"
    "github.com/istarshine/gomcp"
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
    s.Use(gomcp.Logger())
    s.Use(gomcp.Recovery())

    s.ToolFunc("search", "Search documents", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
        items := []string{fmt.Sprintf("Result for %q", in.Query)}
        return SearchResult{Items: items, Total: len(items)}, nil
    })

    s.Stdio() // or s.HTTP(":8080")
}
```

## Struct Tag Reference

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field is required | `mcp:"required"` |
| `desc` | Field description | `mcp:"desc=User name"` |
| `default` | Default value | `mcp:"default=10"` |
| `min` | Minimum value | `mcp:"min=0"` |
| `max` | Maximum value | `mcp:"max=100"` |
| `enum` | Allowed values | `mcp:"enum=asc\|desc"` |
| `pattern` | Regex pattern | `mcp:"pattern=^[a-z]+$"` |

## Tool Groups

```go
user := s.Group("user", authMiddleware)
user.Tool("get", "Get user", getUser)       // → user.get
admin := user.Group("admin", adminOnly)
admin.Tool("delete", "Delete user", deleteUser) // → user.admin.delete
```

## Middleware

```go
s.Use(gomcp.Logger())                    // Log every tool call
s.Use(gomcp.Recovery())                  // Recover from panics
s.Use(gomcp.RequestID())                 // Inject unique request ID
s.Use(gomcp.Timeout(5*time.Second))      // Enforce deadline
s.Use(gomcp.RateLimit(100))              // 100 calls/minute
s.Use(gomcp.BearerAuth(tokenValidator))  // JWT Bearer auth
s.Use(gomcp.APIKeyAuth("X-API-Key", keyValidator)) // API key auth
s.Use(gomcp.BasicAuth(credValidator))    // HTTP Basic auth
```

Authorization:

```go
admin := s.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "Delete", handler)

writer := s.Group("data", gomcp.RequirePermission("write"))
writer.Tool("save", "Save", handler)
```

## Component Versioning

```go
s.Tool("search", "Search v1", searchV1, gomcp.Version("1.0"))
s.Tool("search", "Search v2", searchV2, gomcp.Version("2.0"))
// Client calls "search" → latest, or "search@1.0" → exact version
```

## Async Tasks

```go
s.AsyncTool("report", "Generate report", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    // long-running work...
    return ctx.Text("done"), nil
})
// Returns {"taskId":"abc123"} immediately
// Client polls tasks/get, can call tasks/cancel
```

## Adapters

### Gin

```go
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v1/"},
})
```

### OpenAPI

```go
adapter.ImportOpenAPI(s, "./swagger.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"pets"},
    ServerURL: "https://api.example.com",
})
```

### gRPC

```go
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService"},
})
```

## Hot-Reload Provider

```go
s.LoadDir("./tools/", gomcp.DirOptions{Watch: true})
```

YAML tool definition:

```yaml
name: search_user
description: Search users
version: "1.0"
params:
  - name: query
    type: string
    required: true
handler: http://localhost:8080/api/users/search
method: GET
```

## MCP Inspector

```go
s.Dev(":9090") // Open http://localhost:9090
```

Built-in web UI for browsing tools, resources, prompts and executing them inline.

## Transports

```go
s.Stdio()          // stdin/stdout — for Claude Desktop, Cursor, Kiro
s.HTTP(":8080")    // Streamable HTTP with SSE notifications
s.Handler()        // http.Handler for embedding in existing servers
```

## Use with Claude Desktop

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

## Roadmap

- [x] Core: Tool, Resource, Prompt
- [x] Struct-tag auto schema generation
- [x] Parameter validation
- [x] Middleware system (Logger, Recovery, RateLimit, Timeout, RequestID)
- [x] Auth middleware (Bearer / API Key / Basic) + Role/Permission authorization
- [x] Tool groups with prefix naming
- [x] stdio + Streamable HTTP transports (with SSE notifications)
- [x] Gin adapter
- [x] OpenAPI adapter (with $ref and requestBody support)
- [x] gRPC adapter
- [x] OpenTelemetry integration
- [x] mcptest package
- [x] Component versioning
- [x] Async tasks (Task-augmented Tools)
- [x] MCP Inspector (web debug UI)
- [x] Hot-reload provider
- [ ] MCP Client support
- [ ] Plugin marketplace CLI
- [ ] Code generator

## License

Apache 2.0
