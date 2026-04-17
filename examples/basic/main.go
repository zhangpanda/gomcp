package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/zhangpanda/gomcp"
)

type SearchInput struct {
	Query string `json:"query" mcp:"required,desc=The search query string. Supports keywords and phrases to match against document titles and content."`
	Limit int    `json:"limit" mcp:"default=10,min=1,max=100,desc=Maximum number of results to return. Use smaller values for quick lookups and larger values for comprehensive searches."`
}

type SearchResult struct {
	Items []string `json:"items"`
	Total int      `json:"total"`
}

func main() {
	s := gomcp.New("gomcp-demo", "1.0.0")

	// --- Middleware ---
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Use(gomcp.Logger())
	s.Use(gomcp.Timeout(30 * time.Second))
	s.Use(gomcp.RateLimit(600))

	// --- Tools ---

	s.Tool("greet_user", "Greet a user by name. Returns a personalized greeting message. If no name is provided, defaults to 'World'. Use this tool to verify server connectivity or welcome a user.", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		name := ctx.String("name")
		if name == "" {
			name = "World"
		}
		return ctx.Text(fmt.Sprintf("Hello, %s!", name)), nil
	})

	s.ToolFunc("search_documents", "Search documents by keyword using full-text matching. Returns matching documents ranked by relevance with titles and content snippets. Use this when you need to find documents containing specific words or phrases. For semantic meaning-based search, use search_semantic instead.", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		items := []string{fmt.Sprintf("Result for %q", in.Query)}
		return SearchResult{Items: items, Total: len(items)}, nil
	})

	s.ToolFunc("search_semantic", "Search documents using semantic embedding-based matching. Returns results ranked by meaning similarity rather than exact keyword match. Use this when the user's intent matters more than exact wording. For exact keyword matching, use search_documents instead.", func(ctx *gomcp.Context, in SearchInput) (SearchResult, error) {
		items := []string{fmt.Sprintf("Semantic result for %q", in.Query)}
		return SearchResult{Items: items, Total: len(items)}, nil
	})

	s.AsyncTool("generate_report", "Generate an analytics report for a given topic. This is a long-running operation that executes asynchronously and may take several minutes. Returns a task ID immediately; poll tasks/get for the result, or call tasks/cancel to abort. Use this for comprehensive data analysis tasks.", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		time.Sleep(2 * time.Second)
		return ctx.Text("Report complete for: " + ctx.String("topic")), nil
	})

	s.Tool("get_config", "Retrieve the current server configuration including version, environment, and feature flags. Use this to inspect server state or verify deployment settings. This is a read-only operation with no side effects.", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.JSON(map[string]any{"version": "1.0.0", "env": "dev", "features": []string{"search", "reports"}}), nil
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
