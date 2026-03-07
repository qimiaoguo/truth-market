package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// APIKeyRepo implements repository.APIKeyRepository using PostgreSQL.
type APIKeyRepo struct {
	BaseRepo
}

// NewAPIKeyRepo creates a new APIKeyRepo.
func NewAPIKeyRepo(pool *pgxpool.Pool) *APIKeyRepo {
	return &APIKeyRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.APIKeyRepository = (*APIKeyRepo)(nil)

func (r *APIKeyRepo) Create(ctx context.Context, key *domain.APIKey) error {
	err := r.Q(ctx).CreateAPIKey(ctx, sqlcgen.CreateAPIKeyParams{
		ID:        key.ID,
		UserID:    key.UserID,
		KeyHash:   key.KeyHash,
		KeyPrefix: key.KeyPrefix,
		IsActive:  key.IsActive,
		ExpiresAt: tstzFromOptional(key.ExpiresAt),
		CreatedAt: tstz(key.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create api key: %w", err)
	}
	return nil
}

func (r *APIKeyRepo) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	row, err := r.Q(ctx).GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("postgres: get api key by hash: %w", err)
	}
	return apikeyFromRow(row), nil
}

func (r *APIKeyRepo) ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	rows, err := r.Q(ctx).ListAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list api keys by user: %w", err)
	}

	keys := make([]*domain.APIKey, len(rows))
	for i, row := range rows {
		keys[i] = &domain.APIKey{
			ID:        row.ID,
			UserID:    row.UserID,
			KeyHash:   row.KeyHash,
			KeyPrefix: row.KeyPrefix,
			IsActive:  row.IsActive,
			ExpiresAt: optionalTimeFromTstz(row.ExpiresAt),
			CreatedAt: row.CreatedAt.Time,
		}
	}
	return keys, nil
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
	n, err := r.Q(ctx).RevokeAPIKey(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: revoke api key: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: revoke api key: %w", pgx.ErrNoRows)
	}
	return nil
}

func apikeyFromRow(r sqlcgen.GetAPIKeyByHashRow) *domain.APIKey {
	return &domain.APIKey{
		ID:        r.ID,
		UserID:    r.UserID,
		KeyHash:   r.KeyHash,
		KeyPrefix: r.KeyPrefix,
		IsActive:  r.IsActive,
		ExpiresAt: optionalTimeFromTstz(r.ExpiresAt),
		CreatedAt: r.CreatedAt.Time,
	}
}
