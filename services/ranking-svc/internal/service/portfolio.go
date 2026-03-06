package service

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// initialBalance is the starting balance every user receives on registration.
var initialBalance = decimal.NewFromInt(1000)

// Portfolio represents a user's full portfolio summary.
type Portfolio struct {
	TotalValue    decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Positions     []PortfolioPosition
}

// PortfolioPosition represents a single position within a user's portfolio.
type PortfolioPosition struct {
	MarketID  string
	OutcomeID string
	Quantity  decimal.Decimal
	AvgPrice  decimal.Decimal
	Value     decimal.Decimal // quantity * avgPrice
}

// PortfolioService handles portfolio aggregation and PnL calculations.
type PortfolioService struct {
	positionRepo repository.PositionRepository
	userRepo     repository.UserRepository
}

// NewPortfolioService constructs a PortfolioService with the given dependencies.
func NewPortfolioService(positionRepo repository.PositionRepository, userRepo repository.UserRepository) *PortfolioService {
	return &PortfolioService{
		positionRepo: positionRepo,
		userRepo:     userRepo,
	}
}

// GetPortfolio returns the user's portfolio summary.
// TotalValue = user.Balance + sum(position.Quantity * position.AvgPrice)
// UnrealizedPnL = TotalValue - 1000 (initial balance)
func (s *PortfolioService) GetPortfolio(ctx context.Context, userID string) (*Portfolio, error) {
	// Get user for balance.
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get user positions.
	positions, err := s.positionRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Aggregate positions.
	totalPositionValue := decimal.Zero
	portfolioPositions := make([]PortfolioPosition, 0, len(positions))

	for _, pos := range positions {
		value := pos.Quantity.Mul(pos.AvgPrice)
		totalPositionValue = totalPositionValue.Add(value)

		portfolioPositions = append(portfolioPositions, PortfolioPosition{
			MarketID:  pos.MarketID,
			OutcomeID: pos.OutcomeID,
			Quantity:  pos.Quantity,
			AvgPrice:  pos.AvgPrice,
			Value:     value,
		})
	}

	totalValue := user.Balance.Add(totalPositionValue)
	unrealizedPnL := totalValue.Sub(initialBalance)

	return &Portfolio{
		TotalValue:    totalValue,
		UnrealizedPnL: unrealizedPnL,
		Positions:     portfolioPositions,
	}, nil
}
