package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO trades (id, market_id, outcome_id, maker_order_id, taker_order_id,
		     maker_user_id, taker_user_id, price, quantity, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		trade.ID,
		trade.MarketID,
		trade.OutcomeID,
		trade.MakerOrderID,
		trade.TakerOrderID,
		trade.MakerUserID,
		trade.TakerUserID,
		trade.Price,
		trade.Quantity,
		trade.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create trade: %w", err)
	}

	return nil
}

func (r *TradeRepo) ListByMarket(ctx context.Context, marketID string, limit, offset int) ([]*domain.Trade, int64, error) {
	q := r.Querier(ctx)

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Count.
	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM trades WHERE market_id = $1`, marketID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by market count: %w", err)
	}

	// Data.
	rows, err := q.Query(ctx,
		`SELECT id, market_id, outcome_id, maker_order_id, taker_order_id,
		        maker_user_id, taker_user_id, price, quantity, created_at
		 FROM trades WHERE market_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, marketID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by market query: %w", err)
	}
	defer rows.Close()

	var trades []*domain.Trade
	for rows.Next() {
		t, err := scanTradeFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list trades by market scan: %w", err)
		}
		trades = append(trades, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by market rows: %w", err)
	}

	return trades, total, nil
}

func (r *TradeRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Trade, int64, error) {
	q := r.Querier(ctx)

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Count.
	var total int64
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM trades WHERE maker_user_id = $1 OR taker_user_id = $1`,
		userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by user count: %w", err)
	}

	// Data.
	rows, err := q.Query(ctx,
		`SELECT id, market_id, outcome_id, maker_order_id, taker_order_id,
		        maker_user_id, taker_user_id, price, quantity, created_at
		 FROM trades WHERE maker_user_id = $1 OR taker_user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by user query: %w", err)
	}
	defer rows.Close()

	var trades []*domain.Trade
	for rows.Next() {
		t, err := scanTradeFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list trades by user scan: %w", err)
		}
		trades = append(trades, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list trades by user rows: %w", err)
	}

	return trades, total, nil
}

func (r *TradeRepo) CreateMintTx(ctx context.Context, mintTx *domain.MintTransaction) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO mint_transactions (id, user_id, market_id, quantity, cost, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		mintTx.ID,
		mintTx.UserID,
		mintTx.MarketID,
		mintTx.Quantity,
		mintTx.Cost,
		mintTx.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create mint transaction: %w", err)
	}

	return nil
}

// scanTradeFromRows scans a single trade from pgx.Rows.
func scanTradeFromRows(rows pgx.Rows) (*domain.Trade, error) {
	var t domain.Trade

	err := rows.Scan(
		&t.ID,
		&t.MarketID,
		&t.OutcomeID,
		&t.MakerOrderID,
		&t.TakerOrderID,
		&t.MakerUserID,
		&t.TakerUserID,
		&t.Price,
		&t.Quantity,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
