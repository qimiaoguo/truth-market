package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// RankDimension defines the metric by which users are ranked on the leaderboard.
type RankDimension string

const (
	// RankDimensionTotalAssets ranks users by their total asset value (balance + positions).
	RankDimensionTotalAssets RankDimension = "total_assets"
	// RankDimensionPnL ranks users by their realized and unrealized profit and loss.
	RankDimensionPnL RankDimension = "pnl"
	// RankDimensionVolume ranks users by their total trading volume.
	RankDimensionVolume RankDimension = "volume"
	// RankDimensionWinRate ranks users by their percentage of winning positions.
	RankDimensionWinRate RankDimension = "win_rate"
	// RankDimensionTradeCount ranks users by their total number of executed trades.
	RankDimensionTradeCount RankDimension = "trade_count"
)

// String returns the string representation of a RankDimension.
func (d RankDimension) String() string {
	return string(d)
}

// UserRanking represents a user's rank on a specific leaderboard dimension.
// Rankings are periodically recalculated and stored for efficient retrieval.
type UserRanking struct {
	UserID        string
	WalletAddress string
	UserType      UserType
	Dimension     RankDimension
	Value         decimal.Decimal
	Rank          int
	UpdatedAt     time.Time
}
