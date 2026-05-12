# 5 分钟把现有 REST API 变成 MCP 工具

> 已有一套 Gin API？一行代码让 AI 能调用它。

## 场景

你有一个跑着的 Gin 服务（用户 CRUD），想让 Claude/Cursor 直接调用这些接口，不想重写 handler。

## 步骤

### 1. 现有 Gin 路由（假设已有）

```go
r := gin.Default()
r.GET("/api/users", listUsers)
r.GET("/api/users/:id", getUser)
r.POST("/api/users", createUser)
r.DELETE("/api/users/:id", deleteUser)
```

### 2. 加一个 main_mcp.go

```go
package main

import (
    "github.com/zhangpanda/gomcp"
    "github.com/zhangpanda/gomcp/adapter"
)

func main() {
    ginRouter := setupRouter() // 你现有的 Gin 路由

    s := gomcp.New("my-api", "1.0.0")
    adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
        IncludePaths: []string{"/api/"},
    })

    s.Stdio() // 或 s.HTTP(":8080")
}
```

就这么多。框架自动：
- 扫描 Gin 路由表
- 每个路由生成一个 MCP tool（名字如 `get_api_users_by_id`）
- 路径参数变成 required 参数
- POST/PUT 自动加 `body` 参数
- 调用时通过 `httptest.NewRecorder` 走真实 Gin handler

### 3. AI 怎么调

Claude 看到的工具列表：

```
get_api_users          — GET /api/users
get_api_users_by_id    — GET /api/users/:id
post_api_users         — POST /api/users
delete_api_users_by_id — DELETE /api/users/:id
```

调用示例（AI 自动生成）：
```json
{"name": "get_api_users_by_id", "arguments": {"id": "42"}}
```

### 4. 过滤和自定义命名

```go
adapter.ImportGin(s, ginRouter, adapter.ImportOptions{
    IncludePaths: []string{"/api/v2/"},
    ExcludePaths: []string{"/api/v2/internal/"},
    NamingFunc: func(method, path string) string {
        // 自定义工具名
        return "myapp_" + strings.ToLower(method) + "_" + sanitize(path)
    },
})
```

## 同理：OpenAPI / gRPC

```go
// 从 Swagger 文件导入
adapter.ImportOpenAPI(s, "./swagger.yaml", adapter.OpenAPIOptions{
    TagFilter: []string{"public"},
    ServerURL: "https://api.example.com",
})

// 从 gRPC 服务导入（需要开启 reflection）
adapter.ImportGRPC(s, grpcConn, adapter.GRPCOptions{
    Services: []string{"user.UserService"},
})
```
