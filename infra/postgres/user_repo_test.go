package postgres_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/truthmarket/truth-market/infra/postgres"
	"github.com/truthmarket/truth-market/infra/testutil"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// --------------------------------------------------------------------------
// UserRepo integration tests
// --------------------------------------------------------------------------

func TestUserRepo_Create_InsertsAndReturnsUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser(testutil.WithBalance(500), testutil.WithWallet("0xABCDEF1234567890abcd"))
	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Verify the row was persisted by reading it back.
	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)

	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.WalletAddress, got.WalletAddress)
	assert.Equal(t, domain.UserTypeHuman, got.UserType)
	assert.True(t, decimal.NewFromFloat(500).Equal(got.Balance), "expected balance 500, got %s", got.Balance)
	assert.True(t, decimal.Zero.Equal(got.LockedBalance), "expected locked_balance 0, got %s", got.LockedBalance)
	assert.False(t, got.IsAdmin)
	assert.False(t, got.CreatedAt.IsZero())
}

func TestUserRepo_GetByID_ReturnsUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser()
	require.NoError(t, repo.Create(ctx, user))

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.WalletAddress, got.WalletAddress)
	assert.Equal(t, user.UserType, got.UserType)
}

func TestUserRepo_GetByID_NotFound_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
}

func TestUserRepo_GetByWallet_ReturnsUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser(testutil.WithWallet("0x1111222233334444aaaa"))
	require.NoError(t, repo.Create(ctx, user))

	got, err := repo.GetByWallet(ctx, "0x1111222233334444aaaa")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "0x1111222233334444aaaa", got.WalletAddress)
}

func TestUserRepo_GetByWallet_NotFound_ReturnsNil(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	_, err := repo.GetByWallet(ctx, "0xNONEXISTENTWALLETADDR")
	// The current implementation wraps pgx.ErrNoRows; the caller should see an
	// error (no nil user without error).
	require.Error(t, err)
}

func TestUserRepo_UpdateBalance_UpdatesCorrectly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	user := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, repo.Create(ctx, user))

	newBalance := decimal.NewFromFloat(750)
	newLocked := decimal.NewFromFloat(250)
	err := repo.UpdateBalance(ctx, user.ID, newBalance, newLocked)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, newBalance.Equal(got.Balance), "expected balance %s, got %s", newBalance, got.Balance)
	assert.True(t, newLocked.Equal(got.LockedBalance), "expected locked %s, got %s", newLocked, got.LockedBalance)
}

func TestUserRepo_List_WithFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateUsers(t)

	repo := postgres.NewUserRepo(testPool)
	ctx := context.Background()

	// Seed a mix of users.
	human1 := testutil.NewUser()
	human2 := testutil.NewUser(testutil.WithAdmin())
	agent1 := testutil.NewAgent()

	require.NoError(t, repo.Create(ctx, human1))
	require.NoError(t, repo.Create(ctx, human2))
	require.NoError(t, repo.Create(ctx, agent1))

	t.Run("no filter returns all", func(t *testing.T) {
		users, total, err := repo.List(ctx, repository.UserFilter{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, users, 3)
	})

	t.Run("filter by user type human", func(t *testing.T) {
		ut := domain.UserTypeHuman
		users, total, err := repo.List(ctx, repository.UserFilter{UserType: &ut, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, users, 2)
		for _, u := range users {
			assert.Equal(t, domain.UserTypeHuman, u.UserType)
		}
	})

	t.Run("filter by is_admin true", func(t *testing.T) {
		isAdmin := true
		users, total, err := repo.List(ctx, repository.UserFilter{IsAdmin: &isAdmin, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, users, 1)
		assert.True(t, users[0].IsAdmin)
	})

	t.Run("filter by agent type", func(t *testing.T) {
		ut := domain.UserTypeAgent
		users, total, err := repo.List(ctx, repository.UserFilter{UserType: &ut, Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, users, 1)
		assert.Equal(t, domain.UserTypeAgent, users[0].UserType)
	})

	t.Run("pagination limit and offset", func(t *testing.T) {
		users, total, err := repo.List(ctx, repository.UserFilter{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, users, 2)

		users2, total2, err := repo.List(ctx, repository.UserFilter{Limit: 2, Offset: 2})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total2)
		assert.Len(t, users2, 1)
	})
}
