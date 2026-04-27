package gomcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/zhangpanda/gomcp/transport"
)

// auth context keys
const (
	ctxKeyAuthHeader transport.CtxKey = "auth_header"
	ctxKeyHeaders    transport.CtxKey = "http_headers"
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
type TokenValidator func(token string) (claims map[string]any, err error)

// BearerAuth returns a middleware that validates Bearer tokens.
func BearerAuth(validate TokenValidator) Middleware {
	return func(ctx *Context, next func() error) error {
		header := getAuthHeader(ctx)
		token := strings.TrimPrefix(header, "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")
		if token == "" || token == header {
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

// KeyValidator validates an API key and returns metadata (stored in context as "_auth_claims").
type KeyValidator func(key string) (meta map[string]any, err error)

// APIKeyAuth returns a middleware that validates API keys.
// It checks the header specified by headerName (e.g. "X-API-Key"),
// falling back to the "api_key" tool argument.
func APIKeyAuth(headerName string, validate KeyValidator) Middleware {
	return func(ctx *Context, next func() error) error {
		key := getHeader(ctx, headerName)
		if key == "" {
			key = ctx.String("api_key")
		}
		if key == "" {
			return authError("missing API key")
		}
		meta, err := validate(key)
		if err != nil {
			return authError("invalid API key: " + err.Error())
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
		encoded := strings.TrimPrefix(header, "Basic ")
		encoded = strings.TrimPrefix(encoded, "basic ")
		if encoded == "" || encoded == header {
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
