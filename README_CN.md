# GoMCP

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://gitee.com/rilegouasas/gomcp/releases)

**用 Go 构建 MCP Server 的最快方式。**

[English](README.md) · [Gitee](https://gitee.com/rilegouasas/gomcp)

---

## GoMCP 是什么？

GoMCP 是一个用于构建 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 服务端的 **Go 框架**——不只是 SDK。可以理解为 **"MCP 领域的 Gin"**：把 Go 工程师熟悉的开发体验带到 AI 工具集成领域。

MCP 是 Anthropic 发布的开放协议，让 AI 应用（Claude Desktop、Cursor、Kiro、VS Code Copilot）能够调用外部工具、读取数据源、使用 Prompt 模板。GoMCP 让构建这些服务变得极其简单。

### 为什么选 GoMCP？

| | mcp-go (mark3labs) | 官方 Go SDK | **GoMCP** |
|---|---|---|---|
| 定位 | SDK | SDK | **框架** |
| Schema 生成 | 手动 (`mcp.WithString(...)`) | `jsonschema` tag | **`mcp` tag + 自动校验** |
| 中间件 | 基础钩子 | 无 | **完整链（Logger、Auth、限流、OTel…）** |
| 工具分组 | 无 | 无 | **支持（`user.get`、`admin.delete`）** |
| 导入现有 Gin 路由 | 无 | 无 | **一行代码** |
| 导入 OpenAPI/Swagger | 无 | 无 | **一行代码** |
| 导入 gRPC 服务 | 无 | 无 | **支持** |
| 内置认证 | 无 | 无 | **Bearer、API Key、Basic + RBAC** |
| 调试界面 | 无 | 无 | **内置 Inspector** |
| 测试工具 | 基础 | 无 | **mcptest 包** |

---

## 安装

```bash
go get github.com/zhangpanda/gomcp
```

需要 Go 1.22+。

---

## 快速开始

### 5 行核心代码，一个完整的 MCP Server

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

    s.ToolFunc("search", "按关键词搜索文档", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
        items := []string{fmt.Sprintf("搜索 %q 的结果", in.Query)}
        return SearchResult{Items: items, Total: len(items)}, nil
    })

    s.Stdio()
}
```

`SearchInput` 结构体 **自动生成** 以下 JSON Schema——无需手动定义：

```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "搜索关键词" },
    "limit": { "type": "integer", "default": 10, "minimum": 1, "maximum": 100 }
  },
  "required": ["query"]
}
```

参数在 handler 执行前 **自动校验**。如果客户端发送 `{"limit": 200}` 但没有 `query`，会收到：

```
validation failed: query: required; limit: must be <= 100
```

---

## 核心概念

### Tool（工具）

AI 模型可以调用的函数。两种注册方式：

**简单 handler**——通过 Context 完全控制：

```go
s.Tool("hello", "打招呼", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    name := ctx.String("name")
    return ctx.Text(fmt.Sprintf("你好，%s！", name)), nil
})
```

**类型化 handler**——struct 输入 + 自动 schema（推荐）：

```go
type CreateUserInput struct {
    Name  string `json:"name"  mcp:"required,desc=用户姓名"`
    Email string `json:"email" mcp:"required,pattern=^[^@]+@[^@]+$"`
    Age   int    `json:"age"   mcp:"min=0,max=150"`
}

s.ToolFunc("create_user", "创建新用户", func(ctx *gomcp.Context, in CreateUserInput) (User, error) {
    return db.CreateUser(in.Name, in.Email, in.Age)
})
```

### Resource（资源）

向 AI 模型暴露数据（类似 GET 端点）：

```go
// 静态资源
s.Resource("config://app", "应用配置", func(ctx *gomcp.Context) (any, error) {
    return map[string]any{"version": "1.0", "env": "production"}, nil
})

