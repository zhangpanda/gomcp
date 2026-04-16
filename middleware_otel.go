package gomcp

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const otelScopeName = "github.com/istarshine/gomcp"

// OpenTelemetry returns a middleware that creates a span for each tool call.
// It uses the globally registered TracerProvider by default.
// Pass a custom TracerProvider via OpenTelemetryWithProvider().
func OpenTelemetry() Middleware {
	return OpenTelemetryWithProvider(otel.GetTracerProvider())
}

// OpenTelemetryWithProvider returns an OTel middleware using the given provider.
func OpenTelemetryWithProvider(tp trace.TracerProvider) Middleware {
	tracer := tp.Tracer(otelScopeName)

	return func(ctx *Context, next func() error) error {
		toolName := "unknown"
		if v, ok := ctx.Get("_tool_name"); ok {
			toolName = fmt.Sprintf("%v", v)
		}

		spanCtx, span := tracer.Start(ctx.Context(), "mcp.tool."+toolName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("mcp.tool.name", toolName),
			),
		)
		ctx.ctx = spanCtx
		defer span.End()

		if reqID, ok := ctx.Get("request_id"); ok {
			span.SetAttributes(attribute.String("mcp.request_id", fmt.Sprintf("%v", reqID)))
		}

		err := next()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}
