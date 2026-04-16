package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/istarshine/gomcp/schema"
)

const protocolVersion = "2024-11-05"

// HandlerFunc is the simplest tool handler signature.
type HandlerFunc func(ctx *Context) (*CallToolResult, error)

type toolEntry struct {
	info      ToolInfo
	handler   HandlerFunc
	schemaRes *schema.Result // for validation, nil for simple handlers
}

// Server is the core MCP server.
type Server struct {
	name              string
	version           string
	desc              string
	tools             map[string]toolEntry
	resources         []resourceEntry
	resourceTemplates []resourceTemplateEntry
	prompts           []promptEntry
	middlewares       []Middleware
	logger            *slog.Logger
	mu                sync.RWMutex
	notifyFn          func(method string, params any) // set by HTTP transport
}

// Option configures the Server.
type Option func(*Server)

func WithDescription(desc string) Option { return func(s *Server) { s.desc = desc } }
func WithLogger(l *slog.Logger) Option   { return func(s *Server) { s.logger = l } }

// New creates a new MCP Server.
func New(name, version string, opts ...Option) *Server {
	s := &Server{
		name:   name,
		version: version,
		tools:  make(map[string]toolEntry),
		logger: slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Server) ctx() context.Context { return context.Background() }

func (s *Server) notify(method string) {
	if s.notifyFn != nil {
		s.notifyFn(method, nil)
	}
}

// Tool registers a tool with a simple HandlerFunc.
func (s *Server) Tool(name, description string, handler HandlerFunc) {
	s.mu.Lock()
	s.tools[name] = toolEntry{
		info: ToolInfo{
			Name:        name,
			Description: description,
			InputSchema: JSONSchema{Type: "object", Properties: make(map[string]JSONSchema)},
		},
		handler: handler,
	}
	s.mu.Unlock()
	s.notify("notifications/tools/list_changed")
}

// RegisterToolRaw registers a tool with a pre-built schema (used by adapters).
func (s *Server) RegisterToolRaw(name, description string, inputSchema JSONSchema, handler HandlerFunc) {
	s.mu.Lock()
	s.tools[name] = toolEntry{
		info:    ToolInfo{Name: name, Description: description, InputSchema: inputSchema},
		handler: handler,
	}
	s.mu.Unlock()
	s.notify("notifications/tools/list_changed")
}

// ToolFunc registers a tool using a typed function.
// Signature: func(*Context, InputStruct) (OutputType, error)
func (s *Server) ToolFunc(name, description string, fn any) {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()
	if ft.Kind() != reflect.Func || ft.NumIn() != 2 || ft.NumOut() != 2 {
		panic(fmt.Sprintf("gomcp: ToolFunc %q requires func(*Context, Input) (Output, error)", name))
	}

	inputType := ft.In(1)
	inputSchema := generateSchema(inputType)
	schemaRes := schema.Generate(inputType)

	handler := func(ctx *Context) (*CallToolResult, error) {
		inPtr := reflect.New(inputType)
		if err := ctx.Bind(inPtr.Interface()); err != nil {
			return ErrorResult("invalid parameters: " + err.Error()), nil
		}
		results := fv.Call([]reflect.Value{reflect.ValueOf(ctx), inPtr.Elem()})
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}
		return toResult(results[0].Interface()), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = toolEntry{
		info:      ToolInfo{Name: name, Description: description, InputSchema: inputSchema},
		handler:   handler,
		schemaRes: &schemaRes,
	}
}

func toResult(v any) *CallToolResult {
	switch val := v.(type) {
	case *CallToolResult:
		return val
	case string:
		return TextResult(val)
	default:
		data, _ := json.MarshalIndent(val, "", "  ")
		return TextResult(string(data))
	}
}

// HandleRaw processes a raw JSON-RPC message. Used by mcptest and custom transports.
func (s *Server) HandleRaw(ctx context.Context, raw json.RawMessage) json.RawMessage {
	return s.rawHandler(ctx, raw)
}

func (s *Server) handleRequestInternal(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "notifications/initialized":
		return nil
	case "ping":
		return newResponse(msg.ID, map[string]any{})
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	case "resources/list":
		return s.handleResourcesList(msg)
	case "resources/templates/list":
		return s.handleResourceTemplatesList(msg)
	case "resources/read":
		return s.handleResourcesRead(msg)
	case "prompts/list":
		return s.handlePromptsList(msg)
	case "prompts/get":
		return s.handlePromptsGet(msg)
	default:
		return newErrorResponse(msg.ID, -32601, "method not found: "+msg.Method)
	}
}

func (s *Server) handleInitialize(msg *jsonrpcMessage) *jsonrpcMessage {
	caps := ServerCapabilities{Tools: &ToolCapability{ListChanged: true}}

	s.mu.RLock()
	hasResources := len(s.resources) > 0 || len(s.resourceTemplates) > 0
	hasPrompts := len(s.prompts) > 0
	s.mu.RUnlock()

	if hasResources {
		caps.Resources = &ResourceCapability{ListChanged: true}
	}
	if hasPrompts {
		caps.Prompts = &PromptCapability{ListChanged: true}
	}

	return newResponse(msg.ID, InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    caps,
		ServerInfo:      ServerInfo{Name: s.name, Version: s.version},
	})
}

func (s *Server) handleToolsList(msg *jsonrpcMessage) *jsonrpcMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tools := make([]ToolInfo, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t.info)
	}
	return newResponse(msg.ID, ToolListResult{Tools: tools})
}

func (s *Server) handleToolsCall(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	var params CallToolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params: "+err.Error())
	}

	s.mu.RLock()
	entry, ok := s.tools[params.Name]
	mws := make([]Middleware, len(s.middlewares))
	copy(mws, s.middlewares)
	s.mu.RUnlock()

	if !ok {
		return newErrorResponse(msg.ID, -32001, "tool not found: "+params.Name)
	}

	// validate parameters if schema available
	if entry.schemaRes != nil {
		if err := schema.Validate(params.Arguments, *entry.schemaRes); err != nil {
			return newResponse(msg.ID, ErrorResult("validation failed: "+err.Error()))
		}
	}

	c := newContext(ctx, params.Arguments, s.logger.With("tool", params.Name))
	c.Set("_tool_name", params.Name)

	var result *CallToolResult
	var handlerErr error

	err := executeChain(c, mws, func() error {
		result, handlerErr = entry.handler(c)
		return handlerErr
	})

	// check if panic was recovered
	if panicMsg, ok := c.Get("_panic"); ok {
		return newResponse(msg.ID, ErrorResult(panicMsg.(string)))
	}

	if err != nil {
		return newErrorResponse(msg.ID, -32603, err.Error())
	}
	return newResponse(msg.ID, result)
}
