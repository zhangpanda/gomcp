package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/zhangpanda/gomcp"
	"gopkg.in/yaml.v3"
)

// OpenAPIOptions controls OpenAPI import behavior.
type OpenAPIOptions struct {
	TagFilter  []string // only import operations with these tags
	ServerURL  string   // base URL for API calls
	AuthToken  string   // Bearer token for API calls
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

			inputSchema := buildOpSchema(op, method, &spec)

			capturedMethod := method
			capturedPath := path
			capturedOp := op

			handler := func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
				return callOpenAPI(baseURL, capturedMethod, capturedPath, capturedOp, &spec, opts.AuthToken, ctx)
			}

			s.RegisterToolRaw(toolName, desc, inputSchema, handler)
		}
	}
	return nil
}

func callOpenAPI(baseURL, method, path string, op openAPIOperation, spec *openAPISpec, authToken string, ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
	actualPath := path
	for _, p := range op.Parameters {
		p = spec.resolveParam(p)
		if p.In == "path" {
			actualPath = strings.ReplaceAll(actualPath, "{"+p.Name+"}", ctx.String(p.Name))
		}
	}

	var queryParams url.Values
	for _, p := range op.Parameters {
		p = spec.resolveParam(p)
		if p.In == "query" {
			if v := ctx.String(p.Name); v != "" {
				if queryParams == nil {
					queryParams = make(url.Values)
				}
				queryParams.Set(p.Name, v)
			}
		}
	}

	fullURL := baseURL + actualPath
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	// build body from requestBody schema fields or raw "body" param
	var bodyReader io.Reader
	if op.RequestBody != nil {
		bodySchema := spec.resolveSchema(op.RequestBody.jsonSchema(spec))
		if bodySchema.Type == "object" && len(bodySchema.Properties) > 0 {
			bodyMap := make(map[string]any)
			for name, propSchema := range bodySchema.Properties {
				if v := ctx.String(name); v != "" {
					bodyMap[name] = coerceValue(v, spec.resolveSchema(propSchema).Type)
				}
			}
			if len(bodyMap) > 0 {
				data, _ := json.Marshal(bodyMap)
				bodyReader = bytes.NewReader(data)
			}
		}
	}
	if bodyReader == nil {
		if bodyStr := ctx.String("body"); bodyStr != "" {
			bodyReader = bytes.NewBufferString(bodyStr)
		}
	}

	req, err := http.NewRequestWithContext(ctx.Context(), strings.ToUpper(method), fullURL, bodyReader)
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

func buildOpSchema(op openAPIOperation, method string, spec *openAPISpec) gomcp.JSONSchema {
	props := make(map[string]gomcp.JSONSchema)
	var required []string

	for _, p := range op.Parameters {
		p = spec.resolveParam(p)
		s := spec.resolveSchema(p.Schema)
		prop := gomcp.JSONSchema{
			Type:        schemaType(s.Type),
			Description: p.Description,
		}
		if len(s.Enum) > 0 {
			for _, e := range s.Enum {
				prop.Enum = append(prop.Enum, e)
			}
		}
		props[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	// extract requestBody schema properties as individual tool params
	if op.RequestBody != nil {
		bodySchema := spec.resolveSchema(op.RequestBody.jsonSchema(spec))
		if bodySchema.Type == "object" && len(bodySchema.Properties) > 0 {
			for name, propSchema := range bodySchema.Properties {
				ps := spec.resolveSchema(propSchema)
				prop := gomcp.JSONSchema{
					Type:        schemaType(ps.Type),
					Description: ps.Description,
				}
				if len(ps.Enum) > 0 {
					for _, e := range ps.Enum {
						prop.Enum = append(prop.Enum, e)
					}
				}
				props[name] = prop
			}
			required = append(required, bodySchema.Required...)
		} else {
			// fallback: raw body string
			props["body"] = gomcp.JSONSchema{Type: "string", Description: "JSON request body"}
		}
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

// --- OpenAPI 3.x structs with $ref support ---

type openAPISpec struct {
	Paths      map[string]openAPIPathItem `json:"paths" yaml:"paths"`
	Servers    []struct {
		URL string `json:"url" yaml:"url"`
	} `json:"servers" yaml:"servers"`
	Components openAPIComponents `json:"components" yaml:"components"`
}

// resolveSchema follows $ref to return the concrete schema.
func (s *openAPISpec) resolveSchema(schema openAPISchema) openAPISchema {
	for i := 0; i < 10 && schema.Ref != ""; i++ { // depth limit
		resolved, ok := s.lookupRef(schema.Ref)
		if !ok {
			break
		}
		schema = resolved
	}
	// resolve property refs
	if len(schema.Properties) > 0 {
		resolved := make(map[string]openAPISchema, len(schema.Properties))
		for k, v := range schema.Properties {
			resolved[k] = s.resolveSchema(v)
		}
		schema.Properties = resolved
	}
	// resolve items ref
	if schema.Items != nil {
		r := s.resolveSchema(*schema.Items)
		schema.Items = &r
	}
	return schema
}

// resolveParam follows $ref on parameters.
func (s *openAPISpec) resolveParam(p openAPIParameter) openAPIParameter {
	for i := 0; i < 10 && p.Ref != ""; i++ {
		name := refName(p.Ref)
		if rp, ok := s.Components.Parameters[name]; ok {
			p = rp
		} else {
			break
		}
	}
	return p
}

func (s *openAPISpec) lookupRef(ref string) (openAPISchema, bool) {
	name := refName(ref)
	if schema, ok := s.Components.Schemas[name]; ok {
		return schema, true
	}
	return openAPISchema{}, false
}

// refName extracts the name from "#/components/schemas/Foo" → "Foo"
func refName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

type openAPIComponents struct {
	Schemas    map[string]openAPISchema    `json:"schemas" yaml:"schemas"`
	Parameters map[string]openAPIParameter `json:"parameters" yaml:"parameters"`
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
	OperationID string              `json:"operationId" yaml:"operationId"`
	Summary     string              `json:"summary" yaml:"summary"`
	Tags        []string            `json:"tags" yaml:"tags"`
	Parameters  []openAPIParameter  `json:"parameters" yaml:"parameters"`
	RequestBody *openAPIRequestBody `json:"requestBody" yaml:"requestBody"`
}

type openAPIParameter struct {
	Ref         string        `json:"$ref" yaml:"$ref"`
	Name        string        `json:"name" yaml:"name"`
	In          string        `json:"in" yaml:"in"`
	Required    bool          `json:"required" yaml:"required"`
	Description string        `json:"description" yaml:"description"`
	Schema      openAPISchema `json:"schema" yaml:"schema"`
}

type openAPIRequestBody struct {
	Description string                          `json:"description" yaml:"description"`
	Required    bool                            `json:"required" yaml:"required"`
	Content     map[string]openAPIMediaType     `json:"content" yaml:"content"`
}

func (rb *openAPIRequestBody) jsonSchema(spec *openAPISpec) openAPISchema {
	if rb == nil {
		return openAPISchema{}
	}
	if mt, ok := rb.Content["application/json"]; ok {
		return mt.Schema
	}
	// fallback: try first content type
	for _, mt := range rb.Content {
		return mt.Schema
	}
	return openAPISchema{}
}

type openAPIMediaType struct {
	Schema openAPISchema `json:"schema" yaml:"schema"`
}

type openAPISchema struct {
	Ref         string                     `json:"$ref" yaml:"$ref"`
	Type        string                     `json:"type" yaml:"type"`
	Description string                     `json:"description" yaml:"description"`
	Properties  map[string]openAPISchema   `json:"properties" yaml:"properties"`
	Required    []string                   `json:"required" yaml:"required"`
	Items       *openAPISchema             `json:"items" yaml:"items"`
	Enum        []string                   `json:"enum" yaml:"enum"`
}

// coerceValue converts a string value to the appropriate Go type based on JSON Schema type.
func coerceValue(v, schemaType string) any {
	switch schemaType {
	case "integer":
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	case "number":
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	case "boolean":
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return v
}
