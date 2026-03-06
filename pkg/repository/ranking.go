package repository

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// RankingFilter holds optional criteria for listing user rankings.
type RankingFilter struct {
	Dimension *domain.RankDimension
	UserType  *domain.UserType
	Limit     int
	Offset    int
}

// RankingRepository defines persistence operations for user ranking data,
// typically backed by a materialized view for efficient leaderboard queries.
type RankingRepository interface {
	// Upsert creates or updates a ranking record for a user in a given
	// dimension.
	Upsert(ctx context.Context, ranking *domain.UserRanking) error

	// GetByUser returns all ranking records for the specified user across every
	// dimension.
	GetByUser(ctx context.Context, userID string) ([]*domain.UserRanking, error)

	// List returns paginated rankings matching the given filter along with the
	// total count.
	List(ctx context.Context, filter RankingFilter) ([]*domain.UserRanking, int64, error)

	// RefreshMaterializedView recalculates the ranking materialized view so
	// that subsequent queries reflect the latest data.
	RefreshMaterializedView(ctx context.Context) error
}
