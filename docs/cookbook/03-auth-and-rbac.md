# 5 分钟加认证 + 权限控制

> 让你的 MCP Server 只允许授权用户调用，管理员才能执行危险操作。

## 场景

你的 MCP Server 通过 HTTP 暴露给多个 AI 客户端，需要：
1. 所有工具调用必须带 Bearer token
2. 删除类操作只有 admin 角色能执行

## 步骤

### 1. 加 BearerAuth 中间件

```go
package main

import (
    "fmt"
    "github.com/zhangpanda/gomcp"
)

func main() {
    s := gomcp.New("secure-server", "1.0.0")

    // 认证中间件：initialize/ping 免认证，其余需要 token
    s.Use(gomcp.BearerAuthSkipHandshake(func(token string) (map[string]any, error) {
        // 替换成你的 JWT 验证 / 数据库查询
        switch token {
        case "admin-secret":
            return map[string]any{"sub": "admin", "roles": []string{"admin", "user"}}, nil
        case "user-secret":
            return map[string]any{"sub": "alice", "roles": []string{"user"}}, nil
        default:
            return nil, fmt.Errorf("invalid token")
        }
    }))

    // 普通工具：任何认证用户都能调
    s.Tool("list_items", "列出所有项目", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
        return ctx.Text("item1, item2, item3"), nil
    })

    // 管理员工具：用 Group + RequireRole
    admin := s.Group("admin")
    admin.Use(gomcp.RequireRole("admin"))
    admin.Tool("delete_all", "删除所有数据（危险）", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
        return ctx.Text("all deleted"), nil
    })

    s.HTTP(":8080")
}
```

### 2. 客户端怎么传 token

HTTP 请求头：
```
POST /mcp HTTP/1.1
Authorization: Bearer admin-secret
Content-Type: application/json

{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"admin.delete_all","arguments":{}}}
```

### 3. 错误表现

| 情况 | 返回 |
|------|------|
| 无 token 调 `list_items` | `isError=true`, text="auth: missing or invalid Bearer token" |
| user token 调 `admin.delete_all` | `isError=true`, text="auth: access denied: requires role admin" |
| admin token 调 `admin.delete_all` | 正常结果 |

> **注意**：认证/授权失败是 `tools/call` 结果里的 `isError=true`，不是 JSON-RPC error 也不是 HTTP 401。客户端需要检查 result 的 `isError` 字段。

### 4. 其他认证方式

```go
// API Key（通过 Header）
s.Use(gomcp.APIKeyAuthSkipHandshake("X-API-Key", func(key string) (map[string]any, error) {
    if key == "sk-xxx" {
        return map[string]any{"roles": []string{"user"}}, nil
    }
    return nil, fmt.Errorf("bad key")
}))

// HTTP Basic
s.Use(gomcp.BasicAuthSkipHandshake(func(user, pass string) (map[string]any, error) {
    if user == "admin" && pass == "123" {
        return map[string]any{"roles": []string{"admin"}}, nil
    }
    return nil, fmt.Errorf("wrong credentials")
}))
```

### 5. SSE 连接也要鉴权？

```go
s.HTTP(":8080") // 改成手动配置：

// 在 Server 构建后、HTTP 启动前：
import "github.com/zhangpanda/gomcp/transport"

// SSE GET /mcp 连接也走 Bearer 校验
// （否则任何人都能订阅通知流）
```

用 `WithSSEAuth` 选项或在 `Server.Handler()` 外层包一个 HTTP middleware 拦截 GET 请求。
