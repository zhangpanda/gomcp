package transport

import (
	"net/http"
	"strings"
)

// WrapCORS wraps h for browser clients when allowedOrigins is non-empty.
// Only listed Origin values receive Access-Control-Allow-Origin (never "*"), so credentials stay predictable.
// OPTIONS requests get a 204 with allowed methods and common MCP headers; pass empty allowedOrigins to disable CORS.
func WrapCORS(h http.Handler, allowedOrigins []string) http.Handler {
	if len(allowedOrigins) == 0 {
		return h
	}
	set := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = struct{}{}
		}
	}
	if len(set) == 0 {
		return h
	}

	const allowHeaders = "Authorization, Content-Type, Mcp-Session-Id, X-API-Key, X-Api-Key"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			if _, ok := set[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}
