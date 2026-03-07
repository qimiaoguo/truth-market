package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// TradeRepo implements repository.TradeRepository using PostgreSQL.
type TradeRepo struct {
	BaseRepo
}

// NewTradeRepo creates a new TradeRepo.
func NewTradeRepo(pool *pgxpool.Pool) *TradeRepo {
	return &TradeRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.TradeRepository = (*TradeRepo)(nil)

func (r *TradeRepo) Create(ctx context.Context, trade *domain.Trade) error {
	err := r.Q(ctx).CreateTrade(ctx, sqlcgen.CreateTradeParams{
		ID:           trade.ID,
		MarketID:     trade.MarketID,
		OutcomeID:    trade.OutcomeID,
		MakerOrderID: trade.MakerOrderID,
		TakerOrderID: trade.TakerOrderID,
		MakerUserID:  trade.MakerUserID,
		TakerUserID:  trade.TakerUserID,
		Price:        trade.Price,
		Quantity:     trade.Quantity,
		CreatedAt:    tstz(trade.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create trade: %w", err)
	}
	return nil
}

func (r *TradeRepo) ListByMarket(ctx context.Context, marketID string, limit, offset int) ([]*domain.Trade, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	total, err := r.Q(ctx).CountTradesByMarket(ctx, marketID)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by market count: %w", err)
	}

	rows, err := r.Q(ctx).ListTradesByMarket(ctx, sqlcgen.ListTradesByMarketParams{
		MarketID: marketID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by market query: %w", err)
	}

	return tradesFromRows(rows), total, nil
}

func (r *TradeRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Trade, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	total, err := r.Q(ctx).CountTradesByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by user count: %w", err)
	}

	rows, err := r.Q(ctx).ListTradesByUser(ctx, sqlcgen.ListTradesByUserParams{
		MakerUserID: userID,
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by user query: %w", err)
	}

	return tradesFromRows(rows), total, nil
}

func (r *TradeRepo) CreateMintTx(ctx context.Context, mintTx *domain.MintTransaction) error {
	err := r.Q(ctx).CreateMintTransaction(ctx, sqlcgen.CreateMintTransactionParams{
		ID:        mintTx.ID,
		UserID:    mintTx.UserID,
		MarketID:  mintTx.MarketID,
		Quantity:  mintTx.Quantity,
		Cost:      mintTx.Cost,
		CreatedAt: tstz(mintTx.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create mint transaction: %w", err)
	}
	return nil
}

func tradeFromRow(r sqlcgen.Trade) *domain.Trade {
	return &domain.Trade{
		ID:           r.ID,
		MarketID:     r.MarketID,
		OutcomeID:    r.OutcomeID,
		MakerOrderID: r.MakerOrderID,
		TakerOrderID: r.TakerOrderID,
		MakerUserID:  r.MakerUserID,
		TakerUserID:  r.TakerUserID,
		Price:        r.Price,
		Quantity:     r.Quantity,
		CreatedAt:    r.CreatedAt.Time,
	}
}

func tradesFromRows(rows []sqlcgen.Trade) []*domain.Trade {
	trades := make([]*domain.Trade, len(rows))
	for i, row := range rows {
		trades[i] = tradeFromRow(row)
	}
	return trades
}
