package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// APIKeyRepository defines persistence operations for API key management.
type APIKeyRepository interface {
	// Create stores a new API key record.
	Create(ctx context.Context, key *domain.APIKey) error

	// GetByHash retrieves an API key by its SHA-256 hash.
	GetByHash(ctx context.Context, hash string) (*domain.APIKey, error)

	// ListByUser returns all API keys belonging to the specified user.
	ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error)

	// Revoke marks the API key identified by id as revoked.
	Revoke(ctx context.Context, id string) error
}
