package gomcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zhangpanda/gomcp/internal/uid"
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
	return func(ctx *Context, next func() error) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("internal error: %v", r)
				ctx.Logger().Error("panic recovered", "panic", fmt.Sprintf("%v", r))
				ctx.Set("_panic", msg)
				retErr = fmt.Errorf("%s", msg)
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
// The request context is replaced with a timeout context; handlers should
// use [Context.Context] and check [context.Context.Done] to cancel work
// so long calls return promptly when the deadline hits.
func Timeout(d time.Duration) Middleware {
	return func(ctx *Context, next func() error) error {
		tctx, cancel := context.WithTimeout(ctx.ctx, d)
		defer cancel()
		ctx.ctx = tctx
		err := next()
		// Unwrap to stable message for tool clients (and tests).
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("tool execution timed out after %s", d)
		}
		return err
	}
}

// RateLimit returns a middleware that limits calls to n per minute using a token bucket.
func RateLimit(perMinute int) Middleware {
	bucket := &tokenBucket{
		tokens:   float64(perMinute),
		max:      float64(perMinute),
		rate:     float64(perMinute) / 60.0, // tokens per second
		lastTime: time.Now(),
	}

	return func(ctx *Context, next func() error) error {
		if !bucket.take() {
			return fmt.Errorf("rate limit exceeded (%d/min)", perMinute)
		}
		return next()
	}
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64 // tokens per second
	lastTime time.Time
}

func (b *tokenBucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.tokens += now.Sub(b.lastTime).Seconds() * b.rate
	b.lastTime = now
	if b.tokens > b.max {
		b.tokens = b.max
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
