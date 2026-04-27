package transport

import (
	"context"
	"net/http"
)

// LookupHeader returns a header from context-injected [CtxKey] http_headers, using
// case-insensitive matching per [http.CanonicalHeaderKey].
func LookupHeader(ctx context.Context, name string) string {
	h, ok := ctx.Value(CtxKey("http_headers")).(map[string]string)
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
