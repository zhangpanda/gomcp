package gomcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/zhangpanda/gomcp/transport"
)

// auth context keys — imported from transport so both sides use one
// declaration. Previously these were string literals declared in two
// places; any drift silently broke auth header injection.
const (
	ctxKeyAuthHeader = transport.CtxKeyAuthHeader
	ctxKeyHeaders    = transport.CtxKeyHeaders
)

// WithAuthHeader injects an Authorization header value into a context.
// Used by transport layers to pass HTTP auth info to middleware.
func WithAuthHeader(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, ctxKeyAuthHeader, value)
}

// WithHeaders injects HTTP headers into a context.
func WithHeaders(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, ctxKeyHeaders, headers)
}

func getAuthHeader(ctx *Context) string {
	if v, ok := ctx.ctx.Value(ctxKeyAuthHeader).(string); ok {
		return v
	}
	// fallback: check store (for stdio mode, user can set manually)
	if v, ok := ctx.Get("_auth_header"); ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getHeader(ctx *Context, key string) string {
	if headers, ok := ctx.ctx.Value(ctxKeyHeaders).(map[string]string); ok {
		canon := http.CanonicalHeaderKey(key)
		if v, ok := headers[canon]; ok {
			return v
		}
		for k, v := range headers {
			if http.CanonicalHeaderKey(k) == canon {
				return v
			}
		}
	}
	return ""
}

// --- Auth Middleware ---

// TokenValidator validates a bearer token and returns claims (stored in context as "_auth_claims").
// Implement JWT or opaque-token checks yourself inside validate; this package does not parse JWTs.
type TokenValidator func(token string) (claims map[string]any, err error)

// BearerAuth returns a middleware that validates Bearer tokens.
func BearerAuth(validate TokenValidator) Middleware {
	return func(ctx *Context, next func() error) error {
		header := strings.TrimSpace(getAuthHeader(ctx))
		if len(header) <= 7 || !strings.EqualFold(header[:7], "bearer ") {
			return authError("missing or invalid Bearer token")
		}
		token := strings.TrimSpace(header[7:])
		if token == "" {
			return authError("missing or invalid Bearer token")
		}
		claims, err := validate(token)
		if err != nil {
			return authError("authentication failed: " + err.Error())
		}
		if claims != nil {
			ctx.Set("_auth_claims", claims)
			if roles, ok := claims["roles"]; ok {
				ctx.Set("_auth_roles", roles)
			}
			if perms, ok := claims["permissions"]; ok {
				ctx.Set("_auth_permissions", perms)
			}
		}
		return next()
	}
}

// SSEBearerAuth returns an HTTP gate suitable for [WithSSEAuth], enforcing the same Bearer rules as [BearerAuth].
func SSEBearerAuth(validate TokenValidator) func(*http.Request) error {
	return func(r *http.Request) error {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(header) <= 7 || !strings.EqualFold(header[:7], "bearer ") {
			return fmt.Errorf("missing or invalid Bearer token")
		}
		token := strings.TrimSpace(header[7:])
		if token == "" {
			return fmt.Errorf("missing or invalid Bearer token")
		}
		_, err := validate(token)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		return nil
	}
}

// SSEAPIKeyAuth returns an HTTP gate like [APIKeyAuth] but reads only the named header (no tool-argument fallback).
func SSEAPIKeyAuth(headerName string, validate KeyValidator) func(*http.Request) error {
	canon := http.CanonicalHeaderKey(headerName)
	return func(r *http.Request) error {
		key := r.Header.Get(canon)
		if key == "" {
			return fmt.Errorf("missing API key")
		}
		_, err := validate(key)
		if err != nil {
			return fmt.Errorf("invalid API key: %w", err)
		}
		return nil
	}
}

// SSEBasicAuth returns an HTTP gate enforcing the same Basic rules as [BasicAuth].
func SSEBasicAuth(validate func(username, password string) (map[string]any, error)) func(*http.Request) error {
	return func(r *http.Request) error {
		header := r.Header.Get("Authorization")
		if len(header) <= 6 || !strings.EqualFold(header[:6], "basic ") {
			return fmt.Errorf("missing Basic auth credentials")
		}
		encoded := strings.TrimSpace(header[6:])
		if encoded == "" {
			return fmt.Errorf("missing Basic auth credentials")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("invalid Basic auth encoding")
		}
		user, pass, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return fmt.Errorf("invalid Basic auth format")
		}
		_, err = validate(user, pass)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		return nil
	}
}

// BearerAuthSkipHandshake wraps [BearerAuth] so methods from [HandshakeAuthSkipMethods] do not require a Bearer token.
func BearerAuthSkipHandshake(validate TokenValidator) Middleware {
	return SkipAuthForMCPMethods(HandshakeAuthSkipMethods(), BearerAuth(validate))
}

