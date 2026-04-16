package gomcp

import "encoding/json"

// JSON-RPC 2.0 types

type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func newResponse(id json.RawMessage, result any) *jsonrpcMessage {
	data, _ := json.Marshal(result)
	return &jsonrpcMessage{JSONRPC: "2.0", ID: id, Result: data}
}

func newErrorResponse(id json.RawMessage, code int, msg string) *jsonrpcMessage {
	return &jsonrpcMessage{JSONRPC: "2.0", ID: id, Error: &jsonrpcError{Code: code, Message: msg}}
}

// MCP protocol types

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo        `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools     *ToolCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability `json:"resources,omitempty"`
	Prompts   *PromptCapability   `json:"prompts,omitempty"`
}

type ToolCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	InputSchema JSONSchema        `json:"inputSchema"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type JSONSchema struct {
	Type       string                `json:"type"`
	Properties map[string]JSONSchema `json:"properties,omitempty"`
	Required   []string              `json:"required,omitempty"`
	// field constraints
	Description string   `json:"description,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []any    `json:"enum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Items       *JSONSchema `json:"items,omitempty"`
}

type ToolListResult struct {
	Tools []ToolInfo `json:"tools"`
}

type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
}

func TextResult(s string) *CallToolResult {
	return &CallToolResult{Content: []ContentBlock{{Type: "text", Text: s}}}
}

func ErrorResult(msg string) *CallToolResult {
	return &CallToolResult{Content: []ContentBlock{{Type: "text", Text: msg}}, IsError: true}
}
