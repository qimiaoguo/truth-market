package middleware

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// eventCarrier adapts a *domain.DomainEvent so it can be used as a
// propagation.TextMapCarrier. Trace context is stored in TraceID / SpanID
// fields which are the only two header-like values we need.
type eventCarrier struct {
	event *domain.DomainEvent
}

func (c *eventCarrier) Get(key string) string {
	switch key {
	case "trace_id":
		return c.event.TraceID
	case "span_id":
		return c.event.SpanID
	default:
		return ""
	}
}

func (c *eventCarrier) Set(key, value string) {
	switch key {
	case "trace_id":
		c.event.TraceID = value
	case "span_id":
		c.event.SpanID = value
	}
}

func (c *eventCarrier) Keys() []string {
	return []string{"trace_id", "span_id"}
}

// InjectTraceContext copies the active span's trace ID and span ID from ctx
// into the DomainEvent so that downstream consumers can correlate logs and
// spans.
func InjectTraceContext(ctx context.Context, event *domain.DomainEvent) {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	event.TraceID = sc.TraceID().String()
	event.SpanID = sc.SpanID().String()
}

// ExtractTraceContext reconstructs a context.Context carrying the trace
// context encoded in the DomainEvent. This enables consumers to continue the
// distributed trace started by the publisher.
func ExtractTraceContext(event domain.DomainEvent) context.Context {
	if event.TraceID == "" {
		return context.Background()
	}

	traceID, err := trace.TraceIDFromHex(event.TraceID)
	if err != nil {
		return context.Background()
	}

	var spanID trace.SpanID
	if event.SpanID != "" {
		spanID, err = trace.SpanIDFromHex(event.SpanID)
		if err != nil {
			return context.Background()
		}
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	return trace.ContextWithRemoteSpanContext(context.Background(), sc)
}
