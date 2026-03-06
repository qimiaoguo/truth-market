package testutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// ---------------------------------------------------------------------------
// User options
// ---------------------------------------------------------------------------

// UserOption customises a User created by NewUser or NewAgent.
type UserOption func(*domain.User)

// WithBalance sets the user's available balance.
func WithBalance(b float64) UserOption {
	return func(u *domain.User) {
		u.Balance = decimal.NewFromFloat(b)
	}
}

// WithAdmin marks the user as an administrator.
func WithAdmin() UserOption {
	return func(u *domain.User) {
		u.IsAdmin = true
	}
}

// WithWallet sets the user's wallet address.
func WithWallet(addr string) UserOption {
	return func(u *domain.User) {
		u.WalletAddress = addr
	}
}

// ---------------------------------------------------------------------------
// Market options
// ---------------------------------------------------------------------------

// MarketOption customises a Market created by NewMarket.
type MarketOption func(*domain.Market)

// WithCategory sets the market's category.
func WithCategory(c string) MarketOption {
	return func(m *domain.Market) {
		m.Category = c
	}
}

// WithStatus sets the market's lifecycle status.
func WithStatus(s domain.MarketStatus) MarketOption {
	return func(m *domain.Market) {
		m.Status = s
	}
}

// ---------------------------------------------------------------------------
// Factory functions
// ---------------------------------------------------------------------------

// NewUser creates a domain.User with sensible defaults and applies any
// supplied options. The user type defaults to human.
func NewUser(opts ...UserOption) *domain.User {
	now := time.Now().UTC()
	u := &domain.User{
		ID:            uuid.New().String(),
		WalletAddress: "0x" + uuid.New().String()[:20],
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromFloat(1000),
		LockedBalance: decimal.Zero,
		IsAdmin:       false,
		CreatedAt:     now,
	}
	for _, o := range opts {
		o(u)
	}
	return u
}

// NewAgent creates a domain.User whose type is set to agent and applies any
// supplied options.
func NewAgent(opts ...UserOption) *domain.User {
	u := NewUser(opts...)
	u.UserType = domain.UserTypeAgent
	return u
}

// NewMarket creates a domain.Market with sensible defaults. The market type
// defaults to binary, status defaults to open, and closes one week from now.
func NewMarket(creatorID string, opts ...MarketOption) *domain.Market {
	now := time.Now().UTC()
	closes := now.Add(7 * 24 * time.Hour)
	m := &domain.Market{
		ID:          uuid.New().String(),
		Title:       "Will it rain tomorrow?",
		Description: "Resolves YES if any measurable precipitation is recorded.",
		Category:    "weather",
		MarketType:  domain.MarketTypeBinary,
		Status:      domain.MarketStatusOpen,
		CreatorID:   creatorID,
		CreatedAt:   now,
		UpdatedAt:   now,
		ClosesAt:    &closes,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// NewBinaryMarket creates a binary market together with its two canonical
// outcomes (Yes at index 0, No at index 1). This is a convenience wrapper
// around NewMarket for the most common test scenario.
func NewBinaryMarket(creatorID string) (*domain.Market, []*domain.Outcome) {
	m := NewMarket(creatorID)
	outcomes := []*domain.Outcome{
		{
			ID:       uuid.New().String(),
			MarketID: m.ID,
			Label:    "Yes",
			Index:    0,
			IsWinner: false,
		},
		{
			ID:       uuid.New().String(),
			MarketID: m.ID,
			Label:    "No",
			Index:    1,
			IsWinner: false,
		},
	}
	return m, outcomes
}

// NewOrder creates a domain.Order with the given parameters and sensible
// defaults for the remaining fields.
func NewOrder(
	userID, marketID, outcomeID string,
	side domain.OrderSide,
	price, qty float64,
) *domain.Order {
	now := time.Now().UTC()
	return &domain.Order{
		ID:        uuid.New().String(),
		UserID:    userID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      side,
		Price:     decimal.NewFromFloat(price),
		Quantity:  decimal.NewFromFloat(qty),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewTrade creates a domain.Trade with the given parameters.
func NewTrade(
	makerOrderID, takerOrderID,
	makerUserID, takerUserID,
	marketID, outcomeID string,
	price, qty float64,
) *domain.Trade {
	return &domain.Trade{
		ID:           uuid.New().String(),
		MarketID:     marketID,
		OutcomeID:    outcomeID,
		MakerOrderID: makerOrderID,
		TakerOrderID: takerOrderID,
		MakerUserID:  makerUserID,
		TakerUserID:  takerUserID,
		Price:        decimal.NewFromFloat(price),
		Quantity:     decimal.NewFromFloat(qty),
		CreatedAt:    time.Now().UTC(),
	}
}
