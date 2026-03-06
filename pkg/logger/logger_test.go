package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestNew_DefaultConfig(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf))

	l.Info("hello", "key", "value")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("failed to parse JSON log output: %v", err)
	}

	if rec["msg"] != "hello" {
		t.Errorf("expected msg=hello, got %v", rec["msg"])
	}
	if rec["key"] != "value" {
		t.Errorf("expected key=value, got %v", rec["key"])
	}
}

func TestLogger_WithContext_InjectsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf), WithLevel(slog.LevelDebug))

	// Create a span context with known trace and span IDs.
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	// Embed the span context into a noop span / context.
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	// Use a noop TracerProvider to get a real Span wrapping our SpanContext.
	tp := noop.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(ctx, "test-span")
	defer span.End()

	l.WithContext(ctx).Info("traced message")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("failed to parse JSON log output: %v", err)
	}

	if rec["trace_id"] != traceID.String() {
		t.Errorf("expected trace_id=%s, got %v", traceID.String(), rec["trace_id"])
	}
	if rec["span_id"] == nil || rec["span_id"] == "" {
		t.Errorf("expected span_id to be present, got %v", rec["span_id"])
	}
}

func TestFromContext_NoLogger_ReturnsDefault(t *testing.T) {
	ctx := context.Background()
	l := FromContext(ctx)
	if l == nil {
		t.Fatal("expected non-nil logger from empty context")
	}

	// The nop logger should not panic.
	l.Info("should not panic")
	l.With("k", "v").Debug("also fine")
}

func TestWithLogger_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf), WithServiceName("test-svc"))

	ctx := WithLogger(context.Background(), l)
	got := FromContext(ctx)

	got.Info("roundtrip")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("failed to parse JSON log output: %v", err)
	}
	if rec["service"] != "test-svc" {
		t.Errorf("expected service=test-svc, got %v", rec["service"])
	}
}
