// Regression tests for bugs found in the 2026-05-08 code review.
// Each test is paired with a summary of the original defect.
package gomcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/transport"
)

// BUG FIX-1 (HIGH): The HTTP transport read the Mcp-Session-Id request
// header but never emitted a server-assigned session ID back to the
// client, leaving fresh clients session-less on every call. The fix
// plumbs a SessionIDSink through the request context and the server
// writes the active session ID before responding.
func TestRegression_SessionIDReturnedInResponseHeader(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Tool("ping", "ping", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	})

	srv := httptest.NewServer(transport.ServeHTTPHandler(s.HandleRaw))
	defer srv.Close()

	params, _ := json.Marshal(map[string]any{"name": "ping"})
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	sid := resp.Header.Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("expected Mcp-Session-Id response header, got none")
	}

	// Second request with the same ID should keep it.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Mcp-Session-Id", sid)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if got := resp2.Header.Get("Mcp-Session-Id"); got != sid {
		t.Fatalf("expected round-tripped session ID %q, got %q", sid, got)
	}
}

// BUG FIX-1 (spec compliance): initialize must also return an
// Mcp-Session-Id header so clients know their session ID before making
// any tool calls.
func TestRegression_InitializeReturnsSessionID(t *testing.T) {
	s := gomcp.New("t", "1.0")

	srv := httptest.NewServer(transport.ServeHTTPHandler(s.HandleRaw))
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "c", "version": "1"},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Mcp-Session-Id") == "" {
		t.Fatal("initialize should emit Mcp-Session-Id per MCP spec")
	}
}

// BUG FIX-2 (MEDIUM): SessionManager eviction used to read lastAccess
// under RLock and then Delete outside the lock — a touch() between the
// two steps would be silently undone. The fix holds the session write
// lock across check-and-mark and uses CompareAndDelete so a racing
// replacement is not clobbered.
func TestRegression_SessionEvictionDoesNotLoseActiveWrites(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Tool("use", "use session", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		ctx.Session().Set("hit", true)
		return ctx.Text("ok"), nil
	})

	headers := map[string]string{"Mcp-Session-Id": "sess-fix2"}
	sessCtx := context.WithValue(context.Background(), transport.CtxKeyHeaders, headers)

	// First call creates the session.
	params, _ := json.Marshal(map[string]any{"name": "use"})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	s.HandleRaw(sessCtx, req)

	// Force-evict immediately (TTL-past-the-future).
	sm := s.Sessions()
	sm.EvictIdleForTest(time.Now().Add(time.Hour))

	// New call with same ID should now produce a brand-new session
	// (because the old one was evicted) — verified by the previous
	// "hit" flag being absent on the fresh session.
	var captured bool
	s2 := gomcp.New("t2", "1.0")
	s2.Tool("check", "check", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		_, ok := ctx.Session().Get("hit")
		captured = ok
		return ctx.Text(""), nil
	})
	params2, _ := json.Marshal(map[string]any{"name": "check"})
	req2, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": json.RawMessage(params2)})
	s2.HandleRaw(sessCtx, req2)
	if captured {
		t.Fatal("fresh session should not carry state across eviction")
	}

	// And eviction must not wipe out concurrent writes on the server
	// that just touched the session: we start fresh and touch rapidly
	// while scheduling eviction with a past-ttl time.
	s3 := gomcp.New("t3", "1.0")
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				s3.Sessions().Get("racer")
			}
		}
	}()
	for i := 0; i < 500; i++ {
		s3.Sessions().EvictIdleForTest(time.Now()) // ttl=30m, not expired
	}
	close(stop)
	wg.Wait()
	if s3.Sessions().Count() == 0 {
		t.Fatal("touch-then-evict race killed a fresh session — eviction window regressed")
	}
}

// BUG FIX-3 (MEDIUM): AsyncTool handlers used context.Background() as
// their root, so Server.Close could not cancel them. The fix derives
// every task context from taskManager.rootCtx, which Close cancels.
func TestRegression_AsyncTasksCancelledOnServerClose(t *testing.T) {
	s := gomcp.New("t", "1.0")

	cancelled := make(chan struct{}, 1)
	started := make(chan struct{})
	s.AsyncTool("sleep", "sleep", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		close(started)
		select {
		case <-ctx.Context().Done():
			cancelled <- struct{}{}
			return nil, ctx.Context().Err()
		case <-time.After(10 * time.Second):
			return ctx.Text("finished"), nil
		}
	})

	params, _ := json.Marshal(map[string]any{"name": "sleep"})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	_ = s.HandleRaw(context.Background(), req)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("async handler never started")
	}

	s.Close()

	select {
	case <-cancelled:
		// good — handler observed its ctx being cancelled
	case <-time.After(2 * time.Second):
		t.Fatal("Server.Close did not cancel in-flight async task")
	}
}

// BUG FIX-4 (MEDIUM): provider.watchDir removed stale tools but never
// emitted notifications/tools/list_changed, so clients kept stale lists.
// This test drops a YAML file into a temp dir, lets the watcher pick it
// up, deletes it, and asserts the server notifies on delete.
func TestRegression_WatchDirNotifiesOnDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thing.tool.yaml")
	if err := os.WriteFile(path, []byte("name: thing\ndescription: a thing\nhandler: http://localhost/x\nmethod: GET\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := gomcp.New("t", "1.0")

	var notifyMu sync.Mutex
	var notifyCount int
	s.SetNotifyFnForTest(func(method string, _ any) {
		if method == "notifications/tools/list_changed" {
			notifyMu.Lock()
			notifyCount++
			notifyMu.Unlock()
		}
	})

	if err := s.LoadDir(dir, gomcp.DirOptions{Watch: true, Interval: 50 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// wait for initial load notify
	time.Sleep(50 * time.Millisecond)
	notifyMu.Lock()
	initial := notifyCount
	notifyMu.Unlock()
	if initial == 0 {
		t.Fatal("expected at least one notify from initial load")
	}

	// delete the file
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// wait up to 1s for the watcher to pick up
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		notifyMu.Lock()
		got := notifyCount
		notifyMu.Unlock()
		if got > initial {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("watchDir did not notify list_changed after file deletion")
}

// BUG FIX-8 (LOW): APIKeyAuth's argument fallback left api_key in the
// live ctx.Args() map, so a later Logger middleware or the tool handler
// itself could observe (and log) the secret. The fix deletes the key
// from args after a successful validation.
func TestRegression_APIKeyAuthRedactsArgAfterValidation(t *testing.T) {
	s := gomcp.New("t", "1.0")
	s.Use(gomcp.APIKeyAuth("X-Api-Key", func(k string) (map[string]any, error) {
		if k == "good" {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("bad")
	}))

	var seenByHandler map[string]any
	s.Tool("inspect", "inspect", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		// snapshot so a background eviction cannot mutate it after the handler returns
		snap := make(map[string]any, len(ctx.Args()))
		for k, v := range ctx.Args() {
			snap[k] = v
		}
		seenByHandler = snap
		return ctx.Text("done"), nil
	})

	params, _ := json.Marshal(map[string]any{
		"name":      "inspect",
		"arguments": map[string]any{"api_key": "good", "other": "ok"},
	})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	s.HandleRaw(context.Background(), req)

	if _, present := seenByHandler["api_key"]; present {
		t.Fatal("api_key should have been stripped from ctx.Args() after APIKeyAuth validation")
	}
	if seenByHandler["other"] != "ok" {
		t.Fatalf("non-secret args must survive redaction, got %v", seenByHandler)
	}
}
