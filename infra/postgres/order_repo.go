package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// OrderRepo implements repository.OrderRepository using PostgreSQL.
type OrderRepo struct {
	BaseRepo
}

// NewOrderRepo creates a new OrderRepo.
func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.OrderRepository = (*OrderRepo)(nil)

func (r *OrderRepo) Create(ctx context.Context, order *domain.Order) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO orders (id, user_id, market_id, outcome_id, side, price, quantity,
		     filled_quantity, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		order.ID,
		order.UserID,
		order.MarketID,
		order.OutcomeID,
		string(order.Side),
		order.Price,
		order.Quantity,
		order.FilledQty,
		string(order.Status),
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create order: %w", err)
	}

	return nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, user_id, market_id, outcome_id, side, price, quantity,
		        filled_quantity, status, created_at, updated_at
		 FROM orders WHERE id = $1`, id)

	o, err := scanOrder(row)
	if err != nil {
		return nil, fmt.Errorf("postgres: get order by id: %w", err)
	}

	return o, nil
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, filled decimal.Decimal) error {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE orders SET status = $1, filled_quantity = $2, updated_at = NOW() WHERE id = $3`,
		string(status), filled, id)
	if err != nil {
		return fmt.Errorf("postgres: update order status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: update order status: %w", pgx.ErrNoRows)
	}

	return nil
}

func (r *OrderRepo) ListOpenByMarket(ctx context.Context, marketID string) ([]*domain.Order, error) {
	q := r.Querier(ctx)

	rows, err := q.Query(ctx,
		`SELECT id, user_id, market_id, outcome_id, side, price, quantity,
		        filled_quantity, status, created_at, updated_at
		 FROM orders
		 WHERE market_id = $1 AND status IN ('open', 'partial')
		 ORDER BY price DESC, created_at ASC`, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list open orders by market: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list open orders scan: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list open orders rows: %w", err)
	}

	return orders, nil
}

func (r *OrderRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Order, int64, error) {
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
		`SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list orders by user count: %w", err)
	}

	// Data.
	rows, err := q.Query(ctx,
		`SELECT id, user_id, market_id, outcome_id, side, price, quantity,
		        filled_quantity, status, created_at, updated_at
		 FROM orders WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list orders by user query: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o, err := scanOrderFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list orders by user scan: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list orders by user rows: %w", err)
	}

	return orders, total, nil
}

func (r *OrderRepo) CancelAllByMarket(ctx context.Context, marketID string) (int64, error) {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE orders SET status = 'cancelled', updated_at = NOW()
		 WHERE market_id = $1 AND status IN ('open', 'partial')`, marketID)
	if err != nil {
		return 0, fmt.Errorf("postgres: cancel all orders by market: %w", err)
	}

	return tag.RowsAffected(), nil
}

// scanOrder scans a single order from pgx.Row.
func scanOrder(row pgx.Row) (*domain.Order, error) {
	var o domain.Order
	var side, status string

	err := row.Scan(
		&o.ID,
		&o.UserID,
		&o.MarketID,
		&o.OutcomeID,
		&side,
		&o.Price,
		&o.Quantity,
		&o.FilledQty,
		&status,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	o.Side = domain.OrderSide(side)
	o.Status = domain.OrderStatus(status)
	return &o, nil
}

// scanOrderFromRows scans a single order from pgx.Rows.
func scanOrderFromRows(rows pgx.Rows) (*domain.Order, error) {
	var o domain.Order
	var side, status string

	err := rows.Scan(
		&o.ID,
		&o.UserID,
		&o.MarketID,
		&o.OutcomeID,
		&side,
		&o.Price,
		&o.Quantity,
		&o.FilledQty,
		&status,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	o.Side = domain.OrderSide(side)
	o.Status = domain.OrderStatus(status)
	return &o, nil
}
