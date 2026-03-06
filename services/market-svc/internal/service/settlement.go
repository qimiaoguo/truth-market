package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/eventbus"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// OrderCanceller defines the interface for cancelling all open orders in a
// market. This is typically implemented by the trading service.
type OrderCanceller interface {
	CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error)
}

// SettlementService handles market resolution and cancellation, including
// payout distribution to winning position holders and refunds on cancellation.
type SettlementService struct {
	marketRepo     repository.MarketRepository
	outcomeRepo    repository.OutcomeRepository
	positionRepo   repository.PositionRepository
	userRepo       repository.UserRepository
	orderCanceller OrderCanceller
	txManager      repository.TxManager
	eventBus       eventbus.EventBus
}

// NewSettlementService constructs a SettlementService with the given
// dependencies.
func NewSettlementService(
	marketRepo repository.MarketRepository,
	outcomeRepo repository.OutcomeRepository,
	positionRepo repository.PositionRepository,
	userRepo repository.UserRepository,
	orderCanceller OrderCanceller,
	txManager repository.TxManager,
	eventBus eventbus.EventBus,
) *SettlementService {
	return &SettlementService{
		marketRepo:     marketRepo,
		outcomeRepo:    outcomeRepo,
		positionRepo:   positionRepo,
		userRepo:       userRepo,
		orderCanceller: orderCanceller,
		txManager:      txManager,
		eventBus:       eventBus,
	}
}

// ResolveMarket resolves a closed market by setting the winning outcome and
// paying out $1 per token to holders of the winning outcome. The market must
// be in "closed" status and the winning outcome must belong to the market.
// All open orders are cancelled before the transactional resolution begins.
// After a successful resolution, a MarketResolved event is published.
func (s *SettlementService) ResolveMarket(ctx context.Context, marketID, winningOutcomeID string) error {
	market, err := s.marketRepo.GetByID(ctx, marketID)
	if err != nil {
		return err
	}

	if market.Status != domain.MarketStatusClosed {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("market must be closed to resolve, current status: %s", market.Status))
	}

	// Verify the winning outcome belongs to this market.
	outcomes, err := s.outcomeRepo.ListByMarket(ctx, marketID)
	if err != nil {
		return err
	}

	found := false
	for _, o := range outcomes {
		if o.ID == winningOutcomeID {
			found = true
			break
		}
	}
	if !found {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("outcome %s does not belong to market %s", winningOutcomeID, marketID))
	}

	// Cancel all open orders before entering the transaction.
	if _, err := s.orderCanceller.CancelAllOrdersByMarket(ctx, marketID); err != nil {
		return err
	}

	// Persist resolution and pay out winners inside a transaction.
	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.outcomeRepo.SetWinner(txCtx, winningOutcomeID); err != nil {
			return err
		}

		market.Status = domain.MarketStatusResolved
		market.ResolvedOutcomeID = &winningOutcomeID
		market.UpdatedAt = time.Now()

		if err := s.marketRepo.Update(txCtx, market); err != nil {
			return err
		}

		// List all positions for this market and pay out winners.
		positions, err := s.positionRepo.ListByMarket(txCtx, marketID)
		if err != nil {
			return err
		}

		for _, pos := range positions {
			// Only holders of the winning outcome with positive quantity get paid.
			if pos.OutcomeID != winningOutcomeID || !pos.Quantity.IsPositive() {
				continue
			}

			// Payout is $1 per token (quantity as decimal).
			payout := pos.Quantity

			user, err := s.userRepo.GetByID(txCtx, pos.UserID)
			if err != nil {
				return err
			}

			newBalance := user.Balance.Add(payout)
			if err := s.userRepo.UpdateBalance(txCtx, user.ID, newBalance, user.LockedBalance); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Publish the MarketResolved event after successful transaction.
	payload, err := json.Marshal(map[string]string{
		"market_id":          marketID,
		"winning_outcome_id": winningOutcomeID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	event := domain.DomainEvent{
		ID:        generateID(),
		Type:      domain.EventMarketResolved,
		Source:    "market-svc",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	_ = s.eventBus.Publish(ctx, eventbus.TopicMarketResolved, event)

	return nil
}

// CancelMarket cancels a closed market and refunds all position holders at
// their average purchase price. All open orders are cancelled before the
// transactional cancellation begins.
func (s *SettlementService) CancelMarket(ctx context.Context, marketID string) error {
	market, err := s.marketRepo.GetByID(ctx, marketID)
	if err != nil {
		return err
	}

	if market.Status != domain.MarketStatusClosed {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("market must be closed to cancel, current status: %s", market.Status))
	}

	// Cancel all open orders before entering the transaction.
	if _, err := s.orderCanceller.CancelAllOrdersByMarket(ctx, marketID); err != nil {
		return err
	}

	// Persist cancellation and issue refunds inside a transaction.
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		market.Status = domain.MarketStatusCancelled
		market.UpdatedAt = time.Now()

		if err := s.marketRepo.Update(txCtx, market); err != nil {
			return err
		}

		// List all positions for this market and refund at avg price.
		positions, err := s.positionRepo.ListByMarket(txCtx, marketID)
		if err != nil {
			return err
		}

		for _, pos := range positions {
			if !pos.Quantity.IsPositive() {
				continue
			}

			// Refund = quantity * avgPrice.
			refund := pos.Quantity.Mul(pos.AvgPrice)
			if refund.LessThanOrEqual(decimal.Zero) {
				continue
			}

			user, err := s.userRepo.GetByID(txCtx, pos.UserID)
			if err != nil {
				return err
			}

			newBalance := user.Balance.Add(refund)
			if err := s.userRepo.UpdateBalance(txCtx, user.ID, newBalance, user.LockedBalance); err != nil {
				return err
			}
		}

		return nil
	})
}
