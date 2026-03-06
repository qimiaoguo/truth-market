package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, key_hash, key_prefix, is_active, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		key.ID,
		key.UserID,
		key.KeyHash,
		key.KeyPrefix,
		key.IsActive,
		key.ExpiresAt,
		key.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create api key: %w", err)
	}

	return nil
}

func (r *APIKeyRepo) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, user_id, key_hash, key_prefix, is_active, expires_at, created_at
		 FROM api_keys WHERE key_hash = $1`, hash)

	k, err := scanAPIKey(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: get api key by hash: %w", err)
	}

	return k, nil
}

func (r *APIKeyRepo) ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT id, user_id, key_hash, key_prefix, is_active, expires_at, created_at
		 FROM api_keys WHERE user_id = $1
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list api keys by user: %w", err)
	}
	defer rows.Close()

	var keys []*domain.APIKey
	for rows.Next() {
		k, err := scanAPIKeyFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list api keys scan: %w", err)
		}
		keys = append(keys, k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list api keys rows: %w", err)
	}

	return keys, nil
}

func (r *APIKeyRepo) Revoke(ctx context.Context, id string) error {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE api_keys SET is_active = false WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: revoke api key: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: revoke api key: %w", pgx.ErrNoRows)
	}

	return nil
}

// scanAPIKey scans a single API key from pgx.Row.
func scanAPIKey(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey

	err := row.Scan(
		&k.ID,
		&k.UserID,
		&k.KeyHash,
		&k.KeyPrefix,
		&k.IsActive,
		&k.ExpiresAt,
		&k.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &k, nil
}

// scanAPIKeyFromRows scans a single API key from pgx.Rows.
func scanAPIKeyFromRows(rows pgx.Rows) (*domain.APIKey, error) {
	var k domain.APIKey

	err := rows.Scan(
		&k.ID,
		&k.UserID,
		&k.KeyHash,
		&k.KeyPrefix,
		&k.IsActive,
		&k.ExpiresAt,
		&k.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &k, nil
}
