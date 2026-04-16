package gomcp

// Group allows organizing tools under a common prefix with shared middleware.
type Group struct {
	prefix      string
	server      *Server
	middlewares []Middleware
}

// Group creates a new tool group with a name prefix.
func (s *Server) Group(prefix string, mws ...Middleware) *Group {
	return &Group{prefix: prefix, server: s, middlewares: mws}
}

// Use adds middleware to this group.
func (g *Group) Use(mw ...Middleware) {
	g.middlewares = append(g.middlewares, mw...)
}

// Group creates a nested sub-group.
func (g *Group) Group(prefix string, mws ...Middleware) *Group {
	return &Group{
		prefix:      g.prefix + "." + prefix,
		server:      g.server,
		middlewares: append(append([]Middleware{}, g.middlewares...), mws...),
	}
}

// Tool registers a tool under this group. The tool name becomes "prefix.name".
func (g *Group) Tool(name, description string, handler HandlerFunc, opts ...ToolOption) {
	fullName := g.prefix + "." + name
	wrapped := g.wrapHandler(handler)
	g.server.Tool(fullName, description, wrapped, opts...)
}

// ToolFunc registers a typed tool under this group.
func (g *Group) ToolFunc(name, description string, fn any, opts ...ToolOption) {
	fullName := g.prefix + "." + name
	g.server.ToolFunc(fullName, description, fn, opts...)

	// now wrap the registered handler with group middleware
	if len(g.middlewares) == 0 {
		return
	}
	g.server.mu.Lock()
	defer g.server.mu.Unlock()
	// find the key (may have @version suffix)
	for key, entry := range g.server.tools {
		if key == fullName || (len(key) > len(fullName) && key[:len(fullName)+1] == fullName+"@") {
			original := entry.handler
			entry.handler = g.wrapHandlerFunc(original)
			g.server.tools[key] = entry
		}
	}
}

func (g *Group) wrapHandler(handler HandlerFunc) HandlerFunc {
	if len(g.middlewares) == 0 {
		return handler
	}
	mws := make([]Middleware, len(g.middlewares))
	copy(mws, g.middlewares)
	return func(ctx *Context) (*CallToolResult, error) {
		var result *CallToolResult
		var handlerErr error
		err := executeChain(ctx, mws, func() error {
			result, handlerErr = handler(ctx)
			return handlerErr
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}

func (g *Group) wrapHandlerFunc(handler HandlerFunc) HandlerFunc {
	return g.wrapHandler(handler)
}
