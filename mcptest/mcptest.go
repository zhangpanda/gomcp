// Package mcptest provides testing utilities for GoMCP servers.
package mcptest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// Client is an in-memory MCP client for testing servers without transport.
type Client struct {
	t      *testing.T
	server *gomcp.Server
}

// NewClient creates a test client connected to the given server.
func NewClient(t *testing.T, s *gomcp.Server) *Client {
	t.Helper()
	return &Client{t: t, server: s}
}

// Initialize sends an initialize request and returns the result.
func (c *Client) Initialize() map[string]any {
	c.t.Helper()
	return c.call("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mcptest", "version": "1.0"},
	})
}

// ListTools returns the list of tool names.
func (c *Client) ListTools() []string {
	c.t.Helper()
	result := c.call("tools/list", map[string]any{})
	tools, _ := result["tools"].([]any)
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if m, ok := t.(map[string]any); ok {
			names = append(names, m["name"].(string))
		}
	}
	return names
}

// CallTool invokes a tool and returns the result.
func (c *Client) CallTool(name string, args map[string]any) *ToolResult {
	c.t.Helper()
	result := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	tr := &ToolResult{raw: result}

	if content, ok := result["content"].([]any); ok && len(content) > 0 {
		if block, ok := content[0].(map[string]any); ok {
			tr.text, _ = block["text"].(string)
		}
	}
	tr.isError, _ = result["isError"].(bool)
	return tr
}

// ReadResource reads a resource by URI.
func (c *Client) ReadResource(uri string) string {
	c.t.Helper()
	result := c.call("resources/read", map[string]any{"uri": uri})
	if contents, ok := result["contents"].([]any); ok && len(contents) > 0 {
		if m, ok := contents[0].(map[string]any); ok {
			return m["text"].(string)
		}
	}
	return ""
}

// GetPrompt gets a prompt by name with arguments.
func (c *Client) GetPrompt(name string, args map[string]string) []map[string]any {
	c.t.Helper()
	result := c.call("prompts/get", map[string]any{"name": name, "arguments": args})
	msgs, _ := result["messages"].([]any)
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if mm, ok := m.(map[string]any); ok {
			out = append(out, mm)
		}
	}
	return out
}

func (c *Client) call(method string, params any) map[string]any {
	c.t.Helper()
	paramsJSON, _ := json.Marshal(params)
	reqJSON, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  json.RawMessage(paramsJSON),
	})

	resp := c.server.HandleRaw(context.Background(), reqJSON)
	if resp == nil {
		return nil
	}

	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		c.t.Fatalf("mcptest: unmarshal response: %v", err)
	}
	if msg.Error != nil {
		c.t.Fatalf("mcptest: RPC error %d: %s", msg.Error.Code, msg.Error.Message)
	}

	var result map[string]any
	json.Unmarshal(msg.Result, &result)
	return result
}

// ToolResult wraps a tool call response with assertion helpers.
type ToolResult struct {
	raw     map[string]any
	text    string
	isError bool
}

// Text returns the text content of the result.
func (r *ToolResult) Text() string { return r.text }

// IsError returns true if the tool returned an error.
func (r *ToolResult) IsError() bool { return r.isError }

// AssertContains fails the test if the result text doesn't contain substr.
func (r *ToolResult) AssertContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.text, substr) {
		t.Errorf("expected result to contain %q, got %q", substr, r.text)
	}
}

// AssertNoError fails the test if the result is an error.
func (r *ToolResult) AssertNoError(t *testing.T) {
	t.Helper()
	if r.isError {
		t.Errorf("expected no error, got: %s", r.text)
	}
}

// AssertIsError fails the test if the result is NOT an error.
func (r *ToolResult) AssertIsError(t *testing.T) {
	t.Helper()
	if !r.isError {
		t.Errorf("expected error result, got: %s", r.text)
	}
}

// MatchSnapshot compares the result text against a stored snapshot file.
// On first run (or when UPDATE_SNAPSHOTS=1), it creates/updates the snapshot.
func MatchSnapshot(t *testing.T, name string, result *ToolResult) {
	t.Helper()
	dir := filepath.Join("testdata", "snapshots")
	file := filepath.Join(dir, name+".snap")

	actual := result.Text()

	if _, err := os.Stat(file); os.IsNotExist(err) || os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(file, []byte(actual), 0o644)
		t.Logf("snapshot %q written", file)
		return
	}

	expected, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read snapshot %q: %v", file, err)
	}

	if string(expected) != actual {
		t.Errorf("snapshot %q mismatch:\n--- expected ---\n%s\n--- actual ---\n%s", name, string(expected), actual)
	}
}
