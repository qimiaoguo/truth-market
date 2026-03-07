package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// RankingRepo implements repository.RankingRepository using PostgreSQL.
type RankingRepo struct {
	BaseRepo
}

// NewRankingRepo creates a new RankingRepo.
func NewRankingRepo(pool *pgxpool.Pool) *RankingRepo {
	return &RankingRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.RankingRepository = (*RankingRepo)(nil)

// Upsert is a no-op because user_rankings is a materialized view and cannot
// be written to directly. Rankings are recalculated by calling
// RefreshMaterializedView instead.
func (r *RankingRepo) Upsert(ctx context.Context, ranking *domain.UserRanking) error {
	// user_rankings is a MATERIALIZED VIEW; INSERT/UPDATE is not supported.
	// Use RefreshMaterializedView to recalculate rankings.
	return nil
}

// dimensionColumn maps a RankDimension to the corresponding column in the
// user_rankings materialized view.
func dimensionColumn(d domain.RankDimension) string {
	switch d {
	case domain.RankDimensionTotalAssets:
		return "total_assets"
	case domain.RankDimensionPnL:
		return "pnl"
	case domain.RankDimensionVolume:
		return "volume"
	case domain.RankDimensionWinRate:
		return "win_rate"
	default:
		return "total_assets"
	}
}

// GetByUser returns all ranking dimensions for the specified user. Because the
// materialized view stores one row per user with every metric as a separate
// column, this method reads the single row and fans it out into one
// *domain.UserRanking per dimension.
func (r *RankingRepo) GetByUser(ctx context.Context, userID string) ([]*domain.UserRanking, error) {
	q := r.Querier(ctx)

	// For each dimension we compute the rank with a window function so the
	// caller receives the user's position on every leaderboard.
	dimensions := []domain.RankDimension{
		domain.RankDimensionTotalAssets,
		domain.RankDimensionPnL,
		domain.RankDimensionVolume,
		domain.RankDimensionWinRate,
	}

	var rankings []*domain.UserRanking

	for _, dim := range dimensions {
		col := dimensionColumn(dim)

		sql := fmt.Sprintf(
			`SELECT ur.user_id, ur.wallet_address, ur.user_type, ur.%s, ur.updated_at, ranked.rank
			 FROM user_rankings ur
			 JOIN (
				SELECT user_id, RANK() OVER (ORDER BY %s DESC) AS rank
				FROM user_rankings
			 ) ranked ON ranked.user_id = ur.user_id
			 WHERE ur.user_id = $1`, col, col)

		var (
			uid           string
			walletAddress string
			userType      string
			value         decimal.Decimal
			updatedAt     time.Time
			rank          int
		)

		err := q.QueryRow(ctx, sql, userID).Scan(&uid, &walletAddress, &userType, &value, &updatedAt, &rank)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			return nil, fmt.Errorf("postgres: get ranking by user dim=%s: %w", dim, err)
		}

		rankings = append(rankings, &domain.UserRanking{
			UserID:        uid,
			WalletAddress: walletAddress,
			UserType:      domain.UserType(userType),
			Dimension:     dim,
			Value:         value,
			Rank:          int(rank),
			UpdatedAt:     updatedAt,
		})
	}

	return rankings, nil
}

// List returns paginated rankings for a single dimension, optionally filtered
// by user type. The rank is computed on the fly via a window function over the
// requested dimension column.
func (r *RankingRepo) List(ctx context.Context, filter repository.RankingFilter) ([]*domain.UserRanking, int64, error) {
	q := r.Querier(ctx)

	// Determine which dimension column to use (default to total_assets).
	dim := domain.RankDimensionTotalAssets
	if filter.Dimension != nil {
		dim = *filter.Dimension
	}
	col := dimensionColumn(dim)

	// ---- build WHERE clause ----
	var (
		wheres []string
		args   []any
		idx    int
	)

	if filter.UserType != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("user_type = $%d", idx))
		args = append(args, string(*filter.UserType))
	}

	where := ""
	if len(wheres) > 0 {
		where = "WHERE " + strings.Join(wheres, " AND ")
	}

	// ---- count ----
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM user_rankings %s", where)

	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings count: %w", err)
	}

	// ---- data ----
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	idx++
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", idx)

	idx++
	args = append(args, offset)
	offsetPlaceholder := fmt.Sprintf("$%d", idx)

	dataSQL := fmt.Sprintf(
		`SELECT user_id, wallet_address, user_type, %s, updated_at,
				RANK() OVER (ORDER BY %s DESC) AS rank
		 FROM user_rankings %s
		 ORDER BY rank ASC
		 LIMIT %s OFFSET %s`,
		col, col, where, limitPlaceholder, offsetPlaceholder)

	rows, err := q.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings query: %w", err)
	}
	defer rows.Close()

	var rankings []*domain.UserRanking
	for rows.Next() {
		var (
			uid           string
			walletAddress string
			userType      string
			value         decimal.Decimal
			updatedAt     time.Time
			rank          int
		)

		if err := rows.Scan(&uid, &walletAddress, &userType, &value, &updatedAt, &rank); err != nil {
			return nil, 0, fmt.Errorf("postgres: list rankings scan: %w", err)
		}

		rankings = append(rankings, &domain.UserRanking{
			UserID:        uid,
			WalletAddress: walletAddress,
			UserType:      domain.UserType(userType),
			Dimension:     dim,
			Value:         value,
			Rank:          rank,
			UpdatedAt:     updatedAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings rows: %w", err)
	}

	return rankings, total, nil
}

// RefreshMaterializedView recalculates the user_rankings materialized view.
// CONCURRENTLY is used so that reads are not blocked during the refresh
// (requires a unique index on the view, which is present on user_id).
func (r *RankingRepo) RefreshMaterializedView(ctx context.Context) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY user_rankings`)
	if err != nil {
		return fmt.Errorf("postgres: refresh rankings materialized view: %w", err)
	}

	return nil
}
