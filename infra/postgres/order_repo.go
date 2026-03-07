package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
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
	err := r.Q(ctx).CreateOrder(ctx, sqlcgen.CreateOrderParams{
		ID:             order.ID,
		UserID:         order.UserID,
		MarketID:       order.MarketID,
		OutcomeID:      order.OutcomeID,
		Side:           string(order.Side),
		Price:          order.Price,
		Quantity:       order.Quantity,
		FilledQuantity: order.FilledQty,
		Status:         string(order.Status),
		CreatedAt:      tstz(order.CreatedAt),
		UpdatedAt:      tstz(order.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create order: %w", err)
	}
	return nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	row, err := r.Q(ctx).GetOrderByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: get order by id: %w", err)
	}
	return orderFromModel(row), nil
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, filled decimal.Decimal) error {
	n, err := r.Q(ctx).UpdateOrderStatus(ctx, sqlcgen.UpdateOrderStatusParams{
		Status:         string(status),
		FilledQuantity: filled,
		ID:             id,
	})
	if err != nil {
		return fmt.Errorf("postgres: update order status: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: update order status: %w", pgx.ErrNoRows)
	}
	return nil
}

func (r *OrderRepo) ListOpenByMarket(ctx context.Context, marketID string) ([]*domain.Order, error) {
	rows, err := r.Q(ctx).ListOpenOrdersByMarket(ctx, marketID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list open orders by market: %w", err)
	}
	return ordersFromModels(rows), nil
}

func (r *OrderRepo) ListAllOpen(ctx context.Context) ([]*domain.Order, error) {
	rows, err := r.Q(ctx).ListAllOpenOrders(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list all open orders: %w", err)
	}
	return ordersFromModels(rows), nil
}

func (r *OrderRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Order, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	total, err := r.Q(ctx).CountOrdersByUser(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list orders by user count: %w", err)
	}

	rows, err := r.Q(ctx).ListOrdersByUser(ctx, sqlcgen.ListOrdersByUserParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list orders by user query: %w", err)
	}

	return ordersFromModels(rows), total, nil
}

func (r *OrderRepo) CancelAllByMarket(ctx context.Context, marketID string) (int64, error) {
	n, err := r.Q(ctx).CancelAllOrdersByMarket(ctx, marketID)
	if err != nil {
		return 0, fmt.Errorf("postgres: cancel all orders by market: %w", err)
	}
	return n, nil
}

func orderFromModel(r sqlcgen.Order) *domain.Order {
	return &domain.Order{
		ID:        r.ID,
		UserID:    r.UserID,
		MarketID:  r.MarketID,
		OutcomeID: r.OutcomeID,
		Side:      domain.OrderSide(r.Side),
		Price:     r.Price,
		Quantity:  r.Quantity,
		FilledQty: r.FilledQuantity,
		Status:    domain.OrderStatus(r.Status),
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

func ordersFromModels(rows []sqlcgen.Order) []*domain.Order {
	orders := make([]*domain.Order, len(rows))
	for i, row := range rows {
		orders[i] = orderFromModel(row)
	}
	return orders
}
