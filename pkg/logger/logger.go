package logger

import (
	"context"
	"log/slog"
)

// Logger is the application logging interface. All services use this instead
// of slog directly so that trace correlation and output format can be
// controlled centrally.
type Logger interface {
	// Debug logs at LevelDebug.
	Debug(msg string, args ...any)
	// Info logs at LevelInfo.
	Info(msg string, args ...any)
	// Warn logs at LevelWarn.
	Warn(msg string, args ...any)
	// Error logs at LevelError.
	Error(msg string, args ...any)
	// With returns a Logger that includes the given key-value attributes in
	// every subsequent record.
	With(args ...any) Logger
	// WithContext returns a Logger that uses ctx for trace-context extraction
	// on every subsequent log call.
	WithContext(ctx context.Context) Logger
}

// slogLogger wraps *slog.Logger to implement the Logger interface.
type slogLogger struct {
	inner *slog.Logger
	ctx   context.Context
}

// New constructs a new Logger by applying the provided options over a set of
// sensible defaults (JSON format, INFO level, stderr output).
func New(opts ...Option) Logger {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	handlerOpts := &slog.HandlerOptions{Level: cfg.level}

	var baseHandler slog.Handler
	switch cfg.format {
	case FormatText:
		baseHandler = slog.NewTextHandler(cfg.output, handlerOpts)
	default:
		baseHandler = slog.NewJSONHandler(cfg.output, handlerOpts)
	}

	handler := newTraceHandler(baseHandler)
	inner := slog.New(handler)

	if cfg.serviceName != "" {
		inner = inner.With("service", cfg.serviceName)
	}

	return &slogLogger{
		inner: inner,
		ctx:   context.Background(),
	}
}

func (l *slogLogger) Debug(msg string, args ...any) {
	l.inner.DebugContext(l.ctx, msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.inner.InfoContext(l.ctx, msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.inner.WarnContext(l.ctx, msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.inner.ErrorContext(l.ctx, msg, args...)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{
		inner: l.inner.With(args...),
		ctx:   l.ctx,
	}
}

func (l *slogLogger) WithContext(ctx context.Context) Logger {
	return &slogLogger{
		inner: l.inner,
		ctx:   ctx,
	}
}

// nopLogger is a Logger that silently discards all output.
type nopLogger struct{}

func newNop() Logger           { return &nopLogger{} }
func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}
func (n nopLogger) With(...any) Logger { return n }
func (n nopLogger) WithContext(context.Context) Logger { return n }
