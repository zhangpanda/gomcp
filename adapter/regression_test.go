package adapter_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/adapter"
	"github.com/zhangpanda/gomcp/mcptest"
)

// BUG O1: OpenAPI / gin path parameters used to be substituted into the
// URL verbatim. A value like "foo/bar" rewrote the path segmentation
// and a value with reserved characters could even reach a different
// endpoint. Path params must be URL-escaped.
func TestRegression_GinPathParamURLEscaped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// Route on the raw (still-escaped) URL so a path-param value that
	// contains '/' stays in one segment. Without this, gin decodes
	// "%2F" back to "/" before routing and the request 404s.
	engine.UseRawPath = true
	engine.UnescapePathValues = true
	var capturedID string
	engine.GET("/items/:id", func(c *gin.Context) {
		capturedID = c.Param("id")
		c.String(http.StatusOK, "ok")
	})

	s := gomcp.New("t", "1.0")
	adapter.ImportGin(s, engine, adapter.ImportOptions{})
	client := mcptest.NewClient(t, s)

	for _, tc := range []string{"foo/bar", "a b", "a?b=c"} {
		capturedID = ""
		res := client.CallTool("get_items_by_id", map[string]any{"id": tc})
		if res.IsError() {
			t.Fatalf("%q: expected success, got error: %s", tc, res.Text())
		}
		if !strings.Contains(res.Text(), "ok") {
			t.Fatalf("%q: expected body 'ok', got %q", tc, res.Text())
		}
		// Gin decodes URL-escaped segments, so the handler should see
		// the original pre-escape value.
		if capturedID != tc {
			t.Fatalf("%q: handler received %q — path-escape leaked or over-escaped", tc, capturedID)
		}
	}
}

// BUG O3 (gin): a missing required path parameter used to produce a
// malformed URL like "/items/"; the adapter now errors explicitly.
func TestRegression_GinPathParamMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/items/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "hit")
	})

	s := gomcp.New("t", "1.0")
	adapter.ImportGin(s, engine, adapter.ImportOptions{})
	client := mcptest.NewClient(t, s)

	res := client.CallTool("get_items_by_id", map[string]any{})
	if !res.IsError() {
		t.Fatalf("missing path param should error, got success: %s", res.Text())
	}
	if !strings.Contains(res.Text(), "missing") || !strings.Contains(res.Text(), "id") {
		t.Fatalf("expected message about missing 'id', got: %s", res.Text())
	}
}

// BUG O1 (OpenAPI): same path-escape issue existed in ImportOpenAPI.
func TestRegression_OpenAPIPathParamURLEscaped(t *testing.T) {
	var recordedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordedPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	specPath := writeTempYAML(t, `openapi: 3.0.0
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
`)

	s := gomcp.New("t", "1.0")
	if err := adapter.ImportOpenAPI(s, specPath, adapter.OpenAPIOptions{ServerURL: upstream.URL}); err != nil {
		t.Fatal(err)
	}
	client := mcptest.NewClient(t, s)

	res := client.CallTool("getItem", map[string]any{"id": "foo/bar"})
	if res.IsError() {
		t.Fatalf("call should succeed, got error: %s", res.Text())
	}
	want := "/items/" + url.PathEscape("foo/bar")
	if recordedPath != want {
		t.Fatalf("expected upstream path %q, got %q — path-escape regressed", want, recordedPath)
	}
}

// BUG O3 (OpenAPI): missing required path parameter should error.
func TestRegression_OpenAPIPathParamMissing(t *testing.T) {
	specPath := writeTempYAML(t, `openapi: 3.0.0
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
`)

	s := gomcp.New("t", "1.0")
	if err := adapter.ImportOpenAPI(s, specPath, adapter.OpenAPIOptions{ServerURL: "http://example.invalid"}); err != nil {
		t.Fatal(err)
	}
	client := mcptest.NewClient(t, s)

	res := client.CallTool("getItem", map[string]any{})
	if !res.IsError() {
		t.Fatalf("missing path param should error, got success: %s", res.Text())
	}
	if !strings.Contains(res.Text(), "missing") || !strings.Contains(res.Text(), "id") {
		t.Fatalf("expected message about missing 'id', got: %s", res.Text())
	}
}

// --- helpers ---

func writeTempYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
