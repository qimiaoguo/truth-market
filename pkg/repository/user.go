package repository

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// UserFilter holds optional criteria for listing users.
type UserFilter struct {
	UserType *domain.UserType
	IsAdmin  *bool
	Limit    int
	Offset   int
}

// UserRepository defines persistence operations for user accounts.
type UserRepository interface {
	// Create inserts a new user record.
	Create(ctx context.Context, user *domain.User) error

	// GetByID retrieves a user by their unique identifier.
	GetByID(ctx context.Context, id string) (*domain.User, error)

	// GetByWallet retrieves a user by their wallet address.
	GetByWallet(ctx context.Context, addr string) (*domain.User, error)

	// UpdateBalance atomically sets the available and locked balances for a user.
	UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error

	// List returns users matching the given filter along with the total count.
	List(ctx context.Context, filter UserFilter) ([]*domain.User, int64, error)
}
