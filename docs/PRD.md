# GoMCP — Go MCP Server 开发框架 产品需求文档（PRD）

> 版本：v1.0 | 日期：2026-04-16 | 作者：istarshine 团队

---

## 目录

1. [项目概述](#1-项目概述)
2. [市场分析与竞品调研](#2-市场分析与竞品调研)
3. [目标用户与使用场景](#3-目标用户与使用场景)
4. [产品定位与核心理念](#4-产品定位与核心理念)
5. [功能需求](#5-功能需求)
6. [非功能需求](#6-非功能需求)
7. [技术架构](#7-技术架构)
8. [API 设计规范](#8-api-设计规范)
9. [开发路线图](#9-开发路线图)
10. [专利规划](#10-专利规划)
11. [商业模式](#11-商业模式)
12. [风险评估](#12-风险评估)
13. [成功指标](#13-成功指标)

---

## 1. 项目概述

### 1.1 项目名称

**GoMCP** — The Fast, Idiomatic Way to Build MCP Servers in Go

### 1.2 一句话描述

一个类似 Gin 风格的 Go 框架，让开发者用最少的代码构建生产级 MCP Server，并能一键将现有 HTTP/gRPC 服务转换为 MCP 工具。

### 1.3 背景

Model Context Protocol（MCP）是 Anthropic 于 2024 年底发布的开放协议，定义了 AI 应用与外部数据源/工具之间的标准通信方式。截至 2026 年 4 月：

- Claude Desktop、Cursor、Kiro、Windsurf、VS Code Copilot 等主流 AI 客户端均已支持 MCP
- MCP 协议已迭代至 2025-11-25 版本，支持 Tool、Resource、Prompt、Sampling、Tasks 等能力
- Python 生态有 FastMCP 3.0（装饰器注册、版本化、授权、OpenTelemetry）
- TypeScript 有官方 SDK
- **Go 生态缺少一个真正好用的"框架级"解决方案**

现有 Go 实现（mcp-go、官方 go-sdk）都是 SDK 级别，提供协议实现但不提供框架级开发体验。开发者仍需手动构建 schema、处理参数校验、编写大量样板代码。

### 1.4 核心问题

| 问题 | 影响 |
|------|------|
| Go MCP 开发体验差 | 构建一个 Tool 需要 30+ 行样板代码，schema 手动定义易出错 |
| 现有服务接入 MCP 成本高 | 已有 Gin/gRPC 项目要暴露为 MCP Tool，需要完全重写接口层 |
| 缺少框架级能力 | 没有中间件、分组、认证、限流等生产必需功能 |
| 生态碎片化 | 多个 Go SDK 各自为政，没有统一的最佳实践 |

### 1.5 解决方案

GoMCP 提供：

1. **极简 API** — struct tag 自动生成 JSON Schema，一个函数即一个 Tool
2. **自动适配器** — 现有 Gin/gRPC/OpenAPI 服务一行代码转 MCP（核心差异化）
3. **框架级能力** — 中间件链、Tool 分组、认证授权、限流、可观测性
4. **生产就绪** — 多传输层支持、Session 管理、优雅关闭、健康检查

---

## 2. 市场分析与竞品调研

### 2.1 市场规模

- Go 开发者全球约 300-400 万（TIOBE 2026 排名 Top 10）
- 企业后端服务中 Go 占比持续增长（字节跳动、腾讯、B站、七牛、PingCAP 等大量使用）
- MCP 协议生态爆发式增长，mcp.so 已收录 10,000+ MCP Server
- Go 语言在 AI 基础设施层（非模型层）有天然优势：编译为单二进制、低内存、高并发

### 2.2 竞品详细分析

#### 2.2.1 mcp-go（mark3labs）— 社区最流行

| 维度 | 详情 |
|------|------|
| GitHub | 8.6k star, 814 fork, 488 commits |
| 版本 | v0.48.0（仍为 0.x，API 不稳定） |
| 许可证 | MIT |
| 协议支持 | MCP 2025-11-25（含向后兼容） |

**功能清单：**
- ✅ Tool / Resource / Prompt 完整支持
- ✅ stdio / SSE / Streamable HTTP 传输
- ✅ Session 管理（Per-Session Tools、Tool Filtering）
- ✅ Request Hooks（生命周期钩子）
- ✅ Tool Handler Middleware
- ✅ Task-augmented Tools（异步任务）
- ✅ Auto-completions（参数自动补全）
- ✅ mcptest 测试工具包
- ❌ 无 struct tag 自动 schema 生成（需手动 `mcp.WithString("name", ...)`）
- ❌ 无 Gin/gRPC/OpenAPI 自动适配器
- ❌ 无 Tool 分组（类似 Gin RouterGroup）
- ❌ 无内置认证/授权框架
- ❌ 无内置限流/熔断
- ❌ 无 OpenTelemetry 集成
- ❌ 无组件版本化

**API 风格示例（mcp-go）：**
```go
// 需要手动构建 schema，代码冗长
tool := mcp.NewTool("calculate",
    mcp.WithDescription("Perform arithmetic"),
    mcp.WithString("operation",
        mcp.Required(),
        mcp.Description("The operation"),
        mcp.Enum("add", "subtract", "multiply", "divide"),
    ),
    mcp.WithNumber("x", mcp.Required(), mcp.Description("First number")),
    mcp.WithNumber("y", mcp.Required(), mcp.Description("Second number")),
)
s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    op, _ := req.RequireString("operation")
    x, _ := req.RequireFloat("x")
    y, _ := req.RequireFloat("y")
    // ... 业务逻辑
})
```

#### 2.2.2 官方 Go SDK（modelcontextprotocol/go-sdk）— Google 协助维护

| 维度 | 详情 |
|------|------|
| GitHub | 4.4k star, 406 fork |
| 版本 | v1.5.0（已稳定） |
| 许可证 | Apache 2.0 + MIT |
| 维护方 | MCP 官方 + Google Go 团队 |

**功能清单：**
- ✅ 完整协议实现（MCP 2025-11-25）
- ✅ struct 输入输出（`jsonschema` tag 自动生成 schema）
- ✅ OAuth 支持（实验性）
- ✅ Client + Server 双端
- ❌ 纯 SDK，不是框架
- ❌ 无中间件系统
- ❌ 无自动适配器
- ❌ 无分组/路由概念
- ❌ 无内置认证/限流
- ❌ 无可观测性集成

**API 风格示例（官方 SDK）：**
```go
type Input struct {
    Name string `json:"name" jsonschema:"the name of the person"`
}
type Output struct {
    Greeting string `json:"greeting"`
}
func SayHi(ctx context.Context, req *mcp.CallToolRequest, input Input) (
    *mcp.CallToolResult, Output, error,
) {
    return nil, Output{Greeting: "Hi " + input.Name}, nil
}
// 注册
mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "say hi"}, SayHi)
```

#### 2.2.3 FastMCP 3.0（Python）— 我们的对标物

| 维度 | 详情 |
|------|------|
| 版本 | 3.0（2026-01-19 发布） |
| 定位 | Python MCP 开发框架（非 SDK） |
| 特色 | 装饰器注册、极简 API |

**FastMCP 3.0 功能集（GoMCP 需对标）：**
- ✅ 装饰器注册 Tool/Resource/Prompt（`@mcp.tool()`）
- ✅ 组件版本化（`@mcp.tool(version="2.0")`）
- ✅ 粒度授权（`@mcp.tool(auth=lambda ctx: ...)`）
- ✅ OpenTelemetry 原生集成
- ✅ 多 Provider 类型（FileSystem 热加载、Skills、OpenAPI）
- ✅ MCP Inspector 调试工具
- ✅ 自动依赖管理
- ✅ 内置参数校验
- ✅ 缓存支持

#### 2.2.4 其他 Go 实现

| 项目 | Star | 状态 | 备注 |
|------|------|------|------|
| mcp-golang (metoro-io) | ~500 | 活跃 | 类型安全，但功能有限 |
| go-mcp (ThinkInAIXYZ) | ~300 | 活跃 | 基础 SDK 实现 |
| Foxy Contexts | ~100 | 低活跃 | 实验性质 |
| Higress MCP (Go/Envoy) | ~200 | 活跃 | 网关层 MCP，非通用框架 |

### 2.3 竞争差异化矩阵

| 能力 | mcp-go | 官方 SDK | FastMCP(Py) | **GoMCP（我们）** |
|------|--------|----------|-------------|-------------------|
| struct 自动 schema | ❌ | ✅ | ✅ | ✅ |
| 中间件链 | ⚠️ 基础 | ❌ | ❌ | ✅ 完整 |
| Tool 分组 | ❌ | ❌ | ❌ | ✅ |
| Gin 适配器 | ❌ | ❌ | N/A | ✅ |
| gRPC 适配器 | ❌ | ❌ | N/A | ✅ |
| OpenAPI 适配器 | ❌ | ❌ | ✅ | ✅ |
| 组件版本化 | ❌ | ❌ | ✅ | ✅ |
| 粒度授权 | ❌ | ❌ | ✅ | ✅ |
| OpenTelemetry | ❌ | ❌ | ✅ | ✅ |
| 内置限流 | ❌ | ❌ | ❌ | ✅ |
| 热加载 | ❌ | ❌ | ✅ | ✅ |
| 测试工具 | ✅ | ❌ | ✅ | ✅ |
| Inspector 调试 | ❌ | ❌ | ✅ | ✅ |

---

## 3. 目标用户与使用场景

### 3.1 用户画像

#### P0 — Go 后端开发者（核心用户）

- **特征**：日常使用 Go 开发 Web 服务，熟悉 Gin/Echo/Chi 等框架
- **痛点**：想把现有服务接入 AI 生态，但现有 MCP SDK 学习成本高、样板代码多
- **期望**：像写 Gin handler 一样写 MCP Tool，零学习成本
- **规模**：全球 300-400 万 Go 开发者

#### P1 — 企业技术团队

- **特征**：有大量 Go 微服务，需要统一接入 AI 助手
- **痛点**：每个服务单独接入 MCP 成本高，缺少统一的认证/授权/监控方案
- **期望**：一套框架解决所有服务的 MCP 接入，企业级安全和可观测性
- **规模**：使用 Go 的中大型企业（字节、腾讯、B站、美团等）

#### P2 — SaaS 产品团队

- **特征**：做 SaaS 产品，想让用户通过 AI 操作产品功能
- **痛点**：需要同时维护 REST API 和 MCP 接口，工作量翻倍
- **期望**：从 OpenAPI 文档自动生成 MCP Server，零额外开发
- **规模**：中小型 SaaS 公司

#### P3 — 独立开发者 / 开源贡献者

- **特征**：想快速做一个 MCP 插件发布到社区
- **痛点**：现有工具链不够简洁，从零开始太慢
- **期望**：5 分钟内跑通一个 MCP Server
- **规模**：开源社区活跃开发者

### 3.2 核心使用场景

#### 场景 1：从零构建 MCP Server

**用户故事**：作为 Go 开发者，我想快速构建一个 MCP Server，暴露几个工具给 AI 使用。

```go
package main

import "github.com/istarshine/gomcp"

type SearchInput struct {
    Query string `json:"query" mcp:"required,desc=搜索关键词"`
    Limit int    `json:"limit" mcp:"default=10,min=1,max=100"`
}

func main() {
    s := gomcp.New("search-server", "1.0.0")
    s.Tool("search", "搜索文档", func(ctx *gomcp.Context, in SearchInput) ([]Doc, error) {
        return db.Search(in.Query, in.Limit)
    })
    s.Stdio()
}
```

**验收标准**：
- 5 行核心代码即可定义一个完整的 Tool
- struct tag 自动生成 JSON Schema（含 required、default、min/max、desc）
- 自动参数校验，校验失败返回标准 MCP 错误

#### 场景 2：现有 Gin 项目接入 MCP

**用户故事**：作为企业开发者，我有一个 100+ 接口的 Gin 项目，想让 AI 能调用这些接口。

```go
ginRouter := setupExistingGinRouter() // 已有的 Gin 路由

s := gomcp.New("my-api", "1.0.0")
s.ImportGin(ginRouter, gomcp.ImportOptions{
    IncludePaths: []string{"/api/v1/users/*", "/api/v1/orders/*"},
    ExcludePaths: []string{"/api/v1/internal/*"},
    AuthHeader:   "X-API-Key",
})
s.Stdio()
```

**验收标准**：
- 自动从 Gin 路由提取 path/query/body 参数生成 MCP Tool schema
- 支持路径过滤（include/exclude）
- 保留原有中间件链（认证、日志等）
- 自动生成 Tool 名称和描述（从路由路径和注释推导）

#### 场景 3：从 OpenAPI 文档生成 MCP Server

**用户故事**：作为 SaaS 产品团队，我有 Swagger/OpenAPI 文档，想自动生成 MCP Server。

```go
s := gomcp.New("petstore", "1.0.0")
s.ImportOpenAPI("./openapi.yaml", gomcp.OpenAPIOptions{
    TagFilter:    []string{"pets", "users"},
    ServerURL:    "https://api.example.com",
    AuthToken:    os.Getenv("API_TOKEN"),
})
s.HTTP(":8080")
```

**验收标准**：
- 解析 OpenAPI 3.0/3.1 文档
- 每个 operation 自动转为一个 MCP Tool
- 参数 schema 直接复用 OpenAPI 定义
- 支持按 tag 过滤
- 自动处理认证头

#### 场景 4：企业级多服务 MCP 网关

**用户故事**：作为平台团队，我需要把多个微服务统一暴露为一个 MCP Server，带认证和监控。

```go
s := gomcp.New("platform-gateway", "1.0.0")

// 中间件
s.Use(gomcp.Logger())
s.Use(gomcp.Recovery())
s.Use(gomcp.OpenTelemetry(otelProvider))
s.Use(gomcp.RateLimit(1000)) // 全局限流

// 分组：用户服务
userGroup := s.Group("user", gomcp.BearerAuth(jwtValidator))
userGroup.Tool("get_user", "获取用户信息", handlers.GetUser)
userGroup.Tool("update_user", "更新用户信息", handlers.UpdateUser)

// 分组：订单服务
orderGroup := s.Group("order", gomcp.APIKeyAuth(keyStore))
orderGroup.Tool("list_orders", "查询订单列表", handlers.ListOrders)
orderGroup.Tool("create_order", "创建订单", handlers.CreateOrder)

// 分组：运维（仅管理员）
adminGroup := s.Group("admin", gomcp.RoleAuth("admin"))
adminGroup.Tool("restart_pod", "重启 K8s Pod", handlers.RestartPod)
adminGroup.Tool("scale_deploy", "扩缩容", handlers.ScaleDeploy)

s.HTTP(":8080")
```

**验收标准**：
- 中间件链按顺序执行，支持全局和分组级别
- 分组自动为 Tool 名称添加前缀（如 `user.get_user`）
- 认证失败返回标准 MCP 错误
- OpenTelemetry 自动追踪每个 Tool 调用
- 限流超限返回友好错误信息

#### 场景 5：异步长任务

**用户故事**：作为数据团队，我有一些耗时较长的数据处理任务（几分钟），需要异步执行。

```go
s.AsyncTool("generate_report", "生成数据报告",
    gomcp.TaskRequired(),
    gomcp.MaxConcurrent(5),
    func(ctx *gomcp.Context, in ReportInput) (*Report, error) {
        // 长时间运行的任务
        data := fetchData(in.DateRange)
        report := analyze(data)
        return report, nil
    },
)
```

**验收标准**：
- 客户端调用后立即返回 Task ID
- 支持轮询查询任务状态
- 支持取消正在运行的任务
- 支持并发任务数限制

---

## 4. 产品定位与核心理念

### 4.1 定位

> GoMCP 是 Go 语言的 MCP Server 开发框架，不是 SDK。
> 
> SDK 提供协议实现，框架提供开发体验。

类比：
- mcp-go / 官方 go-sdk = `net/http`（标准库级别）
- **GoMCP = Gin**（框架级别）

### 4.2 设计原则

| 原则 | 说明 |
|------|------|
| **Convention over Configuration** | 合理的默认值，零配置即可运行 |
| **Idiomatic Go** | 遵循 Go 社区惯例，struct tag、interface、error handling |
| **Progressive Disclosure** | 简单场景极简，复杂场景可扩展，不强制用户理解全部概念 |
| **Production First** | 不是玩具，内置生产环境所需的一切 |
| **Adapter Pattern** | 不重复造轮子，拥抱现有 Go 生态（Gin、gRPC、OpenAPI） |

### 4.3 技术选型

| 决策 | 选择 | 理由 |
|------|------|------|
| 底层协议实现 | 基于官方 go-sdk 封装 | 协议兼容性有保障，Google 维护 |
| Schema 生成 | 自研 struct tag 解析 | 官方 SDK 的 `jsonschema` tag 不够灵活 |
| HTTP 框架 | 不依赖特定框架 | 适配器模式，支持 Gin/Echo/Chi/标准库 |
| 序列化 | encoding/json + jsoniter 可选 | 兼容标准库，高性能场景可切换 |
| 日志 | slog（Go 1.21+ 标准库） | 无外部依赖，结构化日志 |
| 测试 | 标准 testing + 自研 mcptest | 无需额外测试框架 |

---

## 5. 功能需求

### 5.1 功能优先级定义

- **P0（MVP 必须）**：没有这些功能框架无法使用
- **P1（核心竞争力）**：差异化功能，第一个版本应包含
- **P2（增强功能）**：提升体验，可在后续版本迭代
- **P3（远期规划）**：长期愿景

### 5.2 P0 — MVP 核心功能

#### F-001 Server 生命周期管理

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 创建、配置、启动、优雅关闭 MCP Server |

**详细需求：**
- `gomcp.New(name, version, ...options)` 创建 Server 实例
- 支持 Option 模式配置（`WithDescription()`, `WithCapabilities()` 等）
- `s.Stdio()` 启动 stdio 传输
- `s.HTTP(addr)` 启动 Streamable HTTP 传输
- `s.SSE(addr)` 启动 SSE 传输（向后兼容）
- 优雅关闭：捕获 SIGINT/SIGTERM，等待进行中的请求完成
- 健康检查端点（HTTP 模式）

#### F-002 Tool 注册与执行

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 注册 Tool 并处理调用请求 |

**详细需求：**

方式一：函数式注册（最简）
```go
s.Tool("name", "description", handlerFunc)
```

方式二：带参数定义
```go
s.Tool("name", "description",
    gomcp.Param("query", "string", "搜索词", true),
    gomcp.Param("limit", "int", "数量", false),
    handlerFunc,
)
```

方式三：struct 自动推导（推荐）
```go
type Input struct {
    Query string `json:"query" mcp:"required,desc=搜索词"`
    Limit int    `json:"limit" mcp:"default=10,min=1,max=100"`
}
s.Tool("search", "搜索", func(ctx *gomcp.Context, in Input) (Result, error) {
    // ...
})
```

**struct tag `mcp` 支持的指令：**

| 指令 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `required` | bool | 必填参数 | `mcp:"required"` |
| `desc` | string | 参数描述 | `mcp:"desc=用户名"` |
| `default` | any | 默认值 | `mcp:"default=10"` |
| `min` | number | 最小值 | `mcp:"min=0"` |
| `max` | number | 最大值 | `mcp:"max=100"` |
| `enum` | string | 枚举值 | `mcp:"enum=asc|desc"` |
| `pattern` | string | 正则校验 | `mcp:"pattern=^[a-z]+$"` |
| `example` | any | 示例值 | `mcp:"example=hello"` |
| `-` | - | 忽略该字段 | `mcp:"-"` |

**返回值处理：**
- 返回 `string` → 自动包装为 TextContent
- 返回 `struct/map` → 自动 JSON 序列化为 TextContent
- 返回 `[]byte` + MIME type → BlobContent
- 返回 `*gomcp.Result` → 完全自定义（多内容、图片等）
- 返回 `error` → 自动转为 MCP 错误响应

#### F-003 Resource 注册与读取

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 注册静态和动态 Resource |

**详细需求：**
```go
// 静态 Resource
s.Resource("config://app", "应用配置", func(ctx *gomcp.Context) (any, error) {
    return loadConfig(), nil
})

// 动态 Resource（URI 模板）
s.ResourceTemplate("db://{table}/{id}", "数据库记录",
    func(ctx *gomcp.Context) (any, error) {
        table := ctx.Param("table")
        id := ctx.Param("id")
        return db.Find(table, id), nil
    },
)
```

- 支持 `text/*` 和 `application/*` MIME 类型
- 支持 Resource 列表变更通知
- 支持 Resource 订阅

#### F-004 Prompt 注册

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 注册可复用的 Prompt 模板 |

```go
s.Prompt("code_review", "代码审查",
    gomcp.PromptArg("language", "编程语言", true),
    gomcp.PromptArg("style", "审查风格", false),
    func(ctx *gomcp.Context) ([]gomcp.Message, error) {
        lang := ctx.String("language")
        return []gomcp.Message{
            gomcp.UserMsg("请审查以下 %s 代码，关注安全和性能", lang),
        }, nil
    },
)
```

#### F-005 Context 对象

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 请求上下文，贯穿整个处理链 |

**Context 提供的能力：**
```go
type Context struct {
    // 参数获取
    String(key string) string
    Int(key string) int
    Float(key string) float64
    Bool(key string) bool
    Param(key string) string        // URI 模板参数
    Bind(v any) error               // 绑定到 struct

    // 结果构建
    Text(s string) *Result
    JSON(v any) *Result
    Blob(data []byte, mime string) *Result
    Image(data []byte, mime string) *Result
    Error(msg string) *Result

    // 上下文信息
    Session() Session               // 当前会话
    RequestID() string              // 请求 ID
    Logger() *slog.Logger           // 带请求上下文的 logger

    // 标准库兼容
    Context() context.Context       // 底层 context.Context
    Set(key string, value any)      // 存储自定义数据
    Get(key string) (any, bool)     // 获取自定义数据
}
```

#### F-006 传输层

| 项目 | 说明 |
|------|------|
| 优先级 | P0 |
| 描述 | 支持 MCP 协议定义的所有传输方式 |

- **stdio**：标准输入输出，用于本地 MCP 客户端（Claude Desktop、Cursor 等）
- **Streamable HTTP**：HTTP + SSE，用于远程部署（MCP 2025-11-25 推荐）
- **SSE**（向后兼容）：旧版 SSE 传输

### 5.3 P1 — 核心竞争力功能

#### F-007 中间件系统

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 类似 Gin 的中间件链，支持全局和分组级别 |

```go
// 中间件签名
type Middleware func(ctx *Context, next func() error) error

// 内置中间件
gomcp.Logger()           // 请求日志
gomcp.Recovery()         // panic 恢复
gomcp.RateLimit(n)       // 限流（每分钟 n 次）
gomcp.Timeout(d)         // 超时控制
gomcp.CORS()             // 跨域（HTTP 模式）
gomcp.RequestID()        // 请求 ID 注入
```

**执行顺序：**
```
全局中间件1 → 全局中间件2 → 分组中间件1 → Handler → 分组中间件1(after) → 全局中间件2(after) → 全局中间件1(after)
```

#### F-008 Tool 分组

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 按业务域对 Tool 进行分组，支持分组级中间件 |

```go
userGroup := s.Group("user")
userGroup.Use(authMiddleware)
userGroup.Tool("get", "获取用户", ...)    // Tool 名称: user.get
userGroup.Tool("update", "更新用户", ...) // Tool 名称: user.update

// 嵌套分组
adminGroup := userGroup.Group("admin")
adminGroup.Use(adminAuthMiddleware)
adminGroup.Tool("delete", "删除用户", ...) // Tool 名称: user.admin.delete
```

#### F-009 Gin 自动适配器

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 将现有 Gin 路由自动转换为 MCP Tool |

```go
s.ImportGin(ginRouter, gomcp.ImportOptions{
    IncludePaths: []string{"/api/v1/*"},
    ExcludePaths: []string{"/api/v1/internal/*"},
    NamingRule:   gomcp.PathToSnakeCase, // GET /api/v1/users/:id → get_api_v1_users_by_id
    AuthHeader:   "Authorization",
})
```

**转换规则：**
| Gin 元素 | MCP 映射 |
|----------|----------|
| 路由路径 + HTTP 方法 | Tool 名称 |
| Path 参数 (`:id`) | Tool 参数（required string） |
| Query 参数 | Tool 参数（optional） |
| Request Body（JSON） | Tool 参数（从 struct 推导） |
| Response Body | Tool 返回值 |
| Gin 中间件 | 保留执行 |

#### F-010 OpenAPI 自动适配器

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 从 OpenAPI 3.0/3.1 文档自动生成 MCP Tool |

```go
s.ImportOpenAPI("./swagger.yaml", gomcp.OpenAPIOptions{
    TagFilter:  []string{"pets", "users"},
    ServerURL:  "https://api.example.com",
    AuthToken:  os.Getenv("API_TOKEN"),
    NamingRule: gomcp.OperationIDNaming, // 使用 operationId 作为 Tool 名称
})
```

**转换规则：**
| OpenAPI 元素 | MCP 映射 |
|-------------|----------|
| operationId | Tool 名称 |
| summary | Tool 描述 |
| parameters | Tool 参数 |
| requestBody schema | Tool 参数（嵌套） |
| responses.200.schema | Tool 返回值 schema |

#### F-011 认证与授权

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 内置多种认证方式，支持 Tool 级别授权 |

```go
// 认证中间件
gomcp.BearerAuth(jwtValidator)    // JWT Bearer Token
gomcp.APIKeyAuth(keyStore)        // API Key
gomcp.BasicAuth(userStore)        // Basic Auth
gomcp.OAuth2(oauthConfig)         // OAuth 2.0

// Tool 级别授权
s.Tool("delete_user", "删除用户",
    gomcp.RequireRole("admin"),
    gomcp.RequirePermission("user:delete"),
    handler,
)
```

#### F-012 参数自动校验

| 项目 | 说明 |
|------|------|
| 优先级 | P1 |
| 描述 | 基于 struct tag 自动校验参数，校验失败返回标准 MCP 错误 |

- required 校验
- 类型校验（string/int/float/bool/array/object）
- 范围校验（min/max）
- 枚举校验（enum）
- 正则校验（pattern）
- 自定义校验器（`gomcp.RegisterValidator(name, func)`)

### 5.4 P2 — 增强功能

#### F-013 OpenTelemetry 集成

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 原生 OpenTelemetry 支持，自动追踪 Tool 调用链 |

- 每个 Tool 调用自动创建 Span
- 记录参数、返回值、耗时、错误
- 支持 Trace、Metrics、Logs 三大信号
- 兼容 Jaeger、Zipkin、Prometheus、Grafana

#### F-014 组件版本化

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | Tool/Resource/Prompt 支持版本管理 |

```go
s.Tool("search", "搜索 v1",
    gomcp.Version("1.0"),
    searchV1Handler,
)
s.Tool("search", "搜索 v2（支持语义搜索）",
    gomcp.Version("2.0"),
    searchV2Handler,
)
```

- 客户端可指定版本调用
- 默认使用最新版本
- 支持版本废弃标记

#### F-015 异步任务（Task-augmented Tools）

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 支持 MCP Tasks 规范的异步工具执行 |

- TaskSupportRequired / TaskSupportOptional 模式
- 任务创建、查询、取消
- 并发任务数限制
- 任务状态通知

#### F-016 自动补全（Completions）

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 为 Prompt 参数和 Resource URI 提供自动补全建议 |

```go
s.Completion("prompt", "code_review", "language",
    func(ctx *gomcp.Context, partial string) ([]string, error) {
        languages := []string{"go", "python", "typescript", "rust", "java"}
        return filterByPrefix(languages, partial), nil
    },
)
```

#### F-017 热加载 Provider

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 从文件系统目录动态加载 Tool 定义，支持热更新 |

```go
s.LoadDir("./tools/", gomcp.DirOptions{
    Watch:    true,           // 监听文件变化
    Pattern:  "*.tool.yaml",  // 匹配模式
    OnReload: func() { log.Println("tools reloaded") },
})
```

YAML Tool 定义格式：
```yaml
name: search_user
description: 搜索用户
version: "1.0"
params:
  - name: query
    type: string
    required: true
    description: 搜索关键词
handler: http://localhost:8080/api/users/search
method: GET
```

#### F-018 gRPC 自动适配器

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 从 gRPC Service 定义自动生成 MCP Tool |

```go
s.ImportGRPC(grpcConn, gomcp.GRPCOptions{
    Services:  []string{"user.UserService", "order.OrderService"},
    ProtoPath: "./proto/",
})
```

#### F-019 内置测试工具

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 提供测试辅助包，简化 MCP Server 单元测试 |

```go
func TestSearchTool(t *testing.T) {
    s := setupServer()
    client := gomcptest.NewClient(t, s)

    result, err := client.CallTool("search", map[string]any{
        "query": "golang",
        "limit": 5,
    })
    assert.NoError(t, err)
    assert.Contains(t, result.Text(), "golang")

    // 快照测试
    gomcptest.MatchSnapshot(t, "search_result", result)
}
```

#### F-020 MCP Inspector（Web 调试界面）

| 项目 | 说明 |
|------|------|
| 优先级 | P2 |
| 描述 | 内置 Web 调试界面，可视化测试 Tool/Resource/Prompt |

```go
// 开发模式启动，自动开启 Inspector
s.Dev(":9090") // 访问 http://localhost:9090
```

功能：
- 列出所有 Tool/Resource/Prompt 及其 schema
- 在线调用 Tool，查看请求/响应
- 查看 Session 列表和状态
- 实时日志流

### 5.5 P3 — 远期规划

| 编号 | 功能 | 说明 |
|------|------|------|
| F-021 | MCP Client 支持 | 框架内置 MCP Client，支持 Server-to-Server 调用 |
| F-022 | 插件市场 CLI | `gomcp install mysql` 安装社区 Tool 插件 |
| F-023 | 代码生成器 | `gomcp gen --proto user.proto` 从 Proto 生成 MCP Server 代码 |
| F-024 | Sampling 支持 | 支持 MCP Sampling 能力（Server 请求 Client 的 LLM） |
| F-025 | 多语言 Gateway | 作为 MCP 网关代理非 Go 服务 |

---

## 6. 非功能需求

### 6.1 性能

| 指标 | 目标 |
|------|------|
| Tool 调用延迟（框架开销） | < 0.1ms（不含业务逻辑） |
| 并发连接数（HTTP 模式） | 10,000+ |
| 内存占用（空 Server） | < 10MB |
| 启动时间 | < 100ms |
| Schema 生成（100 个 Tool） | < 10ms |

### 6.2 可靠性

- panic 自动恢复（Recovery 中间件）
- 优雅关闭（等待进行中请求完成，超时强制退出）
- 连接断开自动检测和清理
- 内存泄漏防护（Session 超时清理）

### 6.3 安全性

- 输入参数自动校验和清洗
- 路径遍历防护（Resource URI）
- 请求大小限制（防止 OOM）
- 敏感信息不出现在日志中（自动脱敏）
- 支持 TLS（HTTP 模式）

### 6.4 可观测性

- 结构化日志（slog 兼容）
- OpenTelemetry Trace/Metrics/Logs
- 内置 Prometheus metrics 端点（HTTP 模式）
- 请求 ID 全链路追踪

### 6.5 兼容性

- Go 1.22+（利用新路由增强特性）
- MCP 协议 2025-11-25（向后兼容 2025-06-18、2025-03-26、2024-11-05）
- 与 mcp-go、官方 go-sdk 的 Handler 可互操作

### 6.6 开发体验

- 完善的 GoDoc 文档
- 每个功能配套 example
- 错误信息友好且可操作（告诉用户怎么修）
- IDE 友好（类型安全，自动补全友好）

---

## 7. 技术架构

### 7.1 分层架构

```
┌─────────────────────────────────────────────────────┐
│                   用户代码层                          │
│  s.Tool() / s.Resource() / s.Prompt() / s.Group()   │
├─────────────────────────────────────────────────────┤
│                   框架核心层                          │
│  Router → Middleware Chain → Handler → Result Builder │
├──────────┬──────────┬───────────┬───────────────────┤
│  Schema  │ Validator│  Adapter  │   Observability   │
│ Generator│  Engine  │  Layer    │   (OTel/Metrics)  │
├──────────┴──────────┴───────────┴───────────────────┤
│                   协议层                             │
│  JSON-RPC 2.0 / MCP Protocol / Capability Negotiation│
├─────────────────────────────────────────────────────┤
│                   传输层                             │
│         stdio  /  Streamable HTTP  /  SSE           │
└─────────────────────────────────────────────────────┘
```

### 7.2 模块划分

```
gomcp/
├── gomcp.go              # 公共 API 入口（New, Tool, Resource, Prompt...）
├── server.go             # Server 核心实现
├── context.go            # Context 对象
├── router.go             # Tool/Resource/Prompt 路由注册
├── group.go              # Tool 分组
├── middleware.go          # 中间件接口和链式执行
├── schema/
│   ├── generator.go      # struct tag → JSON Schema 生成器
│   ├── tags.go           # mcp tag 解析
│   └── validator.go      # 参数校验引擎
├── transport/
│   ├── stdio.go          # stdio 传输
│   ├── http.go           # Streamable HTTP 传输
│   └── sse.go            # SSE 传输（向后兼容）
├── adapter/
│   ├── gin.go            # Gin 路由适配器
│   ├── openapi.go        # OpenAPI 文档适配器
│   └── grpc.go           # gRPC 服务适配器
├── middleware/
│   ├── logger.go         # 日志中间件
│   ├── recovery.go       # panic 恢复
│   ├── ratelimit.go      # 限流
│   ├── timeout.go        # 超时
│   ├── auth.go           # 认证（Bearer/APIKey/Basic/OAuth）
│   └── otel.go           # OpenTelemetry
├── mcptest/
│   ├── client.go         # 测试用 MCP Client
│   ├── snapshot.go       # 快照测试
│   └── assert.go         # 断言辅助
├── inspector/
│   └── server.go         # Web 调试界面
└── examples/
    ├── basic/            # 基础示例
    ├── gin-adapter/      # Gin 适配示例
    ├── openapi-adapter/  # OpenAPI 适配示例
    ├── middleware/        # 中间件示例
    └── enterprise/       # 企业级示例
```

### 7.3 核心流程

```
Client Request (JSON-RPC)
    │
    ▼
Transport Layer (stdio/HTTP/SSE)
    │
    ▼
Protocol Parser (JSON-RPC → MCP Request)
    │
    ▼
Router (匹配 Tool/Resource/Prompt)
    │
    ▼
Middleware Chain (Logger → Auth → RateLimit → ...)
    │
    ▼
Parameter Validation (struct tag 校验)
    │
    ▼
Handler Execution (用户业务逻辑)
    │
    ▼
Result Builder (返回值 → MCP Response)
    │
    ▼
Protocol Serializer (MCP Response → JSON-RPC)
    │
    ▼
Transport Layer → Client
```

---

## 8. API 设计规范

### 8.1 命名规范

| 类别 | 规范 | 示例 |
|------|------|------|
| 包名 | 小写单词 | `gomcp`, `mcptest` |
| 导出函数 | PascalCase | `New()`, `Tool()`, `ImportGin()` |
| Option 函数 | `With` 前缀 | `WithDescription()`, `WithCapabilities()` |
| 中间件 | PascalCase 名词 | `Logger()`, `Recovery()`, `RateLimit()` |
| struct tag | 小写逗号分隔 | `mcp:"required,desc=名称,min=0"` |
| Tool 名称 | snake_case | `search_user`, `create_order` |
| 分组 Tool 名称 | 点分隔 | `user.get`, `order.create` |

### 8.2 错误处理规范

```go
// 框架错误类型
type Error struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

// 预定义错误码（遵循 JSON-RPC 2.0 + MCP 扩展）
var (
    ErrParseError     = &Error{Code: -32700, Message: "Parse error"}
    ErrInvalidRequest = &Error{Code: -32600, Message: "Invalid request"}
    ErrMethodNotFound = &Error{Code: -32601, Message: "Method not found"}
    ErrInvalidParams  = &Error{Code: -32602, Message: "Invalid params"}
    ErrInternal       = &Error{Code: -32603, Message: "Internal error"}
    ErrToolNotFound   = &Error{Code: -32001, Message: "Tool not found"}
    ErrUnauthorized   = &Error{Code: -32002, Message: "Unauthorized"}
    ErrRateLimited    = &Error{Code: -32003, Message: "Rate limit exceeded"}
    ErrValidation     = &Error{Code: -32004, Message: "Validation failed"}
)
```

### 8.3 Option 模式

```go
// Server Options
gomcp.New("name", "version",
    gomcp.WithDescription("服务描述"),
    gomcp.WithToolCapabilities(true),       // 启用 Tool 列表变更通知
    gomcp.WithResourceCapabilities(true),   // 启用 Resource 订阅
    gomcp.WithPromptCapabilities(true),     // 启用 Prompt 列表变更通知
    gomcp.WithMaxRequestSize(10 << 20),     // 最大请求 10MB
    gomcp.WithGracefulShutdown(30*time.Second),
)
```

---

## 9. 开发路线图

### Phase 1 — MVP（第 1-2 月）

**目标**：核心可用，能跑通基本场景

| 周 | 任务 | 交付物 |
|----|------|--------|
| W1 | 项目脚手架、Server 生命周期、stdio 传输 | 能启动的空 Server |
| W2 | Tool 注册（函数式 + struct 自动推导）、schema 生成器 | 能定义和调用 Tool |
| W3 | Resource、Prompt、Context 对象 | 完整 MCP 三大组件 |
| W4 | 参数校验、错误处理、基础中间件（Logger/Recovery） | 可用于简单场景 |
| W5 | Streamable HTTP 传输、SSE 传输 | 支持远程部署 |
| W6 | 文档、examples、README、CI/CD | **MVP 发布 v0.1.0** |

**MVP 验收标准**：
- [ ] 5 行代码定义一个 Tool 并通过 Claude Desktop 调用成功
- [ ] struct tag 自动生成正确的 JSON Schema
- [ ] 参数校验失败返回友好错误
- [ ] stdio 和 HTTP 两种传输均可工作
- [ ] 通过 MCP 协议兼容性测试

### Phase 2 — 核心竞争力（第 3-4 月）

**目标**：差异化功能上线，形成竞争壁垒

| 周 | 任务 | 交付物 |
|----|------|--------|
| W7-8 | Gin 自动适配器 | 现有 Gin 项目一键接入 MCP |
| W9-10 | OpenAPI 自动适配器 | Swagger 文档自动生成 MCP Server |
| W11 | Tool 分组、完整中间件系统 | 企业级路由和中间件 |
| W12 | 认证授权框架（Bearer/APIKey/OAuth） | 生产级安全 |
| W13 | mcptest 测试工具包 | 开发者测试体验 |
| W14 | 性能优化、压测、文档完善 | **v0.5.0 发布** |

### Phase 3 — 生产增强（第 5-6 月）

| 任务 | 交付物 |
|------|--------|
| OpenTelemetry 集成 | 可观测性 |
| 组件版本化 | API 演进支持 |
| 异步任务（Tasks） | 长任务支持 |
| gRPC 适配器 | gRPC 服务接入 |
| Inspector 调试界面 | 开发调试体验 |
| 热加载 Provider | 动态 Tool 管理 |
| **v1.0.0 正式发布** | |

### Phase 4 — 生态建设（第 7-12 月）

| 任务 | 交付物 |
|------|--------|
| 插件市场 CLI | 社区生态 |
| 常用服务 adapter（MySQL/Redis/K8s/ES） | 开箱即用的 Tool 集 |
| 企业版功能（多租户、审计日志、管理后台） | 商业化 |
| MCP Client 支持 | Server-to-Server |
| 代码生成器 | 开发效率 |

---

## 10. 专利规划

### 10.1 专利清单

| 编号 | 专利名称 | 对应功能 | 申请阶段 |
|------|---------|---------|---------|
| PAT-001 | 一种基于结构体标签的 MCP 工具参数 Schema 自动生成方法 | F-002 struct tag → JSON Schema | Phase 1 |
| PAT-002 | 一种 HTTP RESTful 服务到 MCP 协议的自动适配转换方法 | F-009 Gin 适配器 | Phase 2 |
| PAT-003 | 一种基于 OpenAPI 规范的 MCP 工具自动生成方法 | F-010 OpenAPI 适配器 | Phase 2 |
| PAT-004 | 一种面向 MCP 协议的分组路由与中间件链式处理方法 | F-007 + F-008 中间件+分组 | Phase 2 |

### 10.2 专利撰写要点

每个专利需包含：

1. **技术问题**：现有技术的不足
2. **技术方案**：具体实现步骤（含流程图）
3. **技术效果**：量化的改进效果

**示例 — PAT-001：**

- **技术问题**：现有 MCP 工具开发需要手动编写 JSON Schema 定义，代码冗长且易出错，开发效率低
- **技术方案**：
  1. 定义 `mcp` struct tag 语法规范
  2. 通过 Go reflect 包在运行时解析 struct 字段和 tag
  3. 根据字段类型和 tag 指令自动生成符合 JSON Schema Draft 2020-12 的 schema
  4. 支持嵌套 struct、数组、map 等复杂类型的递归解析
  5. 生成的 schema 缓存以避免重复反射开销
- **技术效果**：Tool 定义代码量减少 80%，schema 定义错误率降为 0

### 10.3 时间线

```
Phase 1 结束（第2月）→ 提交 PAT-001 申请（先占申请日）
Phase 2 结束（第4月）→ 提交 PAT-002、PAT-003、PAT-004
```

---

## 11. 商业模式

### 11.1 开源 + 商业双轨

| 层级 | 内容 | 定价 |
|------|------|------|
| **社区版（开源）** | 核心框架全部功能、所有适配器、中间件、测试工具 | 免费（Apache 2.0） |
| **企业版** | 多租户、审计日志、管理后台、SLA 支持 | 按年订阅 |
| **云托管版** | MCP Server 一键部署、自动扩缩容、监控告警 | 按调用量计费 |
| **插件市场** | 社区贡献的 Tool 插件（MySQL/Redis/K8s 等） | 免费 + 付费插件 |
| **技术支持** | 企业级技术咨询、定制开发 | 按项目报价 |

### 11.2 许可证选择

**Apache 2.0**

理由：
- 包含明确的专利授权条款（用户使用代码时自动获得专利许可）
- 企业友好，大公司敢用
- 与官方 Go SDK（Apache 2.0）保持一致
- 允许商业使用，但需保留版权声明

### 11.3 收入预期

| 阶段 | 时间 | 收入来源 | 预期 |
|------|------|---------|------|
| 种子期 | 0-6月 | 无（积累用户） | 0 |
| 增长期 | 6-12月 | 技术支持 + 企业版早期客户 | 小额 |
| 规模期 | 12-24月 | 企业版订阅 + 云托管 + 插件市场 | 规模化 |

---

## 12. 风险评估

| 风险 | 概率 | 影响 | 应对策略 |
|------|------|------|---------|
| MCP 协议被替代或边缘化 | 低 | 高 | 关注 A2A 等竞争协议，保持架构可扩展 |
| 官方 Go SDK 升级为框架 | 中 | 高 | 保持差异化（适配器是核心壁垒），与官方 SDK 保持兼容而非竞争 |
| mcp-go 快速迭代补齐功能 | 中 | 中 | 先发优势 + 适配器专利保护 + 更好的开发体验 |
| Go 在 AI 生态中被边缘化 | 低 | 中 | Go 在基础设施层的地位稳固，AI infra ≠ AI model |
| 用户增长缓慢 | 中 | 中 | 积极参与社区、写技术博客、提交到 awesome-mcp 列表 |
| 专利申请被驳回 | 中 | 低 | 提前找专利代理人评估，准备多个专利点 |

---

## 13. 成功指标

### 13.1 Phase 1 结束（第 2 月）

| 指标 | 目标 |
|------|------|
| GitHub Star | 500+ |
| npm/go module 下载量 | 1,000+ |
| 社区 Issue/PR | 20+ |
| 文档覆盖率 | 100%（每个公共 API 有文档） |
| 测试覆盖率 | > 80% |
| MCP 协议兼容性测试通过率 | 100% |

### 13.2 Phase 2 结束（第 4 月）

| 指标 | 目标 |
|------|------|
| GitHub Star | 2,000+ |
| 周活跃用户（go module 下载） | 500+ |
| 企业试用客户 | 5+ |
| 社区贡献者 | 10+ |
| 专利申请提交 | 4 件 |

### 13.3 v1.0.0 发布（第 6 月）

| 指标 | 目标 |
|------|------|
| GitHub Star | 5,000+ |
| 生产环境使用企业 | 20+ |
| 社区 adapter/plugin | 10+ |
| 技术博客/教程引用 | 50+ |
| 付费客户 | 3+ |

---

## 附录

### A. 参考资料

| 资料 | 链接 |
|------|------|
| MCP 协议规范 | https://modelcontextprotocol.io/specification |
| MCP 官方 Go SDK | https://github.com/modelcontextprotocol/go-sdk |
| mcp-go (mark3labs) | https://github.com/mark3labs/mcp-go |
| FastMCP (Python) | https://github.com/jlowin/fastmcp |
| JSON Schema Draft 2020-12 | https://json-schema.org/draft/2020-12 |
| OpenAPI 3.1 规范 | https://spec.openapis.org/oas/v3.1.0 |

### B. 术语表

| 术语 | 说明 |
|------|------|
| MCP | Model Context Protocol，AI 应用与外部工具的标准通信协议 |
| Tool | MCP 中的可调用函数，由 AI 模型决定何时调用 |
| Resource | MCP 中的数据源，由应用程序控制何时加载 |
| Prompt | MCP 中的提示模板，由用户通过 UI 触发 |
| Transport | 传输层，MCP 消息的底层通信方式（stdio/HTTP/SSE） |
| JSON-RPC | MCP 使用的远程过程调用协议 |
| Schema | JSON Schema，描述 Tool 参数结构的元数据 |
| Adapter | 适配器，将现有服务（Gin/gRPC/OpenAPI）转换为 MCP Tool |

---

> 文档结束 | GoMCP PRD v1.0 | 2026-04-16
