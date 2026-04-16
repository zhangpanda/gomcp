package gomcp

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
