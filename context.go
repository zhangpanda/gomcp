package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// Context carries request-scoped data through the handler chain.
type Context struct {
	ctx     context.Context
	args    map[string]any
	logger  *slog.Logger
	mu      sync.RWMutex
	store   map[string]any
	session *Session
}

func newContext(ctx context.Context, args map[string]any, logger *slog.Logger) *Context {
	if args == nil {
		args = make(map[string]any)
	}
	return &Context{ctx: ctx, args: args, logger: logger, store: make(map[string]any)}
}

// forkContext creates a child Context that shares the parent's deadline/cancellation and
// shallow-copies the parent's store (auth claims, request_id, etc.) for tool/resource/prompt handlers.
func forkContext(parent *Context, args map[string]any, logger *slog.Logger) *Context {
	if args == nil {
		args = make(map[string]any)
	}
	child := &Context{
		ctx:    parent.ctx,
		args:   args,
		logger: logger,
		store:  make(map[string]any),
	}
	parent.mu.RLock()
	for k, v := range parent.store {
		child.store[k] = v
	}
	parent.mu.RUnlock()
	child.session = parent.session
	return child
}

// Context returns the underlying context.Context.
func (c *Context) Context() context.Context { return c.ctx }

// Logger returns a contextual logger.
func (c *Context) Logger() *slog.Logger { return c.logger }

// Session returns the session associated with this request (nil for stdio without session).
func (c *Context) Session() *Session { return c.session }

// Set stores a key-value pair in the context.
func (c *Context) Set(key string, val any) {
	c.mu.Lock()
	c.store[key] = val
	c.mu.Unlock()
}

// Get retrieves a value from the context store.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	v, ok := c.store[key]
	c.mu.RUnlock()
	return v, ok
}

// Args returns the raw arguments map.
func (c *Context) Args() map[string]any { return c.args }

// Bind unmarshals the raw arguments into the given struct.
func (c *Context) Bind(v any) error {
	data, err := json.Marshal(c.args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// String returns a string argument.
func (c *Context) String(key string) string {
	if v, ok := c.args[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// Int returns an integer argument.
func (c *Context) Int(key string) int {
	if v, ok := c.args[key].(float64); ok {
		return int(v)
	}
	return 0
}

// Float returns a float64 argument.
func (c *Context) Float(key string) float64 {
	v, _ := c.args[key].(float64)
	return v
}

// Bool returns a boolean argument.
func (c *Context) Bool(key string) bool {
	v, _ := c.args[key].(bool)
	return v
}

// Text builds a text result.
func (c *Context) Text(s string) *CallToolResult { return TextResult(s) }

// JSON builds a JSON text result.
func (c *Context) JSON(v any) *CallToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return TextResult(fmt.Sprint(v))
	}
	return TextResult(string(data))
}

// Error builds an error result.
func (c *Context) Error(msg string) *CallToolResult { return ErrorResult(msg) }
