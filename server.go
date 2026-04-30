package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/zhangpanda/gomcp/schema"
	"github.com/zhangpanda/gomcp/transport"
)

const protocolVersion = "2025-11-25"

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
	versionLatest     map[string]string // unversioned base name → registration key, e.g. "search" → "search@2.0"
	resources         []resourceEntry
	resourceTemplates []resourceTemplateEntry
	prompts           []promptEntry
	middlewares       []Middleware
	logger            *slog.Logger
	mu                sync.RWMutex
	notifyFn          []func(method string, params any) // set by HTTP transport; protected by mu
	taskMgr           *taskManager
	completions       []completionEntry
	sessions          *SessionManager
	maxRequestSize    int64 // default 10MB
}

// Option configures the Server.
type Option func(*Server)

// WithDescription sets the server description.
func WithDescription(desc string) Option { return func(s *Server) { s.desc = desc } }

// WithLogger sets a custom logger for the server.
func WithLogger(l *slog.Logger) Option { return func(s *Server) { s.logger = l } }

// WithMaxRequestSize sets the maximum request body size in bytes (default 10MB).
func WithMaxRequestSize(n int64) Option { return func(s *Server) { s.maxRequestSize = n } }

// New creates a new MCP Server.
func New(name, version string, opts ...Option) *Server {
	s := &Server{
		name:          name,
		version:       version,
		tools:         make(map[string]toolEntry),
		versionLatest: make(map[string]string),
		logger:        slog.Default(),
		sessions:      newSessionManager(),
		maxRequestSize: 10 << 20, // 10MB
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Server) ctx() context.Context { return context.Background() }

// Sessions returns the session manager for inspecting active sessions.
func (s *Server) Sessions() *SessionManager { return s.sessions }

func (s *Server) notify(method string) {
	s.mu.RLock()
	fns := s.notifyFn
	s.mu.RUnlock()
	for _, fn := range fns {
		fn(method, nil)
	}
}

// ToolOption configures a tool registration.
type ToolOption func(*toolEntry)

// Version sets the version for a tool. Versioned tools are registered as "name@version".
func Version(v string) ToolOption {
	return func(e *toolEntry) {
		if e.info.Annotations == nil {
			e.info.Annotations = make(map[string]string)
		}
		e.info.Annotations["version"] = v
	}
}

// Deprecated marks a tool as deprecated.
func Deprecated(msg string) ToolOption {
	return func(e *toolEntry) {
		if e.info.Annotations == nil {
			e.info.Annotations = make(map[string]string)
		}
		e.info.Annotations["deprecated"] = msg
	}
}

func (s *Server) registerTool(name string, entry toolEntry, opts []ToolOption) {
	for _, o := range opts {
		o(&entry)
	}
	key := name
	if v, ok := entry.info.Annotations["version"]; ok && v != "" {
		key = name + "@" + v
		entry.info.Name = key
	}
	s.mu.Lock()
	s.tools[key] = entry
	if v, ok := entry.info.Annotations["version"]; ok && v != "" {
		s.bumpVersionLatestLocked(name, v, key)
	}
	s.mu.Unlock()
	s.notify("notifications/tools/list_changed")
}

// Tool registers a tool with a simple HandlerFunc.
func (s *Server) Tool(name, description string, handler HandlerFunc, opts ...ToolOption) {
	entry := toolEntry{
		info: ToolInfo{
			Name:        name,
			Description: description,
			InputSchema: JSONSchema{Type: "object", Properties: make(map[string]JSONSchema)},
		},
		handler: handler,
	}
	s.registerTool(name, entry, opts)
}

// RegisterToolRaw registers a tool with a pre-built schema (used by adapters).
func (s *Server) RegisterToolRaw(name, description string, inputSchema JSONSchema, handler HandlerFunc, opts ...ToolOption) {
	entry := toolEntry{
		info:    ToolInfo{Name: name, Description: description, InputSchema: inputSchema},
		handler: handler,
	}
	s.registerTool(name, entry, opts)
}

// ToolFunc registers a tool using a typed function.
// Signature: func(*Context, InputStruct) (OutputType, error)
func (s *Server) ToolFunc(name, description string, fn any, opts ...ToolOption) {
	fv := reflect.ValueOf(fn)
	ft := fv.Type()
	if ft.Kind() != reflect.Func || ft.NumIn() != 2 || ft.NumOut() != 2 {
		panic(fmt.Sprintf("gomcp: ToolFunc %q requires func(*Context, Input) (Output, error)", name))
	}
	ctxType := reflect.TypeOf((*Context)(nil))
	errType := reflect.TypeOf((*error)(nil)).Elem()
	if ft.In(0) != ctxType {
		panic(fmt.Sprintf("gomcp: ToolFunc %q first param must be *Context, got %s", name, ft.In(0)))
	}
	if !ft.Out(1).Implements(errType) {
		panic(fmt.Sprintf("gomcp: ToolFunc %q second return must implement error, got %s", name, ft.Out(1)))
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
		if errVal := results[1]; !errVal.IsNil() {
			if err, ok := errVal.Interface().(error); ok {
				return nil, err
			}
			return nil, fmt.Errorf("%v", errVal.Interface())
		}
		return toResult(results[0].Interface()), nil
	}

	entry := toolEntry{
		info:      ToolInfo{Name: name, Description: description, InputSchema: inputSchema},
		handler:   handler,
		schemaRes: &schemaRes,
	}
	s.registerTool(name, entry, opts)
}

// bumpVersionLatestLocked records the latest semver among keys "base@*" for a given base.
// Must be called with s.mu Lock held (same critical section as mutating s.tools).
func (s *Server) bumpVersionLatestLocked(base, version, key string) {
	if s.versionLatest == nil {
		s.versionLatest = make(map[string]string)
	}
	curKey, ok := s.versionLatest[base]
	if !ok {
		s.versionLatest[base] = key
		return
	}
	curVer := curKey[len(base)+1:] // part after "base@"
	if compareSemver(version, curVer) > 0 {
		s.versionLatest[base] = key
	}
}

// resolveToolVersion finds a versioned tool when the exact name is not registered.
// Must be called with s.mu.RLock held.
func (s *Server) resolveToolVersion(name string) (toolEntry, bool) {
	// O(1): only when the client passes an unversioned base (e.g. "search" for search@* family).
	if !strings.Contains(name, "@") {
		if key, ok := s.versionLatest[name]; ok {
			if e, ok2 := s.tools[key]; ok2 {
				return e, true
			}
		}
	}
	// O(n) fallback: explicit "base@ver" that is missing, or base names with "@" in the logical name.
	return s.resolveToolVersionScan(name)
}

// resolveToolVersionScan is the full-map scan; kept for edge cases and index/tool drift.
func (s *Server) resolveToolVersionScan(name string) (toolEntry, bool) {
	prefix := name + "@"
	var best toolEntry
	var bestVersion string
	found := false
	for key, entry := range s.tools {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			v := key[len(prefix):]
			if !found || compareSemver(v, bestVersion) > 0 {
				best = entry
				bestVersion = v
				found = true
			}
		}
	}
	return best, found
}

