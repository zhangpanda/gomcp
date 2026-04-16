package gomcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/istarshine/gomcp/internal/uid"
)

// Logger returns a middleware that logs tool calls with duration.
func Logger() Middleware {
	return func(ctx *Context, next func() error) error {
		start := time.Now()
		err := next()
		ctx.Logger().Info("tool call",
			"duration", time.Since(start).String(),
			"error", err,
		)
		return err
	}
}

// Recovery returns a middleware that recovers from panics.
func Recovery() Middleware {
	return func(ctx *Context, next func() error) error {
		defer func() {
			if r := recover(); r != nil {
				ctx.Logger().Error("panic recovered", "panic", fmt.Sprintf("%v", r))
				ctx.Set("_panic", fmt.Sprintf("internal error: %v", r))
			}
		}()
		return next()
	}
}

// RequestID injects a unique request ID into the context.
func RequestID() Middleware {
	return func(ctx *Context, next func() error) error {
		id := uid.New()
		ctx.Set("request_id", id)
		ctx.logger = ctx.logger.With("request_id", id)
		return next()
	}
}

// Timeout returns a middleware that enforces a deadline on tool execution.
func Timeout(d time.Duration) Middleware {
	return func(ctx *Context, next func() error) error {
		tctx, cancel := context.WithTimeout(ctx.ctx, d)
		defer cancel()
		ctx.ctx = tctx

		done := make(chan error, 1)
		go func() { done <- next() }()

		select {
		case err := <-done:
			return err
		case <-tctx.Done():
			return fmt.Errorf("tool execution timed out after %s", d)
		}
	}
}

// RateLimit returns a middleware that limits calls to n per minute using a token bucket.
func RateLimit(perMinute int) Middleware {
	bucket := &tokenBucket{
		tokens:   perMinute,
		max:      perMinute,
		interval: time.Minute / time.Duration(perMinute),
	}
	go bucket.refill()

	return func(ctx *Context, next func() error) error {
		if !bucket.take() {
			return fmt.Errorf("rate limit exceeded (%d/min)", perMinute)
		}
		return next()
	}
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   int
	max      int
	interval time.Duration
}

func (b *tokenBucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (b *tokenBucket) refill() {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for range ticker.C {
		b.mu.Lock()
		if b.tokens < b.max {
			b.tokens++
		}
		b.mu.Unlock()
	}
}
