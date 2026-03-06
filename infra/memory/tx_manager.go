package memory

import "context"

// NoOpTxManager implements repository.TxManager as a no-op, simply executing
// the provided function without any transactional wrapping. This is suitable
// for in-memory stores that do not support real transactions.
type NoOpTxManager struct{}

// NewNoOpTxManager returns a new NoOpTxManager.
func NewNoOpTxManager() *NoOpTxManager {
	return &NoOpTxManager{}
}

// WithTx executes fn directly without transactional semantics.
func (m *NoOpTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
