package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// MatchingEngine is the interface for the matching engine.
// It allows the order service to place and cancel orders without depending
// on a concrete matching engine implementation.
type MatchingEngine interface {
	PlaceOrder(order *domain.Order) (*MatchResult, error)
	CancelOrder(outcomeID, orderID string) (*domain.Order, error)
}

// MatchResult holds the result of placing an order in the engine.
type MatchResult struct {
	Trades  []*domain.Trade
	Resting *domain.Order
}

// PlaceOrderRequest contains the parameters needed to place a new order.
type PlaceOrderRequest struct {
	UserID    string
	MarketID  string
	OutcomeID string
	Side      domain.OrderSide
	Price     decimal.Decimal
	Quantity  decimal.Decimal
}

// OrderService handles order placement, cancellation, and retrieval.
type OrderService struct {
	orderRepo    repository.OrderRepository
	userRepo     repository.UserRepository
	positionRepo repository.PositionRepository
	tradeRepo    repository.TradeRepository
	txManager    repository.TxManager
	engine       MatchingEngine
}

// NewOrderService creates a new OrderService with the given dependencies.
func NewOrderService(
	orderRepo repository.OrderRepository,
	userRepo repository.UserRepository,
	positionRepo repository.PositionRepository,
	tradeRepo repository.TradeRepository,
	txManager repository.TxManager,
	engine MatchingEngine,
) *OrderService {
	return &OrderService{
		orderRepo:    orderRepo,
		userRepo:     userRepo,
		positionRepo: positionRepo,
		tradeRepo:    tradeRepo,
		txManager:    txManager,
		engine:       engine,
	}
}

