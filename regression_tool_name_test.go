package gomcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhangpanda/gomcp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestRegression_OTelSeesToolName guards against the bug where the
// outer middleware chain fired before _tool_name was populated,
// making OpenTelemetry spans come out as "mcp.tool.unknown" instead
// of "mcp.tool.<name>". Same fix unblocks PrometheusMetrics and any
// user-authored middleware that reads ctx.Get("_tool_name").
//
// handleRequestInternal now peeks tools/call params and sets
// _tool_name on the outer context before the chain runs.
func TestRegression_OTelSeesToolName(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	s := gomcp.New("t", "1.0")
	s.Use(gomcp.OpenTelemetryWithProvider(tp))
	s.Tool("traced", "emits span",
		func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return ctx.Text("ok"), nil
		},
	)

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"traced","arguments":{}}}`)
	_ = s.HandleRaw(context.Background(), req)

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatalf("no spans emitted")
	}
	for _, sp := range spans {
		if sp.Name == "mcp.tool.traced" {
			return
		}
	}
	names := make([]string, 0, len(spans))
	for _, sp := range spans {
		names = append(names, sp.Name)
	}
	t.Fatalf("expected mcp.tool.traced, got %v", names)
}

// TestRegression_MetricsSeesToolName mirrors the above for
// PrometheusMetrics: previously the tool label ended up as "" because
// the middleware ran at the outer chain level, before _tool_name was
// set by handleToolsCall. After the fix, the counter must be bumped
// for the exact tool name.
func TestRegression_MetricsSeesToolName(t *testing.T) {
	mw, metrics := gomcp.PrometheusMetrics()

	s := gomcp.New("t", "1.0")
	s.Use(mw)
	s.Tool("measured", "",
		func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
			return ctx.Text("ok"), nil
		},
	)

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"measured","arguments":{}}}`)
	_ = s.HandleRaw(context.Background(), req)

	// Probe the Prometheus text-format handler: the exposed
	// gomcp_tool_calls_total series must carry the real tool label.
	rr := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rr.Body.String()

	want := `gomcp_tool_calls_total{tool="measured"} 1`
	if !strings.Contains(body, want) {
		t.Fatalf("metrics output missing %q\n--- body ---\n%s", want, body)
	}
	// Explicitly reject the old broken shape so a regression jumps
	// out: empty label or missing counter.
	if strings.Contains(body, `gomcp_tool_calls_total{tool=""} `) {
		t.Fatalf("metrics regressed: empty tool label present\n%s", body)
	}
}
