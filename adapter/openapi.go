package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/istarshine/gomcp"
	"gopkg.in/yaml.v3"
)

// OpenAPIOptions controls OpenAPI import behavior.
type OpenAPIOptions struct {
	TagFilter []string // only import operations with these tags
	ServerURL string   // base URL for API calls
	AuthToken string   // Bearer token for API calls
	NamingFunc func(operationID, method, path string) string
}

// ImportOpenAPI parses an OpenAPI 3.x file and registers each operation as an MCP Tool.
func ImportOpenAPI(s *gomcp.Server, filePath string, opts OpenAPIOptions) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read openapi file: %w", err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		// try JSON
		if err2 := json.Unmarshal(data, &spec); err2 != nil {
			return fmt.Errorf("parse openapi: %w", err)
		}
	}

	baseURL := opts.ServerURL
	if baseURL == "" && len(spec.Servers) > 0 {
		baseURL = spec.Servers[0].URL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	for path, pathItem := range spec.Paths {
		for method, op := range pathItem.Operations() {
			if !matchTags(op.Tags, opts.TagFilter) {
				continue
			}

			toolName := opToolName(op.OperationID, method, path, opts.NamingFunc)
			desc := op.Summary
			if desc == "" {
				desc = fmt.Sprintf("%s %s", method, path)
			}

			inputSchema := buildOpSchema(op, method)

			capturedMethod := method
			capturedPath := path
			capturedOp := op

			handler := func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
				return callOpenAPI(baseURL, capturedMethod, capturedPath, capturedOp, opts.AuthToken, ctx)
			}

			s.RegisterToolRaw(toolName, desc, inputSchema, handler)
		}
	}
	return nil
}

func callOpenAPI(baseURL, method, path string, op openAPIOperation, authToken string, ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
	// substitute path params
	actualPath := path
	for _, p := range op.Parameters {
		if p.In == "path" {
			actualPath = strings.ReplaceAll(actualPath, "{"+p.Name+"}", ctx.String(p.Name))
		}
	}

	// build query
	var queryParts []string
	for _, p := range op.Parameters {
		if p.In == "query" {
			if v := ctx.String(p.Name); v != "" {
				queryParts = append(queryParts, p.Name+"="+v)
			}
		}
	}

	fullURL := baseURL + actualPath
	if len(queryParts) > 0 {
		fullURL += "?" + strings.Join(queryParts, "&")
	}

	// body
	var bodyReader io.Reader
	if bodyStr := ctx.String("body"); bodyStr != "" {
		bodyReader = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequest(strings.ToUpper(method), fullURL, bodyReader)
	if err != nil {
		return gomcp.ErrorResult("request error: " + err.Error()), nil
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return gomcp.ErrorResult("http error: " + err.Error()), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return gomcp.ErrorResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))), nil
	}
	return gomcp.TextResult(string(respBody)), nil
}

func buildOpSchema(op openAPIOperation, method string) gomcp.JSONSchema {
	props := make(map[string]gomcp.JSONSchema)
	var required []string

	for _, p := range op.Parameters {
		prop := gomcp.JSONSchema{
			Type:        schemaType(p.Schema.Type),
			Description: p.Description,
		}
		props[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	if method == "post" || method == "put" || method == "patch" {
		props["body"] = gomcp.JSONSchema{Type: "string", Description: "JSON request body"}
	}

	return gomcp.JSONSchema{Type: "object", Properties: props, Required: required}
}

func opToolName(opID, method, path string, custom func(string, string, string) string) string {
	if custom != nil {
		return custom(opID, method, path)
	}
	if opID != "" {
		return opID
	}
	// fallback: method_path
	name := strings.ToLower(method)
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if strings.HasPrefix(seg, "{") {
			name += "_by_" + strings.Trim(seg, "{}")
		} else {
			name += "_" + seg
		}
	}
	return name
}

func matchTags(opTags, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, ft := range filter {
		for _, ot := range opTags {
			if strings.EqualFold(ft, ot) {
				return true
			}
		}
	}
	return false
}

func schemaType(t string) string {
	if t == "" {
		return "string"
	}
	return t
}

// Minimal OpenAPI 3.x structs (just enough for tool generation)

type openAPISpec struct {
	Paths   map[string]openAPIPathItem `json:"paths" yaml:"paths"`
	Servers []struct {
		URL string `json:"url" yaml:"url"`
	} `json:"servers" yaml:"servers"`
}

type openAPIPathItem struct {
	Get    *openAPIOperation `json:"get" yaml:"get"`
	Post   *openAPIOperation `json:"post" yaml:"post"`
	Put    *openAPIOperation `json:"put" yaml:"put"`
	Delete *openAPIOperation `json:"delete" yaml:"delete"`
	Patch  *openAPIOperation `json:"patch" yaml:"patch"`
}

func (p openAPIPathItem) Operations() map[string]openAPIOperation {
	ops := make(map[string]openAPIOperation)
	if p.Get != nil {
		ops["get"] = *p.Get
	}
	if p.Post != nil {
		ops["post"] = *p.Post
	}
	if p.Put != nil {
		ops["put"] = *p.Put
	}
	if p.Delete != nil {
		ops["delete"] = *p.Delete
	}
	if p.Patch != nil {
		ops["patch"] = *p.Patch
	}
	return ops
}

type openAPIOperation struct {
	OperationID string             `json:"operationId" yaml:"operationId"`
	Summary     string             `json:"summary" yaml:"summary"`
	Tags        []string           `json:"tags" yaml:"tags"`
	Parameters  []openAPIParameter `json:"parameters" yaml:"parameters"`
}

type openAPIParameter struct {
	Name        string          `json:"name" yaml:"name"`
	In          string          `json:"in" yaml:"in"`
	Required    bool            `json:"required" yaml:"required"`
	Description string          `json:"description" yaml:"description"`
	Schema      openAPISchema   `json:"schema" yaml:"schema"`
}

type openAPISchema struct {
	Type string `json:"type" yaml:"type"`
}
