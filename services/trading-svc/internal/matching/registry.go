package matching

import (
	"sync"

	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/service"

	grpcserver "github.com/truthmarket/truth-market/services/trading-svc/internal/grpc"
)

// Registry manages a set of per-market matching engines and provides
// adapters for the service.MatchingEngine and grpc.OrderbookProvider
// interfaces so that a single value can be injected into both layers.
type Registry struct {
	mu      sync.RWMutex
	engines map[string]*Engine // marketID -> Engine
}

// NewRegistry creates an empty engine registry.
func NewRegistry() *Registry {
	return &Registry{
		engines: make(map[string]*Engine),
	}
}

// compile-time interface checks
var (
	_ service.MatchingEngine      = (*Registry)(nil)
	_ grpcserver.OrderbookProvider = (*Registry)(nil)
)

// getOrCreate returns the engine for the given marketID, creating one lazily if
// it does not yet exist.
func (r *Registry) getOrCreate(marketID string) *Engine {
	r.mu.RLock()
	eng, ok := r.engines[marketID]
	r.mu.RUnlock()
	if ok {
		return eng
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	eng, ok = r.engines[marketID]
	if ok {
		return eng
	}

	eng = NewEngine(marketID)
	r.engines[marketID] = eng
	return eng
}

// PlaceOrder delegates to the per-market engine and converts the result to
// the service-layer MatchResult type.
func (r *Registry) PlaceOrder(order *domain.Order) (*service.MatchResult, error) {
	eng := r.getOrCreate(order.MarketID)

	result, err := eng.PlaceOrder(order)
	if err != nil {
		return nil, err
	}

	return &service.MatchResult{
		Trades:  result.Trades,
		Resting: result.Resting,
	}, nil
}

// CancelOrder delegates to the per-market engine identified by the outcomeID.
// Because outcome-to-market mapping is implicit within each engine, we search
// all engines. This is acceptable because the number of active markets is
// bounded and the lookup is fast (RLock + map probe per engine).
func (r *Registry) CancelOrder(outcomeID, orderID string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, eng := range r.engines {
		order, err := eng.CancelOrder(outcomeID, orderID)
		if err == nil {
			return order, nil
		}
	}

	return nil, errOrderNotFound
}

// GetOrderbookDepth satisfies grpc.OrderbookProvider. It searches all engines
// for the orderbook matching the given outcomeID and returns aggregated depth.
func (r *Registry) GetOrderbookDepth(outcomeID string, levels int) (bids, asks []grpcserver.OrderbookLevel) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, eng := range r.engines {
		ob := eng.GetOrderbook(outcomeID)
		if ob == nil {
			continue
		}

		bidLevels, askLevels := ob.GetDepth(levels)

		bids = make([]grpcserver.OrderbookLevel, len(bidLevels))
		for i, l := range bidLevels {
			bids[i] = grpcserver.OrderbookLevel{
				Price:    l.Price,
				Quantity: l.Quantity,
				Count:    l.Count,
			}
		}

		asks = make([]grpcserver.OrderbookLevel, len(askLevels))
		for i, l := range askLevels {
			asks[i] = grpcserver.OrderbookLevel{
				Price:    l.Price,
				Quantity: l.Quantity,
				Count:    l.Count,
			}
		}

		return bids, asks
	}

	return nil, nil
}
