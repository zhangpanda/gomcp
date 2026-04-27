package transport

import (
	"context"
	"net/http"
	"testing"
)

func TestLookupHeader_CaseInsensitive(t *testing.T) {
	ctx := context.WithValue(
		context.Background(),
		CtxKey("http_headers"),
		map[string]string{http.CanonicalHeaderKey("X-API-Key"): "secret"},
	)
	if got := LookupHeader(ctx, "x-api-key"); got != "secret" {
		t.Fatalf("expected secret, got %q", got)
	}
	if got := LookupHeader(ctx, "X-API-Key"); got != "secret" {
		t.Fatalf("expected secret, got %q", got)
	}
}
