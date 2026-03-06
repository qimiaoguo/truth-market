package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// UserRepository implements repository.UserRepository with an in-memory map
// guarded by a read-write mutex for thread safety.
type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

// NewUserRepository returns a new empty in-memory UserRepository.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*domain.User),
	}
}

// Create inserts a new user record. Returns an error if a user with the same
// ID already exists.
func (r *UserRepository) Create(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; exists {
		return fmt.Errorf("user already exists: %s", user.ID)
	}

	stored := *user
	r.users[user.ID] = &stored
	return nil
}

// GetByID retrieves a user by their unique identifier.
func (r *UserRepository) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}

	out := *user
	return &out, nil
}

// GetByWallet retrieves a user by their wallet address.
func (r *UserRepository) GetByWallet(_ context.Context, addr string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.WalletAddress == addr {
			out := *user
			return &out, nil
		}
	}

	return nil, fmt.Errorf("user not found for wallet: %s", addr)
}

// UpdateBalance atomically sets the available and locked balances for a user.
func (r *UserRepository) UpdateBalance(_ context.Context, id string, balance, locked decimal.Decimal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		return fmt.Errorf("user not found: %s", id)
	}

	user.Balance = balance
	user.LockedBalance = locked
	return nil
}

// List returns users matching the given filter along with the total count.
func (r *UserRepository) List(_ context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*domain.User
	for _, user := range r.users {
		if filter.UserType != nil && user.UserType != *filter.UserType {
			continue
		}
		if filter.IsAdmin != nil && user.IsAdmin != *filter.IsAdmin {
			continue
		}
		out := *user
		matched = append(matched, &out)
	}

	total := int64(len(matched))

	// Apply pagination.
	start := filter.Offset
	if start > len(matched) {
		start = len(matched)
	}
	end := len(matched)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	return matched[start:end], total, nil
}
