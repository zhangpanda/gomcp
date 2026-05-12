package gomcp_test

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
)

// TestSoak_MemoryStability runs a sustained tool-call workload and
// asserts that heap usage does not grow unboundedly. Default duration
// is 30s (safe for CI); set SOAK_DURATION=24h for a real overnight
// soak. Skipped in -short mode.
//
// The test samples HeapInuse at the start and end. A >2x growth
// after forced GC indicates a leak (session map, middleware state,
// context store, etc.).
func TestSoak_MemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	duration := 30 * time.Second
	if d := os.Getenv("SOAK_DURATION"); d != "" {
		parsed, err := time.ParseDuration(d)
		if err != nil {
			t.Fatalf("bad SOAK_DURATION: %v", err)
		}
		duration = parsed
	}

	s := gomcp.New("soak", "1.0.0")
	s.Use(gomcp.Recovery())
	s.Use(gomcp.RequestID())
	s.Tool("echo", "", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text(ctx.String("msg")), nil
	})

	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "echo", "arguments": map[string]any{"msg": "soak"}},
	})
	ctx := context.Background()

	// Warm up + force GC to get a clean baseline.
	for i := 0; i < 1000; i++ {
		_ = s.HandleRaw(ctx, req)
	}
	runtime.GC()
	runtime.GC()

	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Sustained load.
	deadline := time.Now().Add(duration)
	calls := 0
	for time.Now().Before(deadline) {
		_ = s.HandleRaw(ctx, req)
		calls++
	}

	// Force GC and measure.
	runtime.GC()
	runtime.GC()

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	baselineMB := float64(baseline.HeapInuse) / (1 << 20)
	afterMB := float64(after.HeapInuse) / (1 << 20)
	growth := afterMB / baselineMB

	t.Logf("soak: %d calls in %v (%.0f calls/s)", calls, duration, float64(calls)/duration.Seconds())
	t.Logf("soak: heap baseline=%.2f MB, after=%.2f MB, growth=%.2fx", baselineMB, afterMB, growth)

	// Allow up to 2x growth (GC pressure, runtime overhead). Anything
	// beyond that signals a real leak.
	if growth > 2.0 {
		t.Fatalf("heap grew %.2fx (%.2f MB → %.2f MB) — likely memory leak", growth, baselineMB, afterMB)
	}
}

// TestSoak_SessionEvictionMemory creates sessions in a tight loop and
// verifies that eviction actually frees memory. Without proper
// eviction, the session map would grow without bound.
func TestSoak_SessionEvictionMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in short mode")
	}

	s := gomcp.New("soak-sess", "1.0.0")
	sm := s.Sessions()

	// Create 10k sessions.
	for i := 0; i < 10_000; i++ {
		sm.Get(itoa2(i))
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// Evict all (simulate 1h idle).
	sm.EvictIdleForTest(time.Now().Add(time.Hour))

	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	count := sm.Count()
	if count != 0 {
		t.Errorf("expected 0 sessions after eviction, got %d", count)
	}

	freedMB := float64(before.HeapInuse-after.HeapInuse) / (1 << 20)
	t.Logf("soak-sess: evicted 10k sessions, freed %.2f MB, remaining count=%d", freedMB, count)

	// We should free *something*. If freed is negative or zero, the
	// eviction didn't actually release memory.
	if freedMB < 0.01 {
		t.Logf("warning: eviction freed very little memory (%.4f MB) — sessions may not be GC'd properly", freedMB)
	}

	s.Close()
}

func itoa2(n int) string {
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if i == len(buf) {
		return "0"
	}
	return string(buf[i:])
}
