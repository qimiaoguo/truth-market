// Package logger provides structured logging with OpenTelemetry trace
// correlation for the truth-market platform.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// Format controls the output encoding of log records.
type Format int

const (
	// FormatJSON outputs structured JSON log lines (default).
	FormatJSON Format = iota
	// FormatText outputs human-readable coloured text, useful during local development.
	FormatText
)

// config holds the resolved logger configuration after applying options.
type config struct {
	level       slog.Level
	format      Format
	serviceName string
	output      io.Writer
}

// defaultConfig returns sensible defaults: JSON format, INFO level, stderr.
func defaultConfig() config {
	return config{
		level:  slog.LevelInfo,
		format: FormatJSON,
		output: os.Stderr,
	}
}

// Option configures a Logger during construction.
type Option func(*config)

// WithLevel sets the minimum log level.
func WithLevel(l slog.Level) Option {
	return func(c *config) {
		c.level = l
	}
}

// WithFormat selects JSON or Text output.
func WithFormat(f Format) Option {
	return func(c *config) {
		c.format = f
	}
}

// WithServiceName attaches a "service" attribute to every log record.
func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithOutput directs log output to the given writer.
func WithOutput(w io.Writer) Option {
	return func(c *config) {
		c.output = w
	}
}
