package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func echoHandler(_ context.Context, msg json.RawMessage) json.RawMessage {
	var req map[string]any
	json.Unmarshal(msg, &req)
	resp := map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": "ok"}
	data, _ := json.Marshal(resp)
	return data
}

func notificationHandler(_ context.Context, msg json.RawMessage) json.RawMessage {
	return nil // notifications return nil
}

// --- stdio tests ---

func TestServeIO_SingleMessage(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serveIO(ctx, r, &w, echoHandler)
	if err != nil {
		t.Fatalf("serveIO error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["result"] != "ok" {
		t.Errorf("unexpected result: %v", resp["result"])
	}
}

func TestServeIO_MultipleMessages(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"a"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"b"}` + "\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveIO(ctx, r, &w, echoHandler)

	lines := strings.Split(strings.TrimSpace(w.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}
}

func TestServeIO_EmptyLines(t *testing.T) {
	input := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveIO(ctx, r, &w, echoHandler)

	lines := strings.Split(strings.TrimSpace(w.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 response (empty lines skipped), got %d", len(lines))
	}
}

func TestServeIO_Notification(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	r := strings.NewReader(input)
	var w bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveIO(ctx, r, &w, notificationHandler)

	if w.Len() != 0 {
		t.Errorf("expected no output for notification, got: %s", w.String())
	}
}

// --- HTTP tests ---

func TestHTTP_PostSingle(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["result"] != "ok" {
		t.Errorf("unexpected result: %v", resp["result"])
	}
}

func TestHTTP_PostBatch(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	body := `[{"jsonrpc":"2.0","id":1,"method":"a"},{"jsonrpc":"2.0","id":2,"method":"b"}]`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var batch []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &batch); err != nil {
		t.Fatalf("expected JSON array response: %v", err)
	}
	if len(batch) != 2 {
		t.Errorf("expected 2 responses, got %d", len(batch))
	}
}

func TestHTTP_PostNotification(t *testing.T) {
	srv := NewHTTPServer(notificationHandler)
	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202 for notification, got %d", w.Code)
	}
}

func TestHTTP_MethodNotAllowed(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	req := httptest.NewRequest(http.MethodPut, "/mcp", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHTTP_ServeHTTPHandler(t *testing.T) {
	h := ServeHTTPHandler(echoHandler)
	if h == nil {
		t.Fatal("ServeHTTPHandler returned nil")
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHTTP_LargeBody(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	// Create a body larger than 10MB
	large := strings.Repeat("x", 11<<20)
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(large))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	// Should still respond (LimitReader truncates, but handler processes what it gets)
	// The response may be an error since the truncated body isn't valid JSON
	respBody, _ := io.ReadAll(w.Result().Body)
	_ = respBody // just ensure no panic
}

// --- ParseBatch tests ---

func TestParseBatch_Single(t *testing.T) {
	raw := json.RawMessage(`{"jsonrpc":"2.0","id":1}`)
	msgs, isBatch := ParseBatch(raw)
	if isBatch {
		t.Error("expected single, got batch")
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestParseBatch_Array(t *testing.T) {
	raw := json.RawMessage(`[{"jsonrpc":"2.0","id":1},{"jsonrpc":"2.0","id":2}]`)
	msgs, isBatch := ParseBatch(raw)
	if !isBatch {
		t.Error("expected batch")
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestParseBatch_Whitespace(t *testing.T) {
	raw := json.RawMessage(`  [{"id":1}]`)
	_, isBatch := ParseBatch(raw)
	if !isBatch {
		t.Error("expected batch with leading whitespace")
	}
}

func TestHTTP_ServeAddr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeHTTPAddr(ctx, "127.0.0.1:0", echoHandler)
	}()

	// Cancel immediately to test shutdown
	cancel()

	err := <-errCh
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- SSE Notify tests ---

func TestHTTP_Notify(t *testing.T) {
	srv := NewHTTPServer(echoHandler)

	// Simulate SSE client
	ch := make(chan []byte, 32)
	srv.addClient(ch)
	defer srv.removeClient(ch)

	srv.Notify("notifications/tools/list_changed", nil)

	select {
	case event := <-ch:
		s := string(event)
		if !strings.Contains(s, "data: ") {
			t.Errorf("expected SSE data prefix, got: %s", s)
		}
		if !strings.Contains(s, "list_changed") {
			t.Errorf("expected list_changed in event, got: %s", s)
		}
	default:
		t.Fatal("expected notification event")
	}
}

func TestHTTP_Notify_WithParams(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	ch := make(chan []byte, 32)
	srv.addClient(ch)
	defer srv.removeClient(ch)

	srv.Notify("test/method", map[string]string{"key": "val"})

	event := <-ch
	s := string(event)
	if !strings.Contains(s, `"key":"val"`) {
		t.Errorf("expected params in event, got: %s", s)
	}
}

func TestHTTP_Notify_NoClients(t *testing.T) {
	srv := NewHTTPServer(echoHandler)
	// Should not panic with no clients
	srv.Notify("test", nil)
}

func TestHTTP_SSE_Endpoint(t *testing.T) {
	srv := NewHTTPServer(echoHandler)

	// Use a context we can cancel to end the SSE connection
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(w, req)
		close(done)
	}()

	// Send a notification while SSE is connected
	// Give the handler a moment to set up
	time.Sleep(10 * time.Millisecond)
	srv.Notify("test/event", nil)
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-done

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}
}
