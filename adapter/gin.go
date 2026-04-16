package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/istarshine/gomcp"
)

// ImportOptions controls which Gin routes are imported as MCP tools.
type ImportOptions struct {
	IncludePaths []string // glob-like prefixes to include (e.g. "/api/v1/*")
	ExcludePaths []string // glob-like prefixes to exclude
	NamingFunc   func(method, path string) string // custom tool name generator
}

// ImportGin scans a gin.Engine's registered routes and registers each as an MCP Tool.
// Path params become required string params; query/body are passed through.
func ImportGin(s *gomcp.Server, engine *gin.Engine, opts ImportOptions) {
	routes := engine.Routes()
	for _, r := range routes {
		if !shouldInclude(r.Path, r.Method, opts) {
			continue
		}

		toolName := ginToolName(r.Method, r.Path, opts.NamingFunc)
		desc := fmt.Sprintf("%s %s", r.Method, r.Path)
		pathParams := extractGinParams(r.Path)

		// build input schema
		props := make(map[string]gomcp.JSONSchema)
		var required []string

		for _, p := range pathParams {
			props[p] = gomcp.JSONSchema{Type: "string", Description: fmt.Sprintf("Path parameter: %s", p)}
			required = append(required, p)
		}

		// generic body/query params
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			props["body"] = gomcp.JSONSchema{Type: "string", Description: "JSON request body"}
		}
		props["query"] = gomcp.JSONSchema{Type: "string", Description: "Query string (e.g. key=val&key2=val2)"}

		inputSchema := gomcp.JSONSchema{
			Type:       "object",
			Properties: props,
			Required:   required,
		}

		// capture for closure
		method := r.Method
		path := r.Path
		params := pathParams

		handler := func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return callGinRoute(engine, method, path, params, ctx)
		}

		s.RegisterToolRaw(toolName, desc, inputSchema, handler)
	}
}

func callGinRoute(engine *gin.Engine, method, path string, pathParams []string, ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
	// substitute path params
	actualPath := path
	for _, p := range pathParams {
		val := ctx.String(p)
		actualPath = strings.Replace(actualPath, ":"+p, val, 1)
		actualPath = strings.Replace(actualPath, "*"+p, val, 1)
	}

	// build query string
	if qs := ctx.String("query"); qs != "" {
		actualPath += "?" + qs
	}

	// build request
	var bodyReader io.Reader
	if bodyStr := ctx.String("body"); bodyStr != "" {
		bodyReader = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequest(method, actualPath, bodyReader)
	if err != nil {
		return gomcp.ErrorResult("failed to create request: " + err.Error()), nil
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// execute through gin
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	resp := w.Result()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return gomcp.ErrorResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))), nil
	}

	return gomcp.TextResult(string(respBody)), nil
}

// ginToolName generates a tool name from HTTP method + path.
func ginToolName(method, path string, custom func(string, string) string) string {
	if custom != nil {
		return custom(method, path)
	}
	// default: get_api_v1_users_by_id
	name := strings.ToLower(method)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ":") || strings.HasPrefix(p, "*") {
			name += "_by_" + strings.TrimLeft(p, ":*")
		} else {
			name += "_" + p
		}
	}
	return name
}

// extractGinParams returns path parameter names from a Gin route path.
func extractGinParams(path string) []string {
	var params []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
			params = append(params, strings.TrimLeft(seg, ":*"))
		}
	}
	return params
}

func shouldInclude(path, method string, opts ImportOptions) bool {
	if len(opts.ExcludePaths) > 0 {
		for _, ex := range opts.ExcludePaths {
			if matchPrefix(path, ex) {
				return false
			}
		}
	}
	if len(opts.IncludePaths) > 0 {
		for _, inc := range opts.IncludePaths {
			if matchPrefix(path, inc) {
				return true
			}
		}
		return false
	}
	return true
}

func matchPrefix(path, pattern string) bool {
	pattern = strings.TrimSuffix(pattern, "*")
	return strings.HasPrefix(path, pattern)
}

// Helpers for URL encoding
func encodeQuery(params map[string]string) string {
	v := url.Values{}
	for k, val := range params {
		v.Set(k, val)
	}
	return v.Encode()
}

// Ensure JSON is pretty for readability
func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data)
	}
	return buf.String()
}
