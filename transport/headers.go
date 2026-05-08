package transport

import (
	"context"
	"net/http"
)

// CtxKey is the context key type used to inject HTTP metadata into
// handlers. It is an unexported string type so external packages cannot
// accidentally collide with these keys.
type CtxKey string

// Canonical context keys used by the HTTP transport and consumed by
// middleware in the parent package. Keep these in one place so the two
// sides cannot drift.
const (
	// CtxKeyAuthHeader holds the raw Authorization header value (string).
	CtxKeyAuthHeader CtxKey = "auth_header"
	// CtxKeyHeaders holds the full HTTP header map, keys canonicalised
	// via [http.CanonicalHeaderKey] (map[string]string).
	CtxKeyHeaders CtxKey = "http_headers"
	// CtxKeySessionIDSink holds a pointer to a [SessionIDSink] that lets
	// middleware/handlers advertise a server-assigned MCP session ID
	// back to the transport so it can be written to the response header.
	CtxKeySessionIDSink CtxKey = "session_id_sink"
)

// SessionIDSink is a mutable holder the HTTP transport attaches to the
// request context before dispatch. Handlers that own/create a session
// write the ID via [SessionIDSink.Set]; after dispatch the transport
// reads the value via [SessionIDSink.Get] and emits it as the
// Mcp-Session-Id response header.
//
// The zero value is ready for use. Value is goroutine-safe for a single
// producer (the handler for a given request) and a single consumer (the
// transport, after the handler returns).
type SessionIDSink struct{ id string }

// Set stores the session ID to advertise on the HTTP response.
func (s *SessionIDSink) Set(id string) {
	if s == nil {
		return
	}
	s.id = id
}

// Get returns the stored session ID, or an empty string if none.
func (s *SessionIDSink) Get() string {
	if s == nil {
		return ""
	}
	return s.id
}

// WithSessionIDSink attaches a fresh [SessionIDSink] to ctx and returns
// both the new context and the sink. Handlers read the sink via
// [SessionIDFromContext].
func WithSessionIDSink(ctx context.Context) (context.Context, *SessionIDSink) {
	sink := &SessionIDSink{}
	return context.WithValue(ctx, CtxKeySessionIDSink, sink), sink
}

// SessionIDFromContext returns the sink attached by [WithSessionIDSink],
// or nil if none is attached (e.g. stdio transport or a custom transport
// that does not use session IDs).
func SessionIDFromContext(ctx context.Context) *SessionIDSink {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(CtxKeySessionIDSink).(*SessionIDSink); ok {
		return s
	}
	return nil
}

// LookupHeader returns a header value from a context populated via
// [CtxKeyHeaders], matching name case-insensitively per
// [http.CanonicalHeaderKey].
func LookupHeader(ctx context.Context, name string) string {
	h, ok := ctx.Value(CtxKeyHeaders).(map[string]string)
	if !ok {
		return ""
	}
	if v, ok := h[http.CanonicalHeaderKey(name)]; ok {
		return v
	}
	// Rare: map not built with canonical keys
	for k, v := range h {
		if http.CanonicalHeaderKey(k) == http.CanonicalHeaderKey(name) {
			return v
		}
	}
	return ""
}