// compareSemver compares two version strings numerically by splitting on ".".
// Returns >0 if a > b, <0 if a < b, 0 if equal. Non-numeric segments parse as 0.
func compareSemver(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		ai, bi := semverPart(ap, i), semverPart(bp, i)
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

func semverPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
	if err != nil {
		return 0
	}
	return n
}

func toResult(v any) *CallToolResult {
	switch val := v.(type) {
	case *CallToolResult:
		return val
	case string:
		return TextResult(val)
	default:
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return TextResult(fmt.Sprint(v))
		}
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
	case "tasks/get":
		return s.handleTasksGet(msg)
	case "tasks/cancel":
		return s.handleTasksCancel(msg)
	case "completion/complete":
		return s.handleComplete(msg)
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
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return newResponse(msg.ID, ToolListResult{Tools: tools})
}

func (s *Server) handleToolsCall(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	var params CallToolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params: "+err.Error())
	}

	s.mu.RLock()
	entry, ok := s.tools[params.Name]
	if !ok {
		// version fallback: "search" → try find "search" or latest "search@*"
		entry, ok = s.resolveToolVersion(params.Name)
	}
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

	// attach session (header name matched case-insensitively, like net/http)
	sessionID := transport.LookupHeader(ctx, "Mcp-Session-Id")
	c.session = s.sessions.GetOrCreate(sessionID)

	var result *CallToolResult

	// safety net: recover panics even without Recovery middleware
	func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("unrecovered panic in tool handler", "tool", params.Name, "panic", fmt.Sprintf("%v", r))
				c.Set("_panic", fmt.Sprintf("internal error: %v", r))
			}
		}()
		err := executeChain(c, mws, func() error {
			var handlerErr error
			result, handlerErr = entry.handler(c)
			return handlerErr
		})
		if err != nil {
			c.Set("_chain_error", err.Error())
		}
	}()

	// panic recovered → friendly error result
	if panicMsg, ok := c.Get("_panic"); ok {
		if s, ok := panicMsg.(string); ok {
			return newResponse(msg.ID, ErrorResult(s))
		}
	}

	if errMsg, ok := c.Get("_chain_error"); ok {
		if s, ok := errMsg.(string); ok {
			return newResponse(msg.ID, ErrorResult(s))
		}
	}
	if result == nil {
		result = &CallToolResult{}
	}
	return newResponse(msg.ID, result)
}