// PlaceOrder validates, persists, and submits an order to the matching engine.
// For buy orders, the cost (price * quantity) is deducted from the user's
// available balance and added to locked_balance. For sell orders, the user's
// position quantity is reduced.
func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*domain.Order, []*domain.Trade, error) {
	// Validate price: must be between 0.01 and 0.99 inclusive.
	minPrice := decimal.NewFromFloat(0.01)
	maxPrice := decimal.NewFromFloat(0.99)
	if req.Price.LessThan(minPrice) || req.Price.GreaterThan(maxPrice) {
		return nil, nil, apperrors.ErrInvalidPrice
	}

	var order *domain.Order
	var trades []*domain.Trade

	err := s.txManager.WithTx(ctx, func(ctx context.Context) error {
		now := time.Now()

		// Create the order object.
		order = &domain.Order{
			ID:        uuid.New().String(),
			UserID:    req.UserID,
			MarketID:  req.MarketID,
			OutcomeID: req.OutcomeID,
			Side:      req.Side,
			Price:     req.Price,
			Quantity:  req.Quantity,
			FilledQty: decimal.Zero,
			Status:    domain.OrderStatusOpen,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if req.Side == domain.OrderSideBuy {
			// Buy: cost = price * quantity. Check balance, deduct and lock.
			cost := req.Price.Mul(req.Quantity)

			user, err := s.userRepo.GetByID(ctx, req.UserID)
			if err != nil {
				return err
			}

			if user.Balance.LessThan(cost) {
				return apperrors.ErrInsufficientBalance
			}

			newBalance := user.Balance.Sub(cost)
			newLocked := user.LockedBalance.Add(cost)
			if err := s.userRepo.UpdateBalance(ctx, req.UserID, newBalance, newLocked); err != nil {
				return err
			}
		} else {
			// Sell: check position quantity >= order quantity, reduce position.
			pos, err := s.positionRepo.GetByUserAndOutcome(ctx, req.UserID, req.OutcomeID)
			if err != nil {
				return err
			}

			if pos.Quantity.LessThan(req.Quantity) {
				return apperrors.ErrInsufficientBalance
			}

			pos.Quantity = pos.Quantity.Sub(req.Quantity)
			pos.UpdatedAt = now
			if err := s.positionRepo.Upsert(ctx, pos); err != nil {
				return err
			}
		}

		// Persist the order.
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return err
		}

		// Send to matching engine.
		result, err := s.engine.PlaceOrder(order)
		if err != nil {
			return err
		}

		trades = result.Trades

		// Settle each trade: persist, update balances, positions, and order statuses.
		for _, trade := range trades {
			// 1. Persist the trade.
			if err := s.tradeRepo.Create(ctx, trade); err != nil {
				return err
			}

			// 2. Determine buyer and seller.
			var buyerUserID, sellerUserID string
			var buyerOrderPrice decimal.Decimal
			if order.Side == domain.OrderSideBuy {
				// Taker is buyer, maker is seller.
				buyerUserID = trade.TakerUserID
				sellerUserID = trade.MakerUserID
				buyerOrderPrice = order.Price
			} else {
				// Taker is seller, maker is buyer.
				// Trade executes at maker price, so trade.Price == maker's order price.
				buyerUserID = trade.MakerUserID
				sellerUserID = trade.TakerUserID
				buyerOrderPrice = trade.Price
			}

			// 3. Buyer settlement: release locked funds, refund favorable price diff, create position.
			buyerUser, err := s.userRepo.GetByID(ctx, buyerUserID)
			if err != nil {
				return err
			}
			lockedReduction := buyerOrderPrice.Mul(trade.Quantity)
			refund := buyerOrderPrice.Sub(trade.Price).Mul(trade.Quantity)
			newBuyerBalance := buyerUser.Balance.Add(refund)
			newBuyerLocked := buyerUser.LockedBalance.Sub(lockedReduction)
			if err := s.userRepo.UpdateBalance(ctx, buyerUserID, newBuyerBalance, newBuyerLocked); err != nil {
				return err
			}

			// Create or update buyer's position.
			buyerPos, err := s.positionRepo.GetByUserAndOutcome(ctx, buyerUserID, trade.OutcomeID)
			if err != nil {
				// Position doesn't exist yet — create a new one.
				buyerPos = &domain.Position{
					ID:        uuid.New().String(),
					UserID:    buyerUserID,
					MarketID:  trade.MarketID,
					OutcomeID: trade.OutcomeID,
					Quantity:  decimal.Zero,
					AvgPrice:  decimal.Zero,
					UpdatedAt: now,
				}
			}
			// Update average price: weighted average.
			totalCost := buyerPos.AvgPrice.Mul(buyerPos.Quantity).Add(trade.Price.Mul(trade.Quantity))
			buyerPos.Quantity = buyerPos.Quantity.Add(trade.Quantity)
			if buyerPos.Quantity.GreaterThan(decimal.Zero) {
				buyerPos.AvgPrice = totalCost.Div(buyerPos.Quantity)
			}
			buyerPos.UpdatedAt = now
			if err := s.positionRepo.Upsert(ctx, buyerPos); err != nil {
				return err
			}

			// 4. Seller settlement: credit proceeds.
			sellerUser, err := s.userRepo.GetByID(ctx, sellerUserID)
			if err != nil {
				return err
			}
			proceeds := trade.Price.Mul(trade.Quantity)
			newSellerBalance := sellerUser.Balance.Add(proceeds)
			if err := s.userRepo.UpdateBalance(ctx, sellerUserID, newSellerBalance, sellerUser.LockedBalance); err != nil {
				return err
			}

			// 5. Update maker order status in DB.
			// Look up the maker order to get its current filled qty from the engine.
			makerOrder, err := s.orderRepo.GetByID(ctx, trade.MakerOrderID)
			if err != nil {
				return err
			}
			makerNewFilled := makerOrder.FilledQty.Add(trade.Quantity)
			makerStatus := domain.OrderStatusPartial
			if makerNewFilled.GreaterThanOrEqual(makerOrder.Quantity) {
				makerStatus = domain.OrderStatusFilled
			}
			if err := s.orderRepo.UpdateStatus(ctx, trade.MakerOrderID, makerStatus, makerNewFilled); err != nil {
				return err
			}
		}

		// 6. Update taker order status in DB if it changed (partially or fully filled).
		if order.FilledQty.GreaterThan(decimal.Zero) {
			if err := s.orderRepo.UpdateStatus(ctx, order.ID, order.Status, order.FilledQty); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return order, trades, nil
}

// CancelOrder cancels an existing order. It verifies ownership, cancels
// in the matching engine, and for buy orders releases the locked funds back
// to the user's available balance.
func (s *OrderService) CancelOrder(ctx context.Context, userID, orderID string) error {
	// Get the order from the repo.
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}

	// Check ownership.
	if order.UserID != userID {
		return apperrors.ErrForbidden
	}

	// Cancel in the matching engine.
	_, err = s.engine.CancelOrder(order.OutcomeID, orderID)
	if err != nil {
		return err
	}

	// For buy orders, release locked funds.
	if order.Side == domain.OrderSideBuy {
		unfilledQty := order.Quantity.Sub(order.FilledQty)
		unfilledCost := unfilledQty.Mul(order.Price)

		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return err
		}

		newBalance := user.Balance.Add(unfilledCost)
		newLocked := user.LockedBalance.Sub(unfilledCost)
		if err := s.userRepo.UpdateBalance(ctx, userID, newBalance, newLocked); err != nil {
			return err
		}
	}

	// Update order status to cancelled.
	if err := s.orderRepo.UpdateStatus(ctx, orderID, domain.OrderStatusCancelled, order.FilledQty); err != nil {
		return err
	}

	return nil
}

// GetOrder retrieves an order by its ID.
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*domain.Order, error) {
	return s.orderRepo.GetByID(ctx, orderID)
}

