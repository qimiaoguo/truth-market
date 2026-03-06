package logger

import (
	"context"
	"log/slog"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// traceHandler is a slog.Handler decorator that injects OpenTelemetry
// trace_id and span_id attributes into every log record when a valid
// span is present in the context.
type traceHandler struct {
	inner slog.Handler
}

// newTraceHandler wraps an existing handler with trace-context injection.
func newTraceHandler(inner slog.Handler) *traceHandler {
	return &traceHandler{inner: inner}
}

// Enabled delegates to the inner handler.
func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle extracts the active OTel span from ctx and appends trace_id and
// span_id as structured attributes before forwarding to the inner handler.
func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes pre-applied.
func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new handler scoped to the named group.
func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}
