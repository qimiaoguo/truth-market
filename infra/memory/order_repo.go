package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// OrderRepository implements repository.OrderRepository with an in-memory map
// guarded by a read-write mutex for thread safety.
type OrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

// NewOrderRepository returns a new empty in-memory OrderRepository.
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders: make(map[string]*domain.Order),
	}
}

// Create inserts a new order record. Returns an error if an order with the
// same ID already exists.
func (r *OrderRepository) Create(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orders[order.ID]; exists {
		return fmt.Errorf("order already exists: %s", order.ID)
	}

	stored := *order
	r.orders[order.ID] = &stored
	return nil
}

// GetByID retrieves an order by its unique identifier.
func (r *OrderRepository) GetByID(_ context.Context, id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[id]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", id)
	}

	out := *order
	return &out, nil
}

// UpdateStatus changes the status of an order and records the filled amount.
func (r *OrderRepository) UpdateStatus(_ context.Context, id string, status domain.OrderStatus, filled decimal.Decimal) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[id]
	if !ok {
		return fmt.Errorf("order not found: %s", id)
	}

	order.Status = status
	order.FilledQty = filled
	order.UpdatedAt = time.Now()
	return nil
}

// ListOpenByMarket returns all open or partially filled orders for the given
// market.
func (r *OrderRepository) ListOpenByMarket(_ context.Context, marketID string) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Order
	for _, order := range r.orders {
		if order.MarketID != marketID {
			continue
		}
		if order.Status != domain.OrderStatusOpen && order.Status != domain.OrderStatusPartial {
			continue
		}
		out := *order
		result = append(result, &out)
	}

	return result, nil
}

// ListAllOpen returns all open or partially filled orders across all markets.
func (r *OrderRepository) ListAllOpen(_ context.Context) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Order
	for _, order := range r.orders {
		if order.Status == domain.OrderStatusOpen || order.Status == domain.OrderStatusPartial {
			out := *order
			result = append(result, &out)
		}
	}

	return result, nil
}

// ListByUser returns paginated orders for a user along with the total count.
func (r *OrderRepository) ListByUser(_ context.Context, userID string, limit, offset int) ([]*domain.Order, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*domain.Order
	for _, order := range r.orders {
		if order.UserID != userID {
			continue
		}
		out := *order
		matched = append(matched, &out)
	}

	total := int64(len(matched))

	// Apply pagination.
	start := offset
	if start > len(matched) {
		start = len(matched)
	}
	end := len(matched)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return matched[start:end], total, nil
}

// CancelAllByMarket cancels every open order in the specified market and
// returns the number of orders affected.
func (r *OrderRepository) CancelAllByMarket(_ context.Context, marketID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	now := time.Now()
	for _, order := range r.orders {
		if order.MarketID != marketID {
			continue
		}
		if order.Status == domain.OrderStatusOpen || order.Status == domain.OrderStatusPartial {
			order.Status = domain.OrderStatusCancelled
			order.UpdatedAt = now
			count++
		}
	}

	return count, nil
}