// ListOrdersFilter specifies the filter criteria for listing orders.
type ListOrdersFilter struct {
	UserID   string
	MarketID string
	Status   domain.OrderStatus
	Limit    int
	Offset   int
}

// ListOrders returns orders matching the given filter criteria.
// If UserID is provided, orders are filtered by user.
// The MarketID and Status fields provide additional filtering.
func (s *OrderService) ListOrders(ctx context.Context, filter ListOrdersFilter) ([]*domain.Order, int64, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Use the repository to fetch orders by user with pagination.
	orders, _, err := s.orderRepo.ListByUser(ctx, filter.UserID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	// Apply additional in-memory filters for market and status.
	var filtered []*domain.Order
	for _, o := range orders {
		if filter.MarketID != "" && o.MarketID != filter.MarketID {
			continue
		}
		if filter.Status != "" && o.Status != filter.Status {
			continue
		}
		filtered = append(filtered, o)
	}

	// Adjust total to reflect the filtered count. Since the repository
	// pagination is user-level and we apply additional filters in-memory,
	// the most accurate total for the caller is the filtered length.
	return filtered, int64(len(filtered)), nil
}

// CancelAllOrdersByMarket cancels all open orders for a given market.
// For buy orders, locked funds are released back to the user's available balance.
// For sell orders, the unfilled quantity is restored to the user's position.
func (s *OrderService) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	orders, err := s.orderRepo.ListOpenByMarket(ctx, marketID)
	if err != nil {
		return 0, err
	}

	if len(orders) == 0 {
		return 0, nil
	}

	var count int64
	for _, order := range orders {
		// Cancel in the matching engine.
		_, err := s.engine.CancelOrder(order.OutcomeID, order.ID)
		if err != nil {
			return 0, err
		}

		unfilledQty := order.Quantity.Sub(order.FilledQty)

		if order.Side == domain.OrderSideBuy {
			// Release locked funds back to user's available balance.
			unfilledCost := unfilledQty.Mul(order.Price)

			user, err := s.userRepo.GetByID(ctx, order.UserID)
			if err != nil {
				return 0, err
			}

			newBalance := user.Balance.Add(unfilledCost)
			newLocked := user.LockedBalance.Sub(unfilledCost)
			if err := s.userRepo.UpdateBalance(ctx, order.UserID, newBalance, newLocked); err != nil {
				return 0, err
			}
		} else if order.Side == domain.OrderSideSell {
			// Restore unfilled quantity to user's position.
			pos, err := s.positionRepo.GetByUserAndOutcome(ctx, order.UserID, order.OutcomeID)
			if err != nil {
				return 0, err
			}

			pos.Quantity = pos.Quantity.Add(unfilledQty)
			pos.UpdatedAt = time.Now()
			if err := s.positionRepo.Upsert(ctx, pos); err != nil {
				return 0, err
			}
		}

		// Update order status to cancelled.
		if err := s.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCancelled, order.FilledQty); err != nil {
			return 0, err
		}

		count++
	}

	return count, nil
}
