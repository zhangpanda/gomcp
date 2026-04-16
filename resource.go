package gomcp

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ResourceHandler returns content for a resource.
type ResourceHandler func(ctx *Context) (any, error)

type resourceEntry struct {
	info    ResourceInfo
	handler ResourceHandler
}

type resourceTemplateEntry struct {
	info    ResourceTemplateInfo
	handler ResourceHandler
	regex   *regexp.Regexp
	params  []string
}

// MCP Resource types

// ResourceInfo describes a static resource.
type ResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ResourceTemplateInfo describes a dynamic resource with a URI template.
type ResourceTemplateInfo struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ResourceContents holds the content returned by a resource read.
type ResourceContents struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ResourceListResult is the response to resources/list.
type ResourceListResult struct {
	Resources []ResourceInfo `json:"resources"`
}

// ResourceTemplateListResult is the response to resources/templates/list.
type ResourceTemplateListResult struct {
	ResourceTemplates []ResourceTemplateInfo `json:"resourceTemplates"`
}

// ReadResourceParams is the request parameters for resources/read.
type ReadResourceParams struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the response to resources/read.
type ReadResourceResult struct {
	Contents []ResourceContents `json:"contents"`
}

// Resource registers a static resource.
func (s *Server) Resource(uri, name string, handler ResourceHandler, opts ...func(*ResourceInfo)) {
	info := ResourceInfo{URI: uri, Name: name}
	for _, o := range opts {
		o(&info)
	}
	s.mu.Lock()
	s.resources = append(s.resources, resourceEntry{info: info, handler: handler})
	s.mu.Unlock()
	s.notify("notifications/resources/list_changed")
}

// ResourceTemplate registers a dynamic resource with URI template (e.g. "db://{table}/{id}").
func (s *Server) ResourceTemplate(uriTemplate, name string, handler ResourceHandler, opts ...func(*ResourceTemplateInfo)) {
	info := ResourceTemplateInfo{URITemplate: uriTemplate, Name: name}
	for _, o := range opts {
		o(&info)
	}

	// extract param names and build matching regex
	paramRe := regexp.MustCompile(`\{(\w+)\}`)
	params := []string{}
	for _, m := range paramRe.FindAllStringSubmatch(uriTemplate, -1) {
		params = append(params, m[1])
	}
	regexStr := "^" + paramRe.ReplaceAllString(uriTemplate, `([^/]+)`) + "$"

	s.mu.Lock()
	s.resourceTemplates = append(s.resourceTemplates, resourceTemplateEntry{
		info:    info,
		handler: handler,
		regex:   regexp.MustCompile(regexStr),
		params:  params,
	})
	s.mu.Unlock()
	s.notify("notifications/resources/list_changed")
}

// Resource option helpers

// WithResourceDescription sets the description for a resource.
func WithResourceDescription(desc string) func(*ResourceInfo) {
	return func(r *ResourceInfo) { r.Description = desc }
}

// WithMIMEType sets the MIME type for a resource.
func WithMIMEType(mime string) func(*ResourceInfo) {
	return func(r *ResourceInfo) { r.MIMEType = mime }
}

// WithTemplateDescription sets the description for a resource template.
func WithTemplateDescription(desc string) func(*ResourceTemplateInfo) {
	return func(r *ResourceTemplateInfo) { r.Description = desc }
}

// WithTemplateMIMEType sets the MIME type for a resource template.
func WithTemplateMIMEType(mime string) func(*ResourceTemplateInfo) {
	return func(r *ResourceTemplateInfo) { r.MIMEType = mime }
}

// handlers

func (s *Server) handleResourcesList(msg *jsonrpcMessage) *jsonrpcMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	resources := make([]ResourceInfo, len(s.resources))
	for i, r := range s.resources {
		resources[i] = r.info
	}
	return newResponse(msg.ID, ResourceListResult{Resources: resources})
}

func (s *Server) handleResourceTemplatesList(msg *jsonrpcMessage) *jsonrpcMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	templates := make([]ResourceTemplateInfo, len(s.resourceTemplates))
	for i, t := range s.resourceTemplates {
		templates[i] = t.info
	}
	return newResponse(msg.ID, ResourceTemplateListResult{ResourceTemplates: templates})
}

func (s *Server) handleResourcesRead(msg *jsonrpcMessage) *jsonrpcMessage {
	var params ReadResourceParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// try static resources
	for _, r := range s.resources {
		if r.info.URI == params.URI {
			return s.execResource(msg, r.handler, r.info.MIMEType, params.URI, nil)
		}
	}

	// try templates
	for _, t := range s.resourceTemplates {
		if matches := t.regex.FindStringSubmatch(params.URI); matches != nil {
			args := make(map[string]any)
			for i, name := range t.params {
				if i+1 < len(matches) {
					args[name] = matches[i+1]
				}
			}
			return s.execResource(msg, t.handler, t.info.MIMEType, params.URI, args)
		}
	}

	return newErrorResponse(msg.ID, -32002, "resource not found: "+params.URI)
}

func (s *Server) execResource(msg *jsonrpcMessage, handler ResourceHandler, mime, uri string, args map[string]any) *jsonrpcMessage {
	ctx := newContext(s.ctx(), args, s.logger)
	result, err := handler(ctx)
	if err != nil {
		return newErrorResponse(msg.ID, -32603, err.Error())
	}

	text := ""
	switch v := result.(type) {
	case string:
		text = v
	default:
		data, _ := json.MarshalIndent(v, "", "  ")
		text = string(data)
	}

	if mime == "" {
		if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
			mime = "application/json"
		} else {
			mime = "text/plain"
		}
	}

	return newResponse(msg.ID, ReadResourceResult{
		Contents: []ResourceContents{{URI: uri, MIMEType: mime, Text: text}},
	})
}
