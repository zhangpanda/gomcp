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

	// Build the schema-aware entry, apply opts to discover the version
	// suffix, wrap the handler with group middleware, and register in
	// a single s.mu.Lock critical section so no client request can
	// observe the tool in its unwrapped form. The previous design
	// called s.ToolFunc() first (which took s.mu on its own), then
	// acquired s.mu a second time to replace the handler — between
	// those two locks a tools/call for this tool would bypass all
	// group middleware.
	entry := g.server.buildTypedToolEntry(fullName, description, fn)
	if len(g.middlewares) > 0 {
		entry.handler = g.wrapHandler(entry.handler)
	}
	g.server.registerTool(fullName, entry, opts)
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
