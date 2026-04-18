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

// ServerInfo describes the MCP server name and version.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo        `json:"serverInfo"`
}

// ServerCapabilities declares the server's supported features.
type ServerCapabilities struct {
	Tools     *ToolCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability `json:"resources,omitempty"`
	Prompts   *PromptCapability   `json:"prompts,omitempty"`
}

// ToolCapability describes tool-related capabilities.
type ToolCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapability describes resource-related capabilities.
type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapability describes prompt-related capabilities.
type PromptCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolInfo describes a registered tool and its input schema.
type ToolInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	InputSchema JSONSchema        `json:"inputSchema"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// JSONSchema represents a JSON Schema definition for tool parameters.
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

// ToolListResult is the response to tools/list.
type ToolListResult struct {
	Tools []ToolInfo `json:"tools"`
}

// CallToolParams is the request parameters for tools/call.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// CallToolResult is the response from a tool invocation.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a single content item in a tool result.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
}

// TextResult creates a successful text result.
func TextResult(s string) *CallToolResult {
	return &CallToolResult{Content: []ContentBlock{{Type: "text", Text: s}}}
}

// ErrorResult creates an error result with isError set to true.
func ErrorResult(msg string) *CallToolResult {
	return &CallToolResult{Content: []ContentBlock{{Type: "text", Text: msg}}, IsError: true}
}
