# GoMCP

用 Go 构建 MCP Server 的最快方式。

[English](README.md)

GoMCP 是一个**框架**，不只是 SDK。它为 [Model Context Protocol](https://modelcontextprotocol.io) 服务端开发带来了类似 Gin 的开发体验：struct tag 自动生成 schema、中间件链、工具分组、适配器一键接入现有服务。

## 特性

- **Struct tag 自动 schema** — 用 Go 结构体和 `mcp` tag 定义参数，JSON Schema 自动生成
- **类型安全的 handler** — `func(*Context, Input) (Output, error)` — 无需手动解析参数
- **中间件链** — Logger、Recovery、RateLimit、Timeout、RequestID、OpenTelemetry，或自定义
- **认证中间件** — BearerAuth、APIKeyAuth、BasicAuth + RequireRole / RequirePermission 授权
- **工具分组** — 按前缀组织工具，支持分组级中间件（类似 Gin 的 `RouterGroup`）
- **Resource & Prompt** — 完整 MCP 支持，包括 URI 模板和参数化 Prompt
- **参数校验** — required、min/max、enum、pattern — 在 handler 执行前自动校验
- **组件版本化** — 同一工具注册多个版本，客户端可通过 `name@version` 调用
- **异步任务** — 长时间运行的工具立即返回 task ID，支持状态轮询和取消
- **多传输层** — stdio（Claude Desktop、Cursor、Kiro）和 Streamable HTTP（含 SSE 通知）
- **适配器** — 一键导入 Gin 路由、OpenAPI 文档、gRPC 服务为 MCP 工具
- **MCP Inspector** — 内置 Web 调试界面，浏览和测试工具
- **热加载** — 从 YAML 文件加载工具定义，支持文件监听自动重载
- **核心零依赖** — 核心框架仅使用 Go 标准库；适配器和 OpenTelemetry 中间件有各自的依赖

## 快速开始

```bash
go get github.com/zhangpanda/gomcp
```

```go
package main

import (
    "fmt"
    "github.com/zhangpanda/gomcp"
)

type SearchInput struct {
    Query string `json:"query" mcp:"required,desc=搜索关键词"`
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

    s.ToolFunc("search", "搜索文档", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
        items := []string{fmt.Sprintf("搜索 %q 的结果", in.Query)}
        return SearchResult{Items: items, Total: len(items)}, nil
    })

    s.Stdio() // 或 s.HTTP(":8080")
}
```

## Struct Tag 参考

| Tag | 说明 | 示例 |
|-----|------|------|
| `required` | 必填字段 | `mcp:"required"` |
| `desc` | 字段描述 | `mcp:"desc=用户名"` |
| `default` | 默认值 | `mcp:"default=10"` |
| `min` | 最小值 | `mcp:"min=0"` |
| `max` | 最大值 | `mcp:"max=100"` |
| `enum` | 枚举值 | `mcp:"enum=asc\|desc"` |
| `pattern` | 正则校验 | `mcp:"pattern=^[a-z]+$"` |

## 工具分组

```go
user := s.Group("user", authMiddleware)
user.Tool("get", "获取用户", getUser)       // → user.get
admin := user.Group("admin", adminOnly)
admin.Tool("delete", "删除用户", deleteUser) // → user.admin.delete
```

## 中间件

```go
s.Use(gomcp.Logger())                    // 记录每次工具调用
s.Use(gomcp.Recovery())                  // panic 恢复
s.Use(gomcp.RequestID())                 // 注入唯一请求 ID
s.Use(gomcp.Timeout(5*time.Second))      // 超时控制
s.Use(gomcp.RateLimit(100))              // 每分钟 100 次限流
s.Use(gomcp.BearerAuth(tokenValidator))  // JWT Bearer 认证
s.Use(gomcp.APIKeyAuth("X-API-Key", keyValidator)) // API Key 认证
s.Use(gomcp.BasicAuth(credValidator))    // HTTP Basic 认证
```

授权：

```go
admin := s.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "删除", handler)

writer := s.Group("data", gomcp.RequirePermission("write"))
writer.Tool("save", "保存", handler)
```

## 组件版本化

```go
s.Tool("search", "搜索 v1", searchV1, gomcp.Version("1.0"))
s.Tool("search", "搜索 v2", searchV2, gomcp.Version("2.0"))
// 客户端调用 "search" → 最新版本，"search@1.0" → 指定版本
```

## 异步任务

```go
s.AsyncTool("report", "生成报告", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    // 长时间运行的任务...
    return ctx.Text("完成"), nil
})
// 立即返回 {"taskId":"abc123"}
// 客户端通过 tasks/get 轮询状态，tasks/cancel 取消任务
```

## 适配器

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

## 热加载

```go
s.LoadDir("./tools/", gomcp.DirOptions{Watch: true})
```

YAML 工具定义：

```yaml
name: search_user
description: 搜索用户
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
s.Dev(":9090") // 打开 http://localhost:9090
```

内置 Web 界面，可浏览所有工具、资源、Prompt，并在线调用测试。

## 传输层

```go
s.Stdio()          // stdin/stdout — 用于 Claude Desktop、Cursor、Kiro
s.HTTP(":8080")    // Streamable HTTP，支持 SSE 通知
s.Handler()        // http.Handler，嵌入现有 HTTP 服务
```

## 配合 Claude Desktop 使用

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

## 路线图

- [x] 核心：Tool、Resource、Prompt
- [x] Struct tag 自动 schema 生成
- [x] 参数校验
- [x] 中间件系统（Logger、Recovery、RateLimit、Timeout、RequestID）
- [x] 认证中间件（Bearer / API Key / Basic）+ 角色/权限授权
- [x] 工具分组
- [x] stdio + Streamable HTTP 传输（含 SSE 通知）
- [x] Gin 适配器
- [x] OpenAPI 适配器（支持 $ref 和 requestBody）
- [x] gRPC 适配器
- [x] OpenTelemetry 集成
- [x] mcptest 测试包
- [x] 组件版本化
- [x] 异步任务（Task-augmented Tools）
- [x] MCP Inspector（Web 调试界面）
- [x] 热加载 Provider
- [x] Prompt/Resource 参数自动补全
- [ ] MCP Client 支持
- [ ] 插件市场 CLI
- [ ] 代码生成器

## 许可证

Apache 2.0
