package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// UserType distinguishes between human and AI agent participants.
type UserType string

const (
	// UserTypeHuman represents a human user.
	UserTypeHuman UserType = "human"
	// UserTypeAgent represents an AI agent user.
	UserTypeAgent UserType = "agent"
)

// String returns the string representation of a UserType.
func (u UserType) String() string {
	return string(u)
}

// User represents a participant in the prediction market platform.
// Users can place orders, hold positions, and have a wallet balance.
type User struct {
	ID            string
	WalletAddress string
	UserType      UserType
	Balance       decimal.Decimal
	LockedBalance decimal.Decimal
	IsAdmin       bool
	CreatedAt     time.Time
}

// APIKey represents an authentication key issued to a user for programmatic access.
type APIKey struct {
	ID        string
	UserID    string
	KeyHash   string
	KeyPrefix string
	IsActive  bool
	ExpiresAt *time.Time
	CreatedAt time.Time
}
