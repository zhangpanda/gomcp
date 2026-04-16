package main

import (
	"fmt"
	"log"
	"time"

	"github.com/istarshine/gomcp"
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
	s := gomcp.New("demo-server", "0.1.0")

	// Global middleware
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Use(gomcp.Logger())
	s.Use(gomcp.Timeout(30 * time.Second))

	// Top-level tool
	s.Tool("hello", "Say hello", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		name := ctx.String("name")
		if name == "" {
			name = "World"
		}
		return ctx.Text(fmt.Sprintf("Hello, %s!", name)), nil
	})

	// Tool group: search.*
	search := s.Group("search")
	search.ToolFunc("docs", "Search documents", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		items := []string{fmt.Sprintf("Doc result for %q", in.Query)}
		return SearchResult{Items: items, Total: len(items)}, nil
	})
	search.ToolFunc("users", "Search users", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		items := []string{fmt.Sprintf("User result for %q", in.Query)}
		return SearchResult{Items: items, Total: len(items)}, nil
	})

	// Resource
	s.Resource("config://app", "App Config", func(ctx *gomcp.Context) (any, error) {
		return map[string]any{"version": "0.1.0", "env": "dev"}, nil
	})

	s.ResourceTemplate("users://{id}/profile", "User Profile", func(ctx *gomcp.Context) (any, error) {
		return map[string]any{"id": ctx.String("id"), "name": "User " + ctx.String("id")}, nil
	})

	// Prompt
	s.Prompt("code_review", "Code review assistant",
		[]gomcp.PromptArgument{gomcp.PromptArg("language", "Programming language", true)},
		func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
			return []gomcp.PromptMessage{
				gomcp.UserMsg(fmt.Sprintf("Review this %s code.", ctx.String("language"))),
			}, nil
		},
	)

	log.Println("Starting GoMCP demo server...")
	if err := s.Stdio(); err != nil {
		log.Fatal(err)
	}
}
