package main

import (
	"fmt"
	"log"
	"strings"
	"time"

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
	s := gomcp.New("demo-server", "1.0.0")

	// --- Middleware ---
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Use(gomcp.Logger())
	s.Use(gomcp.Timeout(30 * time.Second))
	s.Use(gomcp.RateLimit(600))

	// --- Tools ---
	s.Tool("hello", "Say hello", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		name := ctx.String("name")
		if name == "" {
			name = "World"
		}
		return ctx.Text(fmt.Sprintf("Hello, %s!", name)), nil
	})

	// --- Tool Groups ---
	search := s.Group("search")
	search.ToolFunc("docs", "Search documents", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		items := []string{fmt.Sprintf("Doc result for %q", in.Query)}
		return SearchResult{Items: items, Total: len(items)}, nil
	})

	// --- Component Versioning ---
	s.ToolFunc("search", "Search v1", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		return SearchResult{Items: []string{"v1:" + in.Query}, Total: 1}, nil
	}, gomcp.Version("1.0"))

	s.ToolFunc("search", "Search v2 with semantic matching", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		return SearchResult{Items: []string{"v2:" + in.Query, "semantic:" + in.Query}, Total: 2}, nil
	}, gomcp.Version("2.0"))

	// --- Async Task ---
	s.AsyncTool("report", "Generate a slow report", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		time.Sleep(2 * time.Second) // simulate work
		return ctx.Text("Report complete for: " + ctx.String("topic")), nil
	})

	// --- Resource ---
	s.Resource("config://app", "App Config", func(ctx *gomcp.Context) (any, error) {
		return map[string]any{"version": "1.0.0", "env": "dev"}, nil
	})

	s.ResourceTemplate("users://{id}/profile", "User Profile", func(ctx *gomcp.Context) (any, error) {
		return map[string]any{"id": ctx.String("id"), "name": "User " + ctx.String("id")}, nil
	})

	// --- Prompt ---
	s.Prompt("code_review", "Code review assistant",
		[]gomcp.PromptArgument{gomcp.PromptArg("language", "Programming language", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{
				gomcp.UserMsg(fmt.Sprintf("Review this %s code for bugs and performance.", ctx.String("language"))),
			}, nil
		},
	)

	// --- Completion ---
	s.Completion("prompt", "code_review", "language", func(partial string) []string {
		all := []string{"go", "python", "typescript", "rust", "java", "c++"}
		var out []string
		for _, lang := range all {
			if strings.HasPrefix(lang, partial) {
				out = append(out, lang)
			}
		}
		return out
	})

	log.Println("Starting GoMCP demo server...")
	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
