package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
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
	err := r.Q(ctx).CreateMarket(ctx, sqlcgen.CreateMarketParams{
		ID:                market.ID,
		Title:             market.Title,
		Description:       market.Description,
		Category:          market.Category,
		MarketType:        string(market.MarketType),
		Status:            string(market.Status),
		CreatedBy:         market.CreatorID,
		ResolvedOutcomeID: uuidFromOptionalString(market.ResolvedOutcomeID),
		CreatedAt:         tstz(market.CreatedAt),
		UpdatedAt:         tstz(market.UpdatedAt),
		EndTime:           tstzFromOptional(market.ClosesAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create market: %w", err)
	}
	return nil
}

func (r *MarketRepo) GetByID(ctx context.Context, id string) (*domain.Market, error) {
	row, err := r.Q(ctx).GetMarketByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: get market by id: %w", err)
	}
	return marketFromGetRow(row), nil
}

func (r *MarketRepo) Update(ctx context.Context, market *domain.Market) error {
	n, err := r.Q(ctx).UpdateMarket(ctx, sqlcgen.UpdateMarketParams{
		Title:             market.Title,
		Description:       market.Description,
		Category:          market.Category,
		MarketType:        string(market.MarketType),
		Status:            string(market.Status),
		CreatedBy:         market.CreatorID,
		ResolvedOutcomeID: uuidFromOptionalString(market.ResolvedOutcomeID),
		UpdatedAt:         tstz(market.UpdatedAt),
		EndTime:           tstzFromOptional(market.ClosesAt),
		ID:                market.ID,
	})
	if err != nil {
		return fmt.Errorf("postgres: update market: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: update market: %w", pgx.ErrNoRows)
	}
	return nil
}

func (r *MarketRepo) List(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	status := pgtype.Text{}
	if filter.Status != nil {
		status = textFromString(string(*filter.Status))
	}
	category := pgtype.Text{}
	if filter.Category != nil {
		category = textFromString(*filter.Category)
	}

	total, err := r.Q(ctx).CountMarkets(ctx, sqlcgen.CountMarketsParams{
		Status:   status,
		Category: category,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list markets count: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.Q(ctx).ListMarkets(ctx, sqlcgen.ListMarketsParams{
		Limit:    int32(limit),
		Offset:   int32(offset),
		Status:   status,
		Category: category,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list markets query: %w", err)
	}

	markets := make([]*domain.Market, len(rows))
	for i, row := range rows {
		markets[i] = &domain.Market{
			ID:                row.ID,
			Title:             row.Title,
			Description:       row.Description,
			Category:          row.Category,
			MarketType:        domain.MarketType(row.MarketType),
			Status:            domain.MarketStatus(row.Status),
			CreatorID:         row.CreatedBy,
			ResolvedOutcomeID: optionalStringFromUUID(row.ResolvedOutcomeID),
			CreatedAt:         row.CreatedAt.Time,
			UpdatedAt:         row.UpdatedAt.Time,
			ClosesAt:          optionalTimeFromTstz(row.EndTime),
		}
	}
	return markets, total, nil
}

func marketFromGetRow(r sqlcgen.GetMarketByIDRow) *domain.Market {
	return &domain.Market{
		ID:                r.ID,
		Title:             r.Title,
		Description:       r.Description,
		Category:          r.Category,
		MarketType:        domain.MarketType(r.MarketType),
		Status:            domain.MarketStatus(r.Status),
		CreatorID:         r.CreatedBy,
		ResolvedOutcomeID: optionalStringFromUUID(r.ResolvedOutcomeID),
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
		ClosesAt:          optionalTimeFromTstz(r.EndTime),
	}
}
