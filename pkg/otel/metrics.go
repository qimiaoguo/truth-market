package otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Meter returns a named meter obtained from the global MeterProvider.
// The name should typically be the fully-qualified package or service name.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}
