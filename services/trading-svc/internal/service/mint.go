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

// MintService handles the minting of complete sets of outcome tokens for a market.
// When a user mints tokens, they pay the cost (equal to the quantity for a 1:1 complete set)
// and receive an equal quantity of each outcome token.
type MintService struct {
	userRepo     repository.UserRepository
	outcomeRepo  repository.OutcomeRepository
	positionRepo repository.PositionRepository
	tradeRepo    repository.TradeRepository
	txManager    repository.TxManager
}

// NewMintService creates a new MintService with the given dependencies.
func NewMintService(
	userRepo repository.UserRepository,
	outcomeRepo repository.OutcomeRepository,
	positionRepo repository.PositionRepository,
	tradeRepo repository.TradeRepository,
	txManager repository.TxManager,
) *MintService {
	return &MintService{
		userRepo:     userRepo,
		outcomeRepo:  outcomeRepo,
		positionRepo: positionRepo,
		tradeRepo:    tradeRepo,
		txManager:    txManager,
	}
}

// MintTokens creates a complete set of outcome tokens for the given market.
// The cost is equal to the quantity (1:1 ratio for a complete set).
// All operations are performed within a single transaction.
func (s *MintService) MintTokens(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error) {
	var positions []*domain.Position

	err := s.txManager.WithTx(ctx, func(ctx context.Context) error {
		// Get user and check balance.
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return err
		}

		// Cost is equal to quantity for a 1:1 complete set.
		cost := quantity
		if user.Balance.LessThan(cost) {
			return apperrors.ErrInsufficientBalance
		}

		// Get outcomes for the market.
		outcomes, err := s.outcomeRepo.ListByMarket(ctx, marketID)
		if err != nil {
			return err
		}

		// For each outcome, upsert a position.
		now := time.Now()
		for _, outcome := range outcomes {
			pos := &domain.Position{
				ID:        uuid.New().String(),
				UserID:    userID,
				MarketID:  marketID,
				OutcomeID: outcome.ID,
				Quantity:  quantity,
				AvgPrice:  decimal.NewFromInt(1),
				UpdatedAt: now,
			}
			if err := s.positionRepo.Upsert(ctx, pos); err != nil {
				return err
			}
			positions = append(positions, pos)
		}

		// Deduct user balance by cost, keep locked_balance unchanged.
		newBalance := user.Balance.Sub(cost)
		if err := s.userRepo.UpdateBalance(ctx, userID, newBalance, user.LockedBalance); err != nil {
			return err
		}

		// Record the mint transaction.
		mintTx := &domain.MintTransaction{
			ID:        uuid.New().String(),
			UserID:    userID,
			MarketID:  marketID,
			Quantity:  quantity,
			Cost:      cost,
			CreatedAt: now,
		}
		if err := s.tradeRepo.CreateMintTx(ctx, mintTx); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return positions, nil
}
