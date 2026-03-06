package repository

import "context"

// TxManager provides transactional execution guarantees. Implementations wrap
// the supplied function in a database transaction, committing on success and
// rolling back on error. The transaction handle is propagated through the
// context so that repositories called within fn participate in the same
// transaction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
