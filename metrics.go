package gomcp

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds Prometheus-compatible metrics for tool calls.
type Metrics struct {
	mu         sync.RWMutex
	toolCalls  map[string]*toolMetric
	totalCalls int64
	totalErrs  int64
}

type toolMetric struct {
	calls   int64
	errors  int64
	totalMs int64 // total duration in milliseconds
}

func newMetrics() *Metrics {
	return &Metrics{toolCalls: make(map[string]*toolMetric)}
}

func (m *Metrics) record(tool string, dur time.Duration, isErr bool) {
	atomic.AddInt64(&m.totalCalls, 1)
	if isErr {
		atomic.AddInt64(&m.totalErrs, 1)
	}

	m.mu.Lock()
	tm, ok := m.toolCalls[tool]
	if !ok {
		tm = &toolMetric{}
		m.toolCalls[tool] = tm
	}
	m.mu.Unlock()

	atomic.AddInt64(&tm.calls, 1)
	atomic.AddInt64(&tm.totalMs, dur.Milliseconds())
	if isErr {
		atomic.AddInt64(&tm.errors, 1)
	}
}

// Handler returns an http.Handler that serves /metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		fmt.Fprintf(w, "# HELP gomcp_calls_total Total tool calls\n")
		fmt.Fprintf(w, "# TYPE gomcp_calls_total counter\n")
		fmt.Fprintf(w, "gomcp_calls_total %d\n", atomic.LoadInt64(&m.totalCalls))

		fmt.Fprintf(w, "# HELP gomcp_errors_total Total tool errors\n")
		fmt.Fprintf(w, "# TYPE gomcp_errors_total counter\n")
		fmt.Fprintf(w, "gomcp_errors_total %d\n", atomic.LoadInt64(&m.totalErrs))

		m.mu.RLock()
		snapshot := make(map[string]*toolMetric, len(m.toolCalls))
		for k, tm := range m.toolCalls {
			snapshot[k] = tm
		}
		m.mu.RUnlock()
		tools := make([]string, 0, len(snapshot))
		for k := range snapshot {
			tools = append(tools, k)
		}
		sort.Strings(tools)

		fmt.Fprintf(w, "# HELP gomcp_tool_calls_total Calls per tool\n")
		fmt.Fprintf(w, "# TYPE gomcp_tool_calls_total counter\n")
		for _, name := range tools {
			tm := snapshot[name]
			fmt.Fprintf(w, "gomcp_tool_calls_total{tool=%q} %d\n", name, atomic.LoadInt64(&tm.calls))
		}

		fmt.Fprintf(w, "# HELP gomcp_tool_errors_total Errors per tool\n")
		fmt.Fprintf(w, "# TYPE gomcp_tool_errors_total counter\n")
		for _, name := range tools {
			tm := snapshot[name]
			fmt.Fprintf(w, "gomcp_tool_errors_total{tool=%q} %d\n", name, atomic.LoadInt64(&tm.errors))
		}

		fmt.Fprintf(w, "# HELP gomcp_tool_duration_ms_total Total duration per tool in ms\n")
		fmt.Fprintf(w, "# TYPE gomcp_tool_duration_ms_total counter\n")
		for _, name := range tools {
			tm := snapshot[name]
			fmt.Fprintf(w, "gomcp_tool_duration_ms_total{tool=%q} %d\n", name, atomic.LoadInt64(&tm.totalMs))
		}
	})
}

// PrometheusMetrics returns a middleware that records tool call metrics.
// Access metrics via s.MetricsHandler() for the /metrics endpoint.
func PrometheusMetrics() (Middleware, *Metrics) {
	m := newMetrics()
	mw := func(ctx *Context, next func() error) error {
		start := time.Now()
		err := next()
		tool := ""
		if v, ok := ctx.Get("_tool_name"); ok {
			tool = fmt.Sprintf("%v", v)
		}
		m.record(tool, time.Since(start), err != nil)
		return err
	}
	return mw, m
}
