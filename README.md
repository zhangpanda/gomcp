# GoMCP

The fast, idiomatic way to build MCP servers in Go.

GoMCP is a framework — not just an SDK — for building [Model Context Protocol](https://modelcontextprotocol.io) servers. It brings a Gin-like developer experience to MCP: struct-tag schema generation, middleware chains, tool groups, and adapters that turn existing services into MCP tools with minimal code.

## Features

- **Struct-tag auto schema** — define tool parameters with Go structs and `mcp` tags, JSON Schema is generated automatically
- **Typed handlers** — `func(*Context, Input) (Output, error)` — no manual parameter parsing
- **Middleware chain** — Logger, Recovery, RateLimit, Timeout, RequestID — or write your own
- **Tool groups** — organize tools with prefixes and group-level middleware (like Gin's `RouterGroup`)
- **Resource & Prompt** — full MCP support including URI templates and parameterized prompts
- **Parameter validation** — required, min/max, enum, pattern — checked before your handler runs
- **Multiple transports** — stdio (Claude Desktop, Cursor, Kiro) and Streamable HTTP (remote deployment)
- **Zero-dependency core** — the core framework uses only the Go standard library; adapters (Gin, OpenAPI) and OpenTelemetry middleware bring their own dependencies

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

That's it. The `SearchInput` struct automatically generates this JSON Schema:

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

## Struct Tag Reference

Use the `mcp` tag on struct fields to control schema generation and validation:

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Field is required | `mcp:"required"` |
| `desc` | Field description | `mcp:"desc=User name"` |
| `default` | Default value | `mcp:"default=10"` |
| `min` | Minimum value | `mcp:"min=0"` |
| `max` | Maximum value | `mcp:"max=100"` |
| `enum` | Allowed values | `mcp:"enum=asc\|desc"` |
| `pattern` | Regex pattern | `mcp:"pattern=^[a-z]+$"` |

Combine multiple tags: `mcp:"required,desc=Search query,min=1"`

## Tool Groups

Organize tools with prefixes and shared middleware:

```go
s := gomcp.New("platform", "1.0.0")

// Group: user tools → user.get, user.update
user := s.Group("user", authMiddleware)
user.Tool("get", "Get user", getUser)
user.Tool("update", "Update user", updateUser)

// Nested group: user.admin.delete
admin := user.Group("admin", adminOnly)
admin.Tool("delete", "Delete user", deleteUser)
```

## Middleware

Built-in middleware:

```go
s.Use(gomcp.Logger())           // Log every tool call with duration
s.Use(gomcp.Recovery())         // Recover from panics
s.Use(gomcp.RequestID())        // Inject unique request ID
s.Use(gomcp.Timeout(5*time.Second)) // Enforce execution deadline
s.Use(gomcp.RateLimit(100))     // 100 calls/minute token bucket
```

Write your own:

```go
func MyMiddleware() gomcp.Middleware {
    return func(ctx *gomcp.Context, next func() error) error {
        // before
        err := next()
        // after
        return err
    }
}
```

## Resources

Expose data to AI models:

```go
// Static resource
s.Resource("config://app", "App Config", func(ctx *gomcp.Context) (any, error) {
    return map[string]any{"version": "1.0"}, nil
})

// Dynamic resource with URI template
s.ResourceTemplate("db://{table}/{id}", "DB Record", func(ctx *gomcp.Context) (any, error) {
    table := ctx.String("table")
    id := ctx.String("id")
    return db.Find(table, id), nil
})
```

## Prompts

Reusable prompt templates:

```go
s.Prompt("code_review", "Code review",
    []gomcp.PromptArgument{gomcp.PromptArg("language", "Language", true)},
    func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
        lang := ctx.String("language")
        return []gomcp.PromptMessage{
            gomcp.UserMsg(fmt.Sprintf("Review this %s code for bugs.", lang)),
        }, nil
    },
)
```

## Transports

```go
s.Stdio()          // stdin/stdout — for Claude Desktop, Cursor, Kiro
s.HTTP(":8080")    // Streamable HTTP — for remote deployment
```

## Use with Claude Desktop

Add to `claude_desktop_config.json`:

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
- [x] Tool groups with prefix naming
- [x] stdio + Streamable HTTP transports
- [x] Gin adapter — import existing Gin routes as MCP tools
- [x] OpenAPI adapter — generate tools from Swagger/OpenAPI docs
- [x] OpenTelemetry integration
- [x] mcptest package
- [ ] gRPC adapter
- [ ] Component versioning
- [ ] Async tasks (Task-augmented Tools)
- [ ] MCP Inspector (web debug UI)
- [ ] Auth middleware (Bearer / API Key / OAuth)
- [ ] Hot-reload provider

## License

Apache 2.0
