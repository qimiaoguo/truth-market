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

// MarketRepo implements repository.MarketRepository using PostgreSQL.
type MarketRepo struct {
	BaseRepo
}

// NewMarketRepo creates a new MarketRepo.
func NewMarketRepo(pool *pgxpool.Pool) *MarketRepo {
	return &MarketRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.MarketRepository = (*MarketRepo)(nil)

func (r *MarketRepo) Create(ctx context.Context, market *domain.Market) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO markets (id, title, description, category, market_type, status,
		     created_by, resolved_outcome_id, created_at, updated_at, end_time)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		market.ID,
		market.Title,
		market.Description,
		market.Category,
		string(market.MarketType),
		string(market.Status),
		market.CreatorID,
		market.ResolvedOutcomeID,
		market.CreatedAt,
		market.UpdatedAt,
		market.ClosesAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create market: %w", err)
	}

	return nil
}

func (r *MarketRepo) GetByID(ctx context.Context, id string) (*domain.Market, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, title, description, category, market_type, status,
		        created_by, resolved_outcome_id, created_at, updated_at, end_time
		 FROM markets WHERE id = $1`, id)

	m, err := scanMarket(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: get market by id: %w", err)
	}

	return m, nil
}

func (r *MarketRepo) Update(ctx context.Context, market *domain.Market) error {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE markets SET
			title = $1, description = $2, category = $3, market_type = $4, status = $5,
			created_by = $6, resolved_outcome_id = $7, updated_at = $8, end_time = $9
		 WHERE id = $10`,
		market.Title,
		market.Description,
		market.Category,
		string(market.MarketType),
		string(market.Status),
		market.CreatorID,
		market.ResolvedOutcomeID,
		market.UpdatedAt,
		market.ClosesAt,
		market.ID,
	)
	if err != nil {
		return fmt.Errorf("postgres: update market: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: update market: %w", pgx.ErrNoRows)
	}

	return nil
}

func (r *MarketRepo) List(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	q := r.Querier(ctx)

	var (
		wheres []string
		args   []any
		idx    int
	)

	if filter.Status != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
	}

	if filter.Category != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("category = $%d", idx))
		args = append(args, *filter.Category)
	}

	where := ""
	if len(wheres) > 0 {
		where = "WHERE " + strings.Join(wheres, " AND ")
	}

	// Count.
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM markets %s", where)

	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list markets count: %w", err)
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
		`SELECT id, title, description, category, market_type, status,
		        created_by, resolved_outcome_id, created_at, updated_at, end_time
		 FROM markets %s ORDER BY created_at DESC LIMIT %s OFFSET %s`,
		where, limitPlaceholder, offsetPlaceholder)

	rows, err := q.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list markets query: %w", err)
	}
	defer rows.Close()

	var markets []*domain.Market
	for rows.Next() {
		m, err := scanMarketFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list markets scan: %w", err)
		}
		markets = append(markets, m)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list markets rows: %w", err)
	}

	return markets, total, nil
}

// scanMarket scans a single market from pgx.Row.
func scanMarket(row pgx.Row) (*domain.Market, error) {
	var m domain.Market
	var marketType, status string

	err := row.Scan(
		&m.ID,
		&m.Title,
		&m.Description,
		&m.Category,
		&marketType,
		&status,
		&m.CreatorID,
		&m.ResolvedOutcomeID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.ClosesAt,
	)
	if err != nil {
		return nil, err
	}

	m.MarketType = domain.MarketType(marketType)
	m.Status = domain.MarketStatus(status)
	return &m, nil
}

// scanMarketFromRows scans a single market from pgx.Rows.
func scanMarketFromRows(rows pgx.Rows) (*domain.Market, error) {
	var m domain.Market
	var marketType, status string

	err := rows.Scan(
		&m.ID,
		&m.Title,
		&m.Description,
		&m.Category,
		&marketType,
		&status,
		&m.CreatorID,
		&m.ResolvedOutcomeID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.ClosesAt,
	)
	if err != nil {
		return nil, err
	}

	m.MarketType = domain.MarketType(marketType)
	m.Status = domain.MarketStatus(status)
	return &m, nil
}
