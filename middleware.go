package gomcp

// HandshakeAuthSkipMethods returns MCP JSON-RPC methods that commonly omit credentials on Streamable HTTP before authenticated calls.
// Pass the result to [SkipAuthForMCPMethods] or use helpers such as [BearerAuthSkipHandshake].
func HandshakeAuthSkipMethods() []string {
	return []string{"initialize", "ping"}
}

// SkipAuthForMCPMethods runs mw unless the current MCP method on ctx (store key "_mcp_method") is in skip.
// Place auth middleware inside this wrapper so handshake calls can omit credentials without weakening other methods.
func SkipAuthForMCPMethods(skip []string, mw Middleware) Middleware {
	set := make(map[string]struct{}, len(skip))
	for _, m := range skip {
		set[m] = struct{}{}
	}
	return func(ctx *Context, next func() error) error {
		method := ""
		if v, ok := ctx.Get("_mcp_method"); ok {
			if s, ok := v.(string); ok {
				method = s
			}
		}
		if _, ok := set[method]; ok {
			return next()
		}
		return mw(ctx, next)
	}
}

// Middleware processes a request and calls next to continue the chain.
type Middleware func(ctx *Context, next func() error) error

// Use adds global middleware to the server.
func (s *Server) Use(mw ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = append(s.middlewares, mw...)
}

// executeChain runs the middleware chain then the final handler.
func executeChain(ctx *Context, mws []Middleware, handler func() error) error {
	if len(mws) == 0 {
		return handler()
	}
	return mws[0](ctx, func() error {
		return executeChain(ctx, mws[1:], handler)
	})
}