// APIKeyAuthSkipHandshake wraps [APIKeyAuth] so methods from [HandshakeAuthSkipMethods] do not require an API key.
func APIKeyAuthSkipHandshake(headerName string, validate KeyValidator) Middleware {
	return SkipAuthForMCPMethods(HandshakeAuthSkipMethods(), APIKeyAuth(headerName, validate))
}

// BasicAuthSkipHandshake wraps [BasicAuth] so methods from [HandshakeAuthSkipMethods] do not require Basic credentials.
func BasicAuthSkipHandshake(validate func(username, password string) (map[string]any, error)) Middleware {
	return SkipAuthForMCPMethods(HandshakeAuthSkipMethods(), BasicAuth(validate))
}

// KeyValidator validates an API key and returns metadata (stored in context as "_auth_claims").
type KeyValidator func(key string) (meta map[string]any, err error)

// APIKeyAuth returns a middleware that validates API keys.
//
// The key is sourced from:
//  1. The header specified by headerName (e.g. "X-API-Key"), which is the
//     normal and recommended path.
//  2. Otherwise, the "api_key" argument merged from tools/call arguments,
//     prompts/get arguments, or resources/read params (see
//     [mergedArgsForMiddleware]).
//
// Reading the key from tool arguments is supported for MCP clients that
// cannot easily set HTTP headers, but it has a security caveat: tool
// arguments are visible to any middleware earlier in the chain (including
// [Logger], which can end up writing the key to logs). This middleware
// therefore deletes "api_key" from the request's argument map after a
// successful validation so downstream middleware and handlers do not
// observe the secret. If you must not use the argument fallback at all,
// prefer [SSEAPIKeyAuth] or a custom header-only check.
func APIKeyAuth(headerName string, validate KeyValidator) Middleware {
	return func(ctx *Context, next func() error) error {
		key := getHeader(ctx, headerName)
		fromArgs := false
		if key == "" {
			key = ctx.String("api_key")
			fromArgs = key != ""
		}
		if key == "" {
			return authError("missing API key")
		}
		meta, err := validate(key)
		if err != nil {
			return authError("invalid API key: " + err.Error())
		}
		if fromArgs {
			// Strip the key from the live argument map so Logger /
			// handler / downstream middleware never see it.
			if args := ctx.Args(); args != nil {
				delete(args, "api_key")
			}
		}
		if meta != nil {
			ctx.Set("_auth_claims", meta)
			if roles, ok := meta["roles"]; ok {
				ctx.Set("_auth_roles", roles)
			}
		}
		return next()
	}
}

// BasicAuth returns a middleware that validates HTTP Basic Authentication.
func BasicAuth(validate func(username, password string) (map[string]any, error)) Middleware {
	return func(ctx *Context, next func() error) error {
		header := getAuthHeader(ctx)
		if len(header) <= 6 || !strings.EqualFold(header[:6], "basic ") {
			return authError("missing Basic auth credentials")
		}
		encoded := strings.TrimSpace(header[6:])
		if encoded == "" {
			return authError("missing Basic auth credentials")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return authError("invalid Basic auth encoding")
		}
		user, pass, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return authError("invalid Basic auth format")
		}
		meta, err := validate(user, pass)
		if err != nil {
			return authError("authentication failed: " + err.Error())
		}
		if meta != nil {
			ctx.Set("_auth_claims", meta)
			if roles, ok := meta["roles"]; ok {
				ctx.Set("_auth_roles", roles)
			}
		}
		ctx.Set("_auth_user", user)
		return next()
	}
}

// --- Authorization Middleware ---

// RequireRole returns a middleware that checks if the authenticated user has the required role.
// Roles are read from ctx store key "_auth_roles" (set by auth middleware).
func RequireRole(role string) Middleware {
	return func(ctx *Context, next func() error) error {
		roles, ok := ctx.Get("_auth_roles")
		if !ok {
			return authError("access denied: no roles")
		}
		if !containsRole(roles, role) {
			return authError("access denied: requires role " + role)
		}
		return next()
	}
}

// RequirePermission returns a middleware that checks if the user has the required permission.
func RequirePermission(perm string) Middleware {
	return func(ctx *Context, next func() error) error {
		perms, ok := ctx.Get("_auth_permissions")
		if !ok {
			return authError("access denied: no permissions")
		}
		if !containsRole(perms, perm) {
			return authError("access denied: requires permission " + perm)
		}
		return next()
	}
}

func containsRole(v any, target string) bool {
	switch roles := v.(type) {
	case []string:
		for _, r := range roles {
			if r == target {
				return true
			}
		}
	case []any:
		for _, r := range roles {
			if fmt.Sprintf("%v", r) == target {
				return true
			}
		}
	}
	return false
}

func authError(msg string) error {
	return fmt.Errorf("auth: %s", msg)
}
