package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (r *RankingRepo) Upsert(ctx context.Context, ranking *domain.UserRanking) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO user_rankings (user_id, user_type, dimension, value, rank, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, dimension) DO UPDATE SET
			user_type = EXCLUDED.user_type,
			value = EXCLUDED.value,
			rank = EXCLUDED.rank,
			updated_at = EXCLUDED.updated_at`,
		ranking.UserID,
		string(ranking.UserType),
		string(ranking.Dimension),
		ranking.Value,
		ranking.Rank,
		ranking.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: upsert ranking: %w", err)
	}

	return nil
}

func (r *RankingRepo) GetByUser(ctx context.Context, userID string) ([]*domain.UserRanking, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT user_id, user_type, dimension, value, rank, updated_at
		 FROM user_rankings WHERE user_id = $1
		 ORDER BY dimension ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: get rankings by user: %w", err)
	}
	defer rows.Close()

	var rankings []*domain.UserRanking
	for rows.Next() {
		rk, err := scanRankingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: get rankings by user scan: %w", err)
		}
		rankings = append(rankings, rk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get rankings by user rows: %w", err)
	}

	return rankings, nil
}

func (r *RankingRepo) List(ctx context.Context, filter repository.RankingFilter) ([]*domain.UserRanking, int64, error) {
	q := r.Querier(ctx)

	var (
		wheres []string
		args   []any
		idx    int
	)

	if filter.Dimension != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("dimension = $%d", idx))
		args = append(args, string(*filter.Dimension))
	}

	if filter.UserType != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("user_type = $%d", idx))
		args = append(args, string(*filter.UserType))
	}

	where := ""
	if len(wheres) > 0 {
		where = "WHERE " + strings.Join(wheres, " AND ")
	}

	// Count.
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM user_rankings %s", where)

	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings count: %w", err)
	}

	// Data.
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
		`SELECT user_id, user_type, dimension, value, rank, updated_at
		 FROM user_rankings %s ORDER BY rank ASC LIMIT %s OFFSET %s`,
		where, limitPlaceholder, offsetPlaceholder)

	rows, err := q.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings query: %w", err)
	}
	defer rows.Close()

	var rankings []*domain.UserRanking
	for rows.Next() {
		rk, err := scanRankingFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list rankings scan: %w", err)
		}
		rankings = append(rankings, rk)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list rankings rows: %w", err)
	}

	return rankings, total, nil
}

func (r *RankingRepo) RefreshMaterializedView(ctx context.Context) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY user_rankings_mv`)
	if err != nil {
		return fmt.Errorf("postgres: refresh rankings materialized view: %w", err)
	}

	return nil
}

// scanRankingFromRows scans a single ranking from pgx.Rows.
func scanRankingFromRows(rows pgx.Rows) (*domain.UserRanking, error) {
	var rk domain.UserRanking
	var userType, dimension string

	err := rows.Scan(
		&rk.UserID,
		&userType,
		&dimension,
		&rk.Value,
		&rk.Rank,
		&rk.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rk.UserType = domain.UserType(userType)
	rk.Dimension = domain.RankDimension(dimension)
	return &rk, nil
}
