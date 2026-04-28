# GoMCP

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://github.com/zhangpanda/gomcp/releases)
[![gomcp MCP server](https://glama.ai/mcp/servers/zhangpanda/gomcp/badges/score.svg)](https://glama.ai/mcp/servers/zhangpanda/gomcp)

**The fast, idiomatic way to build MCP servers in Go.**

[中文文档](README_CN.md)

---

## 🚀 Quick Links

- **GitHub**: https://github.com/zhangpanda/gomcp
- **Gitee**: https://gitee.com/rilegouasas/gomcp
- **MCP Protocol**: https://modelcontextprotocol.io

---

## 🎯 What is GoMCP?

GoMCP is a **framework** for building [Model Context Protocol (MCP)](https://modelcontextprotocol.io) servers — not just an SDK. Think of it as **"Gin for MCP"**.

MCP is the open protocol that lets AI applications (Claude Desktop, Cursor, Kiro, VS Code Copilot) call external tools, read data sources, and use prompt templates. GoMCP makes building those servers trivial.

### Why GoMCP?

| | mcp-go (mark3labs) | Official Go SDK | **GoMCP** |
|---|---|---|---|
| Level | SDK | SDK | **Framework** |
| Schema generation | Manual | `jsonschema` tag | **`mcp` tag + auto validation** |
| Middleware | Basic hooks | None | **Full chain (Logger, Auth, RateLimit, OTel…)** |
| Tool groups | No | No | **Yes (`user.get`, `admin.delete`)** |
| Import Gin routes | No | No | **✅ One line** |
| Import OpenAPI/Swagger | No | No | **✅ One line** |
| Import gRPC services | No | No | **✅** |
| Built-in auth | No | No | **Bearer, API Key, Basic + RBAC** |
| Inspector UI | No | No | **✅** |
| Test utilities | Basic | No | **mcptest package** |

---

## 🛠️ Tech Stack

### Environment Requirements

| Requirement | Version |
|-------------|---------|
| **Go** | ≥ 1.25 |
| **MCP Protocol** | 2024-11-05 (backward compatible with 2025-11-25) |

### Core Dependencies

| Technology | Description |
|------------|-------------|
| **Go standard library** | Core framework — zero external dependencies |
| **Gin** | Adapter only — import existing Gin routes |
| **gRPC** | Adapter only — import gRPC services |
| **OpenTelemetry** | Optional — distributed tracing |
| **YAML v3** | Provider only — hot-reload tool definitions |

---

## 🌟 Core Features

### 🔧 Tool Development

- **Struct-tag auto schema** — define parameters with Go structs and `mcp` tags, JSON Schema generated automatically
- **Typed handlers** — `func(*Context, Input) (Output, error)` — no manual parameter parsing
- **Parameter validation** — required, min/max, enum, pattern — checked before your handler runs
- **Component versioning** — register multiple versions, clients call `name@version`
- **Async tasks** — long-running tools return task ID, with polling and cancellation

### 🔌 Adapters (Core Differentiator)

- **Gin adapter** — import existing Gin routes as MCP tools with one line
- **OpenAPI adapter** — generate tools from Swagger/OpenAPI 3.x docs
- **gRPC adapter** — import gRPC service methods as MCP tools

### 🔐 Security

- **BearerAuth** — JWT token validation
- **APIKeyAuth** — API key validation via header
- **BasicAuth** — HTTP Basic authentication
- **RequireRole / RequirePermission** — RBAC authorization on tool groups

### 🧩 Framework Features

- **Middleware chain** — Logger, Recovery, RequestID, Timeout, RateLimit, OpenTelemetry
- **Tool groups** — organize tools with prefixes and group-level middleware
- **Resource & Prompt** — full MCP support including URI templates and parameterized prompts
- **Auto-completions** — suggest values for prompt/resource arguments

### 🚀 Production Ready

- **Multiple transports** — stdio (Claude Desktop, Cursor, Kiro) and Streamable HTTP with SSE
- **MCP Inspector** — built-in web debug UI for browsing and testing tools
- **Hot-reload** — load tool definitions from YAML files with file watching
- **mcptest package** — in-memory client for unit testing with snapshot support

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        User Code                             │
│   s.Tool() / s.ToolFunc() / s.Resource() / s.Prompt()        │
├──────────────────────────────────────────────────────────────┤
│                     Framework Core                           │
│   Router → Middleware Chain → Validation → Handler → Result   │
├────────────┬─────────────┬───────────────┬───────────────────┤
│   Schema   │  Validator  │   Adapters    │  Observability    │
│  Generator │   Engine    │ Gin/OpenAPI/  │  OTel / Logger    │
│ (mcp tags) │ (auto)      │ gRPC          │  / Inspector      │
├────────────┴─────────────┴───────────────┴───────────────────┤
│                     Protocol Layer                           │
│          JSON-RPC 2.0 / MCP / Capability Negotiation         │
├──────────────────────────────────────────────────────────────┤
│                     Transport Layer                          │
│              stdio  /  Streamable HTTP + SSE                 │
└──────────────────────────────────────────────────────────────┘
```

### Project Structure

```
gomcp/
├── server.go              # Server core, tool/resource/prompt registration
├── context.go             # Request context with typed accessors
├── group.go               # Tool groups with prefix naming
├── middleware.go           # Middleware interface and chain execution
├── middleware_builtin.go   # Logger, Recovery, RequestID, Timeout, RateLimit
├── middleware_auth.go      # BearerAuth, APIKeyAuth, BasicAuth, RBAC
├── middleware_otel.go      # OpenTelemetry tracing
├── schema/                # struct tag → JSON Schema generator + validator
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

## 📦 Installation

```bash
go get github.com/zhangpanda/gomcp
```

---

## ⚡ Quick Start

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

The `SearchInput` struct **automatically generates** this JSON Schema:

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

Invalid parameters are **rejected before your handler runs**:

```
validation failed: query: required; limit: must be <= 100
```

---

## 📖 Usage Guide

### Struct Tag Reference

| Tag | Type | Description | Example |
|-----|------|-------------|---------|
| `required` | flag | Field must be provided | `mcp:"required"` |
| `desc` | string | Human-readable description | `mcp:"desc=Search keyword"` |
| `default` | any | Default value | `mcp:"default=10"` |
| `min` | number | Minimum value (inclusive) | `mcp:"min=0"` |
| `max` | number | Maximum value (inclusive) | `mcp:"max=100"` |
| `enum` | string | Pipe-separated allowed values | `mcp:"enum=asc\|desc"` |
| `pattern` | string | Regex validation | `mcp:"pattern=^[a-z]+$"` |

Combine: `mcp:"required,desc=User email,pattern=^[^@]+@[^@]+$"`

Supported types: `string`, `int`, `float64`, `bool`, `[]T`, nested structs.

### Tools

**Simple handler:**

```go
s.Tool("hello", "Say hello", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    return ctx.Text("Hello, " + ctx.String("name")), nil
})
```

**Typed handler (recommended):**

```go
type Input struct {
    Name  string `json:"name"  mcp:"required,desc=User name"`
    Email string `json:"email" mcp:"required,pattern=^[^@]+@[^@]+$"`
}

s.ToolFunc("create_user", "Create user", func(ctx *gomcp.Context, in Input) (User, error) {
    return db.CreateUser(in.Name, in.Email)
})
```

### Resources

```go
// Static
s.Resource("config://app", "App config", func(ctx *gomcp.Context) (any, error) {
    return map[string]any{"version": "1.0"}, nil
})

// Dynamic URI template
s.ResourceTemplate("db://{table}/{id}", "DB record", func(ctx *gomcp.Context) (any, error) {
    return db.Find(ctx.String("table"), ctx.String("id")), nil
})
```

### Prompts

```go
s.Prompt("code_review", "Code review",
    []gomcp.PromptArgument{gomcp.PromptArg("language", "Language", true)},
    func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
        return []gomcp.PromptMessage{
            gomcp.UserMsg(fmt.Sprintf("Review this %s code for bugs.", ctx.String("language"))),
        }, nil
    },
)
```

### Middleware

```go
s.Use(gomcp.Logger())                              // Log tool name + duration
s.Use(gomcp.Recovery())                            // Recover from panics
s.Use(gomcp.RequestID())                           // Unique request ID
s.Use(gomcp.Timeout(10 * time.Second))             // Deadline enforcement
s.Use(gomcp.RateLimit(100))                        // 100 calls/minute
s.Use(gomcp.OpenTelemetry())                       // Distributed tracing
s.Use(gomcp.BearerAuth(tokenValidator))            // JWT auth
s.Use(gomcp.APIKeyAuth("X-API-Key", keyValidator)) // API key auth
```

**Custom middleware:**

```go
func AuditLog() gomcp.Middleware {
    return func(ctx *gomcp.Context, next func() error) error {
        start := time.Now()
        err := next()
        log.Printf("tool=%s duration=%s err=%v", ctx.String("_tool_name"), time.Since(start), err)
        return err
    }
}
```

### Tool Groups

```go
user := s.Group("user", authMiddleware)
user.Tool("get", "Get user", getUser)              // → user.get
user.Tool("update", "Update user", updateUser)      // → user.update

admin := user.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "Delete user", deleteUser)     // → user.admin.delete
```

### Adapters

**Gin — one line to import your existing API:**

```go
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v1/"},
})
// GET /api/v1/users/:id → Tool get_api_v1_users_by_id (id = required param)
```

**OpenAPI — generate from Swagger docs:**

```go
adapter.ImportOpenAPI(s, "./swagger.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"pets"},
    ServerURL: "https://api.example.com",
})
```

**gRPC:**

```go
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService"},
})
```

### Component Versioning

```go
s.ToolFunc("search", "v1", searchV1, gomcp.Version("1.0"))
s.ToolFunc("search", "v2 with embeddings", searchV2, gomcp.Version("2.0"))
// "search" → latest, "search@1.0" → exact version
```

### Async Tasks

```go
s.AsyncTool("report", "Generate report", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    // long-running work
    return ctx.Text("done"), nil
})
// Client gets taskId immediately, polls tasks/get, can tasks/cancel
```

### Hot-Reload

```go
s.LoadDir("./tools/", gomcp.DirOptions{Watch: true})
```

### MCP Inspector

```go
s.Dev(":9090") // http://localhost:9090 — browse and test all tools
```

### Testing

```go
func TestSearch(t *testing.T) {
    c := mcptest.NewClient(t, setupServer())
    c.Initialize()

    result := c.CallTool("search", map[string]any{"query": "golang"})
    result.AssertNoError(t)
    result.AssertContains(t, "golang")

    mcptest.MatchSnapshot(t, "search_result", result)
}
```

### Transports

```go
s.Stdio()          // Claude Desktop, Cursor, Kiro
s.HTTP(":8080")    // Remote deployment with SSE
s.Handler()        // Embed in existing HTTP server
```

### Use with AI Clients

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

Works with Claude Desktop, Cursor, Kiro, Windsurf, VS Code Copilot, and any MCP-compatible client.

---

## 📋 Roadmap

- [x] Core: Tool, Resource, Prompt with full MCP protocol support
- [x] Struct-tag auto schema generation + parameter validation
- [x] Middleware chain (Logger, Recovery, RateLimit, Timeout, RequestID)
- [x] Auth middleware (Bearer / API Key / Basic) + RBAC authorization
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

---

## 🤝 Feedback & Support

- **Bug Reports**: [GitHub Issues](https://github.com/zhangpanda/gomcp/issues)
- **Feature Requests**: [GitHub Issues](https://github.com/zhangpanda/gomcp/issues)
- **Discussions**: [GitHub Discussions](https://github.com/zhangpanda/gomcp/discussions)

> 💡 Recommended reading: [How To Ask Questions The Smart Way](https://github.com/ryanhanwu/How-To-Ask-Questions-The-Smart-Way)

---

## 🔒 Security

To report security vulnerabilities, see [SECURITY.md](SECURITY.md).

---

## ⚖️ Copyright & License

Copyright © 2026 GoMCP Contributors

Licensed under the [Apache License 2.0](LICENSE).

### Important Notes

1. This project is **open source and free** for both personal and commercial use under the Apache 2.0 license.
2. You **must retain** the copyright notice, license text, and any attribution notices in all copies or substantial portions of the software.
3. The Apache 2.0 license includes an **express grant of patent rights** from contributors to users.
4. Contributions to this project are licensed under the same Apache 2.0 license.
5. Unauthorized removal of copyright notices may result in legal action.

### Patent Notice

Certain features of this framework (struct-tag schema generation, HTTP-to-MCP automatic adapter, OpenAPI-to-MCP automatic adapter) are the subject of pending patent applications. The Apache 2.0 license grants you a perpetual, worldwide, royalty-free patent license to use these features as part of this software.

---

## ⭐ Star History

If you find GoMCP useful, please consider giving it a star! It helps others discover the project.

