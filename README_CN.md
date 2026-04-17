# GoMCP

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-green.svg)](https://github.com/zhangpanda/gomcp/releases)

**用 Go 构建 MCP Server 的最快方式。**

[English](README.md)

---

## 🚀 快速链接

- **GitHub**: https://github.com/zhangpanda/gomcp
- **Gitee**: https://gitee.com/rilegouasas/gomcp
- **MCP 协议**: https://modelcontextprotocol.io

---

## 🎯 GoMCP 是什么？

GoMCP 是一个用于构建 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 服务端的 **Go 框架**——不只是 SDK。可以理解为 **"MCP 领域的 Gin"**。

MCP 是 Anthropic 发布的开放协议，让 AI 应用（Claude Desktop、Cursor、Kiro、VS Code Copilot）能够调用外部工具、读取数据源、使用 Prompt 模板。GoMCP 让构建这些服务变得极其简单。

### 为什么选 GoMCP？

| | mcp-go (mark3labs) | 官方 Go SDK | **GoMCP** |
|---|---|---|---|
| 定位 | SDK | SDK | **框架** |
| Schema 生成 | 手动 | `jsonschema` tag | **`mcp` tag + 自动校验** |
| 中间件 | 基础钩子 | 无 | **完整链（Logger、Auth、限流、OTel…）** |
| 工具分组 | 无 | 无 | **支持（`user.get`、`admin.delete`）** |
| 导入 Gin 路由 | 无 | 无 | **✅ 一行代码** |
| 导入 OpenAPI/Swagger | 无 | 无 | **✅ 一行代码** |
| 导入 gRPC 服务 | 无 | 无 | **✅** |
| 内置认证 | 无 | 无 | **Bearer、API Key、Basic + RBAC** |
| 调试界面 | 无 | 无 | **✅** |
| 测试工具 | 基础 | 无 | **mcptest 包** |

---

## 🛠️ 技术栈

### 环境要求

| 要求 | 版本 |
|------|------|
| **Go** | ≥ 1.25 |
| **MCP 协议** | 2024-11-05（向后兼容 2025-11-25） |

### 核心依赖

| 技术 | 说明 |
|------|------|
| **Go 标准库** | 核心框架——零外部依赖 |
| **Gin** | 仅适配器——导入现有 Gin 路由 |
| **gRPC** | 仅适配器——导入 gRPC 服务 |
| **OpenTelemetry** | 可选——分布式追踪 |
| **YAML v3** | 仅 Provider——热加载工具定义 |

---

## 🌟 核心功能

### 🔧 工具开发

- **Struct tag 自动 schema** — 用 Go 结构体和 `mcp` tag 定义参数，JSON Schema 自动生成
- **类型安全 handler** — `func(*Context, Input) (Output, error)` — 无需手动解析参数
- **参数校验** — required、min/max、enum、pattern — handler 执行前自动校验
- **组件版本化** — 同一工具注册多个版本，客户端通过 `name@version` 调用
- **异步任务** — 长时间运行的工具立即返回 task ID，支持轮询和取消

### 🔌 适配器（核心差异化）

- **Gin 适配器** — 一行代码将现有 Gin 路由导入为 MCP 工具
- **OpenAPI 适配器** — 从 Swagger/OpenAPI 3.x 文档自动生成工具
- **gRPC 适配器** — 将 gRPC 服务方法导入为 MCP 工具

### 🔐 安全

- **BearerAuth** — JWT Token 验证
- **APIKeyAuth** — 通过 Header 验证 API Key
- **BasicAuth** — HTTP Basic 认证
- **RequireRole / RequirePermission** — 基于角色/权限的授权控制

### 🧩 框架能力

- **中间件链** — Logger、Recovery、RequestID、Timeout、RateLimit、OpenTelemetry
- **工具分组** — 按前缀组织工具，支持分组级中间件
- **Resource & Prompt** — 完整 MCP 支持，包括 URI 模板和参数化 Prompt
- **自动补全** — 为 Prompt/Resource 参数提供补全建议

### 🚀 生产就绪

- **多传输层** — stdio（Claude Desktop、Cursor、Kiro）和 Streamable HTTP + SSE
- **MCP Inspector** — 内置 Web 调试界面，浏览和测试工具
- **热加载** — 从 YAML 文件加载工具定义，支持文件监听
- **mcptest 包** — 内存级测试客户端，支持快照测试

---

## 🏗️ 系统架构

```
┌──────────────────────────────────────────────────────────────┐
│                         用户代码                              │
│   s.Tool() / s.ToolFunc() / s.Resource() / s.Prompt()        │
├──────────────────────────────────────────────────────────────┤
│                        框架核心层                             │
│   路由 → 中间件链 → 参数校验 → Handler → 结果构建              │
├────────────┬─────────────┬───────────────┬───────────────────┤
│   Schema   │   校验引擎   │    适配器     │    可观测性        │
│   生成器    │  （自动）    │ Gin/OpenAPI/ │  OTel / Logger    │
│ (mcp tags) │             │ gRPC          │  / Inspector      │
├────────────┴─────────────┴───────────────┴───────────────────┤
│                        协议层                                │
│          JSON-RPC 2.0 / MCP 协议 / 能力协商                   │
├──────────────────────────────────────────────────────────────┤
│                        传输层                                │
│              stdio  /  Streamable HTTP + SSE                 │
└──────────────────────────────────────────────────────────────┘
```

### 项目结构

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

## 📦 安装

```bash
go get github.com/zhangpanda/gomcp
```

---

## ⚡ 快速开始

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

`SearchInput` 结构体 **自动生成** JSON Schema：

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

无效参数在 handler 执行前 **自动拒绝**：

```
validation failed: query: required; limit: must be <= 100
```

---

## 📖 使用指南

### Struct Tag 参考

| Tag | 类型 | 说明 | 示例 |
|-----|------|------|------|
| `required` | 标志 | 必填字段 | `mcp:"required"` |
| `desc` | 字符串 | 字段描述（展示给 AI） | `mcp:"desc=搜索关键词"` |
| `default` | 任意 | 默认值 | `mcp:"default=10"` |
| `min` | 数字 | 最小值（含） | `mcp:"min=0"` |
| `max` | 数字 | 最大值（含） | `mcp:"max=100"` |
| `enum` | 字符串 | 竖线分隔的枚举值 | `mcp:"enum=asc\|desc"` |
| `pattern` | 字符串 | 正则校验 | `mcp:"pattern=^[a-z]+$"` |

组合使用：`mcp:"required,desc=用户邮箱,pattern=^[^@]+@[^@]+$"`

### 工具注册

**简单 handler：**

```go
s.Tool("hello", "打招呼", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
    return ctx.Text("你好，" + ctx.String("name")), nil
})
```

**类型化 handler（推荐）：**

```go
type Input struct {
    Name  string `json:"name"  mcp:"required,desc=用户姓名"`
    Email string `json:"email" mcp:"required,pattern=^[^@]+@[^@]+$"`
}

s.ToolFunc("create_user", "创建用户", func(ctx *gomcp.Context, in Input) (User, error) {
    return db.CreateUser(in.Name, in.Email)
})
```

### 资源

```go
// 静态资源
s.Resource("config://app", "应用配置", func(ctx *gomcp.Context) (any, error) {
    return map[string]any{"version": "1.0"}, nil
})

// 动态 URI 模板
s.ResourceTemplate("db://{table}/{id}", "数据库记录", func(ctx *gomcp.Context) (any, error) {
    return db.Find(ctx.String("table"), ctx.String("id")), nil
})
```

### 中间件

```go
s.Use(gomcp.Logger())                              // 记录工具名 + 耗时
s.Use(gomcp.Recovery())                            // panic 恢复
s.Use(gomcp.RequestID())                           // 唯一请求 ID
s.Use(gomcp.Timeout(10 * time.Second))             // 超时控制
s.Use(gomcp.RateLimit(100))                        // 100 次/分钟限流
s.Use(gomcp.OpenTelemetry())                       // 分布式追踪
s.Use(gomcp.BearerAuth(tokenValidator))            // JWT 认证
```

### 工具分组

```go
user := s.Group("user", authMiddleware)
user.Tool("get", "获取用户", getUser)              // → user.get

admin := user.Group("admin", gomcp.RequireRole("admin"))
admin.Tool("delete", "删除用户", deleteUser)       // → user.admin.delete
```

### 适配器

**Gin——一行代码导入现有 API：**

```go
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v1/"},
})
// GET /api/v1/users/:id → Tool get_api_v1_users_by_id
```

**OpenAPI——从 Swagger 文档生成：**

```go
adapter.ImportOpenAPI(s, "./swagger.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"pets"},
    ServerURL: "https://api.example.com",
})
```

**gRPC：**

```go
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService"},
})
```

### 组件版本化

```go
s.ToolFunc("search", "v1", searchV1, gomcp.Version("1.0"))
s.ToolFunc("search", "v2 语义搜索", searchV2, gomcp.Version("2.0"))
// "search" → 最新版本，"search@1.0" → 指定版本
```

### 异步任务

```go
s.AsyncTool("report", "生成报告", handler)
// 客户端立即收到 taskId，通过 tasks/get 轮询，tasks/cancel 取消
```

### 热加载

```go
s.LoadDir("./tools/", gomcp.DirOptions{Watch: true})
```

### MCP Inspector

```go
s.Dev(":9090") // http://localhost:9090 — 浏览和测试所有工具
```

### 测试

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

### 传输层

```go
s.Stdio()          // Claude Desktop、Cursor、Kiro
s.HTTP(":8080")    // 远程部署，支持 SSE
s.Handler()        // 嵌入现有 HTTP 服务
```

### 配合 AI 客户端使用

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/your/binary"
    }
  }
}
```

支持 Claude Desktop、Cursor、Kiro、Windsurf、VS Code Copilot 及所有 MCP 兼容客户端。

---

## 📋 路线图

- [x] 核心：Tool、Resource、Prompt，完整 MCP 协议支持
- [x] struct tag 自动 schema 生成 + 参数校验
- [x] 中间件链（Logger、Recovery、RateLimit、Timeout、RequestID）
- [x] 认证中间件（Bearer / API Key / Basic）+ RBAC 授权
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

## 🤝 如何贡献

欢迎各种形式的贡献！

1. **Fork** 本仓库
2. **创建** 功能分支（`git checkout -b feature/amazing-feature`）
3. **提交** 更改（`git commit -m 'feat: add amazing feature'`）
4. **推送** 分支（`git push origin feature/amazing-feature`）
5. **发起** Pull Request

> 💡 推荐阅读：[《提问的智慧》](https://github.com/ryanhanwu/How-To-Ask-Questions-The-Smart-Way) 和 [《如何向开源社区提问题》](https://github.com/seajs/seajs/issues/545)

---

## ⚖️ 版权与许可

Copyright © 2026 [istarshine](https://gitee.com/rilegouasas)

本项目基于 [Apache License 2.0](LICENSE) 开源。

### 重要说明

1. 本项目 **开源免费**，个人和商业使用均遵循 Apache 2.0 协议。
2. 使用本项目时，**必须保留** 版权声明和许可证文本。
3. Apache 2.0 协议包含 **明确的专利授权**——贡献者向用户授予免费的专利使用许可。
4. 如果您在商业产品中使用本项目，欢迎（但不强制）在文档中注明使用了 GoMCP。
5. 对本项目的贡献同样遵循 Apache 2.0 协议。

### 专利声明

本框架的部分功能（struct tag schema 生成、HTTP 到 MCP 自动适配、OpenAPI 到 MCP 自动适配）已提交专利申请。Apache 2.0 协议授予您永久的、全球范围的、免版税的专利许可，允许您在使用本软件时使用这些功能。

---

## ⭐ Star

如果 GoMCP 对你有帮助，请给个 Star！这能帮助更多人发现这个项目。

---

## 📬 联系方式

- **Issues**: [GitHub Issues](https://github.com/zhangpanda/gomcp/issues)
- **Discussions**: [GitHub Discussions](https://github.com/zhangpanda/gomcp/discussions)