// 动态资源（URI 模板）
s.ResourceTemplate("db://{table}/{id}", "数据库记录", func(ctx *gomcp.Context) (any, error) {
    table := ctx.String("table")
    id := ctx.String("id")
    return db.Find(table, id), nil
})
```

### Prompt（提示模板）

可复用的消息模板：

```go
s.Prompt("code_review", "代码审查助手",
    []gomcp.PromptArgument{
        gomcp.PromptArg("language", "编程语言", true),
        gomcp.PromptArg("focus", "审查重点", false),
    },
    func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
        lang := ctx.String("language")
        focus := ctx.String("focus")
        if focus == "" {
            focus = "bug、性能和安全"
        }
        return []gomcp.PromptMessage{
            gomcp.UserMsg(fmt.Sprintf("请审查以下 %s 代码，重点关注：%s", lang, focus)),
        }, nil
    },
)
```

---

## Struct Tag 参考

`mcp` tag 控制 schema 生成和参数校验：

| Tag | 类型 | 说明 | 示例 |
|-----|------|------|------|
| `required` | 标志 | 必填字段 | `mcp:"required"` |
| `desc` | 字符串 | 字段描述（展示给 AI） | `mcp:"desc=搜索关键词"` |
| `default` | 任意 | 默认值 | `mcp:"default=10"` |
| `min` | 数字 | 最小值（含） | `mcp:"min=0"` |
| `max` | 数字 | 最大值（含） | `mcp:"max=100"` |
| `enum` | 字符串 | 竖线分隔的枚举值 | `mcp:"enum=asc\|desc\|random"` |
| `pattern` | 字符串 | 正则校验 | `mcp:"pattern=^[a-z]+$"` |

组合使用：`mcp:"required,desc=用户邮箱,pattern=^[^@]+@[^@]+$"`

支持的 Go 类型：`string`、`int`、`float64`、`bool`、`[]T`、嵌套 struct。

---

## 中间件

GoMCP 使用 Gin 风格的中间件链，按顺序包裹 handler：

```
请求 → Logger → Recovery → Auth → RateLimit → Handler → RateLimit → Auth → Recovery → Logger → 响应
```

### 内置中间件

```go
s.Use(gomcp.Logger())                              // 记录工具名 + 耗时
s.Use(gomcp.Recovery())                            // panic 优雅恢复
s.Use(gomcp.RequestID())                           // 注入唯一请求 ID
s.Use(gomcp.Timeout(10 * time.Second))             // 超时控制
s.Use(gomcp.RateLimit(100))                        // 令牌桶限流：100 次/分钟
s.Use(gomcp.OpenTelemetry())                       // 自动追踪每次工具调用
```

### 认证中间件

```go
// 认证——验证身份
s.Use(gomcp.BearerAuth(func(token string) (*gomcp.AuthInfo, error) {
    claims, err := jwt.Verify(token)
    if err != nil {
        return nil, err
    }
    return &gomcp.AuthInfo{UserID: claims.Sub, Roles: claims.Roles}, nil
}))

// 授权——检查权限（应用到分组）
admin := s.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete_user", "删除用户", deleteHandler)
```

### 自定义中间件

```go
func AuditLog() gomcp.Middleware {
    return func(ctx *gomcp.Context, next func() error) error {
        start := time.Now()
        err := next()
        audit.Log(ctx.String("_tool_name"), time.Since(start), err)
        return err
    }
}
```

---

## 工具分组

按业务域组织工具，支持分组级中间件——类似 Gin 的 `RouterGroup`：

```go
s := gomcp.New("platform", "1.0.0")

// 用户工具——需要认证
user := s.Group("user", authMiddleware)
user.Tool("get", "获取用户信息", getUser)         // → user.get
user.Tool("update", "更新用户信息", updateUser)    // → user.update

// 管理员工具——需要 admin 角色
admin := user.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "删除用户", deleteUser)       // → user.admin.delete
```

---

## 适配器

**核心差异化功能。** 把现有服务变成 MCP 工具，无需重写任何代码。

### Gin 适配器

已有 Gin API？一行代码接入 MCP：

```go
ginRouter := setupYourExistingGinApp() // 你已有的 100+ 接口的 Gin 应用

s := gomcp.New("my-api", "1.0.0")
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v1/"},
    ExcludePaths: []string{"/api/v1/internal/"},
})
s.Stdio()
```

**自动转换规则：**
- `GET /api/v1/users` → Tool `get_api_v1_users`
- `GET /api/v1/users/:id` → Tool `get_api_v1_users_by_id`（id = 必填 string 参数）
- `POST /api/v1/users` → Tool `post_api_v1_users`（body = JSON 字符串参数）
- `DELETE /api/v1/users/:id` → Tool `delete_api_v1_users_by_id`

路径参数自动变为必填 Tool 参数。你原有的 Gin 中间件（认证、日志等）照常执行。

### OpenAPI 适配器

有 Swagger/OpenAPI 文档？自动生成 MCP 工具：

```go
s := gomcp.New("petstore", "1.0.0")
adapter.ImportOpenAPI(s, "./openapi.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"pets", "users"},  // 只导入这些 tag
    ServerURL: "https://api.example.com",
    AuthToken: os.Getenv("API_TOKEN"),
})
s.Stdio()
```

支持 OpenAPI 3.0/3.1、`$ref` 解析、`requestBody` schema。

### gRPC 适配器

```go
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService", "order.OrderService"},
})
```

---

## 组件版本化

同一工具注册多个版本，支持渐进式 API 演进：

```go
s.ToolFunc("search", "全文搜索", searchV1, gomcp.Version("1.0"))
s.ToolFunc("search", "语义搜索（向量）", searchV2, gomcp.Version("2.0"))

