package logger

import "context"

// ctxKey is an unexported type used as the context key for Logger values,
// preventing collisions with keys defined in other packages.
type ctxKey struct{}

// FromContext returns the Logger stored in ctx. If no Logger is present a
// no-op Logger is returned so callers never need to nil-check.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return newNop()
}

// WithLogger returns a child context carrying the given Logger.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}
