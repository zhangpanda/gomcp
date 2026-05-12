# 5 分钟搭一个 AI 搜索工具

> 从零到 Claude Desktop 可调用,只需一个 Go 文件。

## 最终效果

Claude Desktop 里输入"搜索 Go 并发"，AI 自动调用你的 `search` 工具，返回结果。

## 步骤

### 1. 初始化项目

```bash
mkdir mcp-search && cd mcp-search
go mod init mcp-search
go get github.com/zhangpanda/gomcp
```

### 2. 写 main.go

```go
package main

import (
    "fmt"
    "github.com/zhangpanda/gomcp"
)

type SearchInput struct {
    Query string `json:"query" mcp:"required,desc=搜索关键词"`
    Limit int    `json:"limit" mcp:"default=5,min=1,max=20"`
}

type Result struct {
    Title string `json:"title"`
    URL   string `json:"url"`
}

func main() {
    s := gomcp.New("my-search", "1.0.0")

    s.ToolFunc("search", "按关键词搜索文档", func(ctx *gomcp.Context, in SearchInput) ([]Result, error) {
        // 替换成你的真实搜索逻辑
        results := []Result{
            {Title: fmt.Sprintf("关于 %q 的文档", in.Query), URL: "https://example.com/1"},
        }
        return results, nil
    })

    s.Stdio()
}
```

### 3. 配置 Claude Desktop

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "my-search": {
      "command": "go",
      "args": ["run", "/path/to/mcp-search"]
    }
  }
}
```

重启 Claude Desktop，输入"搜索 xxx"即可触发。

### 4. 本地调试

不想每次重启 Claude？用 Inspector：

```go
// 把 s.Stdio() 换成：
s.Dev(":9090")
```

打开 http://localhost:9090 ，浏览器里直接测试工具。

## 要点

- `mcp:"required"` → 参数必填，AI 不传就报错
- `mcp:"min=1,max=20"` → 自动校验范围
- `mcp:"default=5"` → AI 不传时用默认值
- 返回值自动序列化为 JSON，AI 能直接读