// 客户端调用：
//   "search"     → 最新版本（2.0）
//   "search@1.0" → 指定版本
```

标记废弃：

```go
s.Tool("old_search", "旧版搜索", handler, gomcp.Version("0.9"), gomcp.Deprecated("请使用 search@2.0"))
```

---

## 异步任务

适用于耗时较长的操作：

```go
s.AsyncTool("generate_report", "生成分析报告", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    data := fetchLargeDataset()
    report := analyze(data) // 可能需要几分钟
    return ctx.Text(report.Summary()), nil
})
```

客户端立即收到 task ID，然后通过 `tasks/get` 轮询状态，`tasks/cancel` 取消任务。

---

## 热加载

从 YAML 文件加载工具定义，文件变更自动重载：

```go
s.LoadDir("./tools/", gomcp.DirOptions{
    Watch:   true,
    Pattern: "*.tool.yaml",
})
```

```yaml
# tools/search.tool.yaml
name: search_user
description: 按名称搜索用户
version: "1.0"
params:
  - name: query
    type: string
    required: true
    description: 搜索关键词
handler: http://localhost:8080/api/users/search
method: GET
```

---

## MCP Inspector

内置 Web 调试界面：

```go
s.Dev(":9090") // 访问 http://localhost:9090
```

浏览所有注册的工具、资源、Prompt，在线调用并查看响应。

---

## 测试

`mcptest` 包提供内存级 MCP 客户端，无需启动传输层即可单元测试：

```go
func TestSearch(t *testing.T) {
    s := setupServer()
    c := mcptest.NewClient(t, s)
    c.Initialize()

    // 调用工具
    result := c.CallTool("search", map[string]any{"query": "golang", "limit": 5})
    result.AssertNoError(t)
    result.AssertContains(t, "golang")

    // 快照测试
    mcptest.MatchSnapshot(t, "search_result", result)

    // 测试校验
    bad := c.CallTool("search", map[string]any{"limit": 999})
    bad.AssertIsError(t)
    bad.AssertContains(t, "query: required")
}
```

---

## 传输层

```go
s.Stdio()          // stdin/stdout — Claude Desktop、Cursor、Kiro 等
s.HTTP(":8080")    // Streamable HTTP + SSE — 远程部署
s.Handler()        // http.Handler — 嵌入现有 HTTP 服务
```

### 配合 Claude Desktop / Cursor / Kiro 使用

在 MCP 客户端配置中添加：

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

---

## 项目结构

```
gomcp/
├── server.go              # Server 核心，工具/资源/Prompt 注册
├── context.go             # 请求上下文，类型化参数访问
├── group.go               # 工具分组
├── middleware.go           # 中间件接口和链式执行
├── middleware_builtin.go   # Logger、Recovery、RequestID、Timeout、RateLimit
├── middleware_auth.go      # BearerAuth、APIKeyAuth、BasicAuth、RBAC
├── middleware_otel.go      # OpenTelemetry 追踪
├── schema/                # struct tag → JSON Schema 生成器 + 校验器
├── transport/             # stdio + Streamable HTTP
├── adapter/               # Gin、OpenAPI、gRPC 适配器
├── mcptest/               # 测试工具包
├── task.go                # 异步任务
├── completion.go          # 自动补全
├── inspector.go           # Web 调试界面
├── provider.go            # YAML 热加载
└── examples/              # 可运行的示例
```

---

## 路线图

- [x] 核心：Tool、Resource、Prompt，完整 MCP 协议支持
- [x] struct tag 自动 schema 生成 + 参数校验
- [x] 中间件链（Logger、Recovery、RateLimit、Timeout、RequestID）
- [x] 认证中间件（Bearer / API Key / Basic）+ 角色/权限授权
- [x] 工具分组 + 嵌套分组
- [x] stdio + Streamable HTTP 传输（含 SSE 通知）
- [x] Gin 适配器——现有 Gin 路由一键转 MCP 工具
- [x] OpenAPI 适配器——从 Swagger 文档自动生成 MCP 工具
- [x] gRPC 适配器
- [x] OpenTelemetry 集成
- [x] mcptest 测试包 + 快照测试
- [x] 组件版本化 + 废弃标记
- [x] 异步任务（轮询 + 取消）
- [x] MCP Inspector Web 调试界面
- [x] YAML 热加载 Provider
- [x] Prompt/Resource 参数自动补全
- [ ] MCP Client 支持（Server 间调用）

---

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

[Apache 2.0](LICENSE)
