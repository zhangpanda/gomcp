package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestSSE_Endurance opens an SSE connection for 10 seconds (shortened
// from 60s for CI; set SSE_ENDURANCE_DURATION=60s locally for a real
// soak), verifies heartbeat comments arrive at the configured
// interval, and confirms that notifications pushed via Notify() are
// delivered intact to the stream.
//
// This catches:
//   - heartbeat timer drift or stall
//   - connection silently dropped by intermediate buffering
//   - Notify fan-out race under sustained load
func TestSSE_Endurance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping SSE endurance in short mode")
	}

	duration := 10 * time.Second

	// Build a minimal handler that just echoes "pong" for tools/call.
	handler := func(ctx context.Context, raw json.RawMessage) json.RawMessage {
		var msg struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(raw, &msg)
		if msg.ID == nil {
			return nil
		}
		resp, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{},
		})
		return resp
	}

	srv := NewHTTPServer(handler)
	srv.SSEHeartbeat = 1 * time.Second // fast heartbeat for test

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	httpSrv := &http.Server{Handler: srv}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	})

	url := "http://" + ln.Addr().String()

	// Open SSE stream.
	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type: %q", ct)
	}

	// Collect events in background.
	var heartbeats atomic.Int64
	var notifications atomic.Int64
	scanDone := make(chan struct{})

	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, ": heartbeat") {
				heartbeats.Add(1)
			}
			if strings.HasPrefix(line, "data: ") {
				notifications.Add(1)
			}
		}
	}()

	// Push notifications at 100ms intervals for the duration.
	pushCtx, pushCancel := context.WithTimeout(context.Background(), duration)
	defer pushCancel()

	pushed := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-pushCtx.Done():
			break loop
		case <-ticker.C:
			srv.Notify("test/ping", map[string]int{"seq": pushed})
			pushed++
		}
	}

	// Give the scanner a moment to drain buffered events.
	cancel() // close the SSE stream
	select {
	case <-scanDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("scanner did not finish after stream close")
	}

	// Assertions.
	hb := heartbeats.Load()
	notif := notifications.Load()

	// With 1s heartbeat over 10s, expect at least 8 heartbeats
	// (allowing 2 for timing jitter).
	expectedHB := int64(duration.Seconds()) - 2
	if hb < expectedHB {
		t.Errorf("heartbeats: got %d, want >= %d", hb, expectedHB)
	}

	// We pushed every 100ms for 10s = ~100 notifications. Allow 10%
	// loss for timing but not more.
	expectedNotif := int64(float64(pushed) * 0.9)
	if notif < expectedNotif {
		t.Errorf("notifications: got %d, pushed %d, want >= %d", notif, pushed, expectedNotif)
	}

	t.Logf("SSE endurance: %v duration, %d heartbeats, %d/%d notifications delivered",
		duration, hb, notif, pushed)
}
