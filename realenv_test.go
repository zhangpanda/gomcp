package gomcp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
)

// --- watchDir: file change triggers reload ---

func TestProvider_WatchDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping watch test in short mode")
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.tool.yaml"), []byte(`
name: tool_a
description: Tool A
params:
  - name: x
    type: string
handler: http://localhost:1/noop
`), 0o644)

	reloaded := make(chan struct{}, 1)
	s := gomcp.New("t", "1.0")
	s.LoadDir(dir, gomcp.DirOptions{
		Watch:    true,
		Interval: 100 * time.Millisecond,
		OnReload: func() {
			select {
			case reloaded <- struct{}{}:
			default:
			}
		},
	})

	// verify initial load
	resp := callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "tool_a") {
		t.Fatalf("initial load failed: %s", string(resp))
	}

	// write a new file — sleep first to ensure modtime differs from snapshot
	time.Sleep(200 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "b.tool.yaml"), []byte(`
name: tool_b
description: Tool B
params: []
handler: http://localhost:1/noop
`), 0o644)

	// wait for reload
	select {
	case <-reloaded:
	case <-time.After(5 * time.Second):
		t.Fatal("watchDir did not trigger reload")
	}

	resp = callMethod(t, s, "tools/list", map[string]any{})
	if !strings.Contains(string(resp), "tool_b") {
		t.Errorf("new tool not loaded after watch: %s", string(resp))
	}
}

// --- callHTTPHandler: provider tool calls real HTTP ---

func TestProvider_CallHTTPHandler(t *testing.T) {
	// start a mock HTTP server
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			q := r.URL.Query().Get("name")
			fmt.Fprintf(w, "hello %s", q)
		} else {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			fmt.Fprintf(w, "got %v", body["name"])
		}
	}))
	defer mock.Close()

	dir := t.TempDir()

	// GET tool
	os.WriteFile(filepath.Join(dir, "get.tool.yaml"), []byte(fmt.Sprintf(`
name: greet_get
description: Greet via GET
params:
  - name: name
    type: string
    required: true
handler: %s/greet
method: GET
`, mock.URL)), 0o644)

	// POST tool
	os.WriteFile(filepath.Join(dir, "post.tool.yaml"), []byte(fmt.Sprintf(`
name: greet_post
description: Greet via POST
params:
  - name: name
    type: string
    required: true
handler: %s/greet
method: POST
`, mock.URL)), 0o644)

	s := gomcp.New("t", "1.0")
	if err := s.LoadDir(dir, gomcp.DirOptions{}); err != nil {
		t.Fatal(err)
	}

	// test GET
	text, isErr := callRaw(t, s, "greet_get", map[string]any{"name": "alice"})
	if isErr {
		t.Fatalf("GET error: %s", text)
	}
	if !strings.Contains(text, "hello alice") {
		t.Errorf("GET: expected 'hello alice', got: %s", text)
	}

	// test POST
	text, isErr = callRaw(t, s, "greet_post", map[string]any{"name": "bob"})
	if isErr {
		t.Fatalf("POST error: %s", text)
	}
	if !strings.Contains(text, "got bob") {
		t.Errorf("POST: expected 'got bob', got: %s", text)
	}
}

// --- OpenTelemetry middleware: runs with noop provider ---

func TestOTel_NoopProvider(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.OpenTelemetry()) // uses global noop provider
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("pong"), nil
	})
	text, isErr := callRaw(t, s, "ping", nil)
	if isErr || text != "pong" {
		t.Errorf("otel noop: got %s, isErr=%v", text, isErr)
	}
}

// --- Dev() Inspector: start, request, stop ---

func TestDev_Inspector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Dev test in short mode")
	}

	s := gomcp.New("t", "1.0")
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("pong"), nil
	})
	s.Resource("test://x", "X", func(ctx *gomcp.Context) (any, error) { return "x", nil })
	s.Prompt("p", "P", nil, func(ctx *gomcp.Context) ([]gomcp.PromptMessage, error) {
		return []gomcp.PromptMessage{gomcp.UserMsg("hi")}, nil
	})

	// start Dev in background
	errCh := make(chan error, 1)
	go func() { errCh <- s.Dev("127.0.0.1:19877") }()
	time.Sleep(300 * time.Millisecond)

	base := "http://127.0.0.1:19877"

	// test inspector HTML page
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// test /api/tools
	resp, _ = http.Get(base + "/api/tools")
	var tools []map[string]any
	json.NewDecoder(resp.Body).Decode(&tools)
	resp.Body.Close()
	if len(tools) != 1 {
		t.Errorf("/api/tools: expected 1, got %d", len(tools))
	}

	// test /api/resources
	resp, _ = http.Get(base + "/api/resources")
	var res map[string]any
	json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	resources, _ := res["resources"].([]any)
	if len(resources) != 1 {
		t.Errorf("/api/resources: expected 1, got %d", len(resources))
	}

	// test /api/prompts
	resp, _ = http.Get(base + "/api/prompts")
	var prompts []map[string]any
	json.NewDecoder(resp.Body).Decode(&prompts)
	resp.Body.Close()
	if len(prompts) != 1 {
		t.Errorf("/api/prompts: expected 1, got %d", len(prompts))
	}

	// test /api/call
	body := `{"method":"tools/call","params":{"name":"ping"}}`
	resp, _ = http.Post(base+"/api/call", "application/json", strings.NewReader(body))
	var callResp map[string]any
	json.NewDecoder(resp.Body).Decode(&callResp)
	resp.Body.Close()
	if callResp["result"] == nil {
		t.Error("/api/call: no result")
	}
}
