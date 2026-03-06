package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/truthmarket/truth-market/infra/postgres"
	"github.com/truthmarket/truth-market/infra/testutil"
)

// --------------------------------------------------------------------------
// TxManager integration tests
// --------------------------------------------------------------------------

func TestTxManager_WithTx_Success_Commits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	txm := postgres.NewTxManager(testPool)
	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser(testutil.WithBalance(100))

	// Create the user inside a transaction that succeeds.
	err := txm.WithTx(ctx, func(txCtx context.Context) error {
		return repo.Create(txCtx, user)
	})
	require.NoError(t, err)

	// The user should be visible outside the transaction (committed).
	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestTxManager_WithTx_Error_Rollbacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	txm := postgres.NewTxManager(testPool)
	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser()

	// Create the user but return an error so the tx rolls back.
	deliberateErr := errors.New("deliberate failure")
	err := txm.WithTx(ctx, func(txCtx context.Context) error {
		if createErr := repo.Create(txCtx, user); createErr != nil {
			return createErr
		}
		return deliberateErr
	})
	require.ErrorIs(t, err, deliberateErr)

	// The user should NOT be visible (rolled back).
	_, err = repo.GetByID(ctx, user.ID)
	require.Error(t, err, "expected error because user should not exist after rollback")
}

func TestTxManager_WithTx_Panic_Rollbacks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	txm := postgres.NewTxManager(testPool)
	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser()

	// Wrap in a recover to catch the re-panic (if any) from WithTx.
	require.Panics(t, func() {
		_ = txm.WithTx(ctx, func(txCtx context.Context) error {
			if createErr := repo.Create(txCtx, user); createErr != nil {
				return createErr
			}
			panic("unexpected boom")
		})
	})

	// The user should NOT be visible (rolled back via deferred rollback).
	_, err := repo.GetByID(ctx, user.ID)
	require.Error(t, err, "expected error because user should not exist after panic rollback")
}

func TestTxManager_Repository_InsideTx_UsesTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	txm := postgres.NewTxManager(testPool)
	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser(testutil.WithBalance(500))

	err := txm.WithTx(ctx, func(txCtx context.Context) error {
		// Create and update within the same transaction.
		if createErr := repo.Create(txCtx, user); createErr != nil {
			return createErr
		}
		return repo.UpdateBalance(txCtx, user.ID,
			decimal.NewFromFloat(300), decimal.NewFromFloat(200))
	})
	require.NoError(t, err)

	// Both operations should be committed atomically.
	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(300).Equal(got.Balance),
		"expected balance 300, got %s", got.Balance)
	assert.True(t, decimal.NewFromFloat(200).Equal(got.LockedBalance),
		"expected locked 200, got %s", got.LockedBalance)
}

func TestTxManager_Repository_OutsideTx_UsesPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	// Operating without a transaction should work normally via the pool.
	user := testutil.NewUser()
	err := repo.Create(ctx, user)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}
