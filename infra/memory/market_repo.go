package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// MarketRepository implements repository.MarketRepository with an in-memory
// map guarded by a read-write mutex for thread safety.
type MarketRepository struct {
	mu      sync.RWMutex
	markets map[string]*domain.Market
}

// NewMarketRepository returns a new empty in-memory MarketRepository.
func NewMarketRepository() *MarketRepository {
	return &MarketRepository{
		markets: make(map[string]*domain.Market),
	}
}

// Create inserts a new market record. Returns an error if a market with the
// same ID already exists.
func (r *MarketRepository) Create(_ context.Context, market *domain.Market) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.markets[market.ID]; exists {
		return fmt.Errorf("market already exists: %s", market.ID)
	}

	stored := *market
	r.markets[market.ID] = &stored
	return nil
}

// GetByID retrieves a market by its unique identifier.
func (r *MarketRepository) GetByID(_ context.Context, id string) (*domain.Market, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	market, ok := r.markets[id]
	if !ok {
		return nil, fmt.Errorf("market not found: %s", id)
	}

	out := *market
	return &out, nil
}

// Update persists changes to an existing market record.
func (r *MarketRepository) Update(_ context.Context, market *domain.Market) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.markets[market.ID]; !exists {
		return fmt.Errorf("market not found: %s", market.ID)
	}

	stored := *market
	r.markets[market.ID] = &stored
	return nil
}

// List returns markets matching the given filter along with the total count.
func (r *MarketRepository) List(_ context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*domain.Market
	for _, market := range r.markets {
		if filter.Status != nil && market.Status != *filter.Status {
			continue
		}
		if filter.Category != nil && market.Category != *filter.Category {
			continue
		}
		out := *market
		matched = append(matched, &out)
	}

	total := int64(len(matched))

	// Apply pagination.
	start := filter.Offset
	if start > len(matched) {
		start = len(matched)
	}
	end := len(matched)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	return matched[start:end], total, nil
}
