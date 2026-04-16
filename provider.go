package gomcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DirOptions configures directory-based tool loading.
type DirOptions struct {
	Watch    bool          // watch for file changes
	Interval time.Duration // poll interval (default 2s)
	Pattern  string        // glob pattern (default "*.tool.yaml")
	OnReload func()        // callback after reload
}

// toolDef is the YAML tool definition format.
type toolDef struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Version     string       `yaml:"version"`
	Params      []paramDef   `yaml:"params"`
	Handler     string       `yaml:"handler"` // HTTP URL to forward to
	Method      string       `yaml:"method"`  // HTTP method (default GET)
}

type paramDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// LoadDir loads tool definitions from YAML files in a directory.
func (s *Server) LoadDir(dir string, opts DirOptions) error {
	if opts.Pattern == "" {
		opts.Pattern = "*.tool.yaml"
	}
	if opts.Interval == 0 {
		opts.Interval = 2 * time.Second
	}

	if err := s.loadToolFiles(dir, opts.Pattern); err != nil {
		return err
	}

	if opts.Watch {
		go s.watchDir(dir, opts)
	}
	return nil
}

func (s *Server) loadToolFiles(dir, pattern string) error {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return fmt.Errorf("glob %s: %w", pattern, err)
	}

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			s.logger.Warn("skip tool file", "path", path, "error", err)
			continue
		}
		var def toolDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			s.logger.Warn("skip tool file", "path", path, "error", err)
			continue
		}
		s.registerToolDef(def)
	}
	return nil
}

func (s *Server) registerToolDef(def toolDef) {
	props := make(map[string]JSONSchema)
	var required []string
	for _, p := range def.Params {
		props[p.Name] = JSONSchema{
			Type:        paramType(p.Type),
			Description: p.Description,
		}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	inputSchema := JSONSchema{Type: "object", Properties: props, Required: required}

	method := strings.ToUpper(def.Method)
	if method == "" {
		method = "GET"
	}
	handlerURL := def.Handler

	handler := func(ctx *Context) (*CallToolResult, error) {
		return callHTTPHandler(handlerURL, method, ctx)
	}

	var opts []ToolOption
	if def.Version != "" {
		opts = append(opts, Version(def.Version))
	}
	s.RegisterToolRaw(def.Name, def.Description, inputSchema, handler, opts...)
}

func callHTTPHandler(url, method string, ctx *Context) (*CallToolResult, error) {
	var bodyReader io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		data, _ := json.Marshal(ctx.Args())
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx.Context(), method, url, bodyReader)
	if err != nil {
		return ErrorResult("request error: " + err.Error()), nil
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// append query params for GET
	if method == "GET" {
		q := req.URL.Query()
		for k, v := range ctx.Args() {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ErrorResult("http error: " + err.Error()), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return ErrorResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))), nil
	}
	return TextResult(string(body)), nil
}

func (s *Server) watchDir(dir string, opts DirOptions) {
	// simple polling watcher — tracks file mod times
	modTimes := make(map[string]time.Time)
	snapshot := func() map[string]time.Time {
		m := make(map[string]time.Time)
		matches, _ := filepath.Glob(filepath.Join(dir, opts.Pattern))
		for _, path := range matches {
			if info, err := os.Stat(path); err == nil {
				m[path] = info.ModTime()
			}
		}
		return m
	}
	modTimes = snapshot()

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for range ticker.C {
		current := snapshot()
		changed := len(current) != len(modTimes)
		if !changed {
			for k, v := range current {
				if old, ok := modTimes[k]; !ok || !old.Equal(v) {
					changed = true
					break
				}
			}
		}
		if changed {
			s.logger.Info("tool files changed, reloading", "dir", dir)
			s.loadToolFiles(dir, opts.Pattern)
			modTimes = current
			if opts.OnReload != nil {
				opts.OnReload()
			}
		}
	}
}

func paramType(t string) string {
	switch strings.ToLower(t) {
	case "int", "integer":
		return "integer"
	case "float", "number":
		return "number"
	case "bool", "boolean":
		return "boolean"
	default:
		return "string"
	}
}
