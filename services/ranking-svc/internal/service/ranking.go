package service

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// RankingService implements the core business logic for leaderboard and ranking queries.
type RankingService struct {
	rankingRepo repository.RankingRepository
	userRepo    repository.UserRepository
}

// NewRankingService constructs a RankingService with the given dependencies.
func NewRankingService(rankingRepo repository.RankingRepository, userRepo repository.UserRepository) *RankingService {
	return &RankingService{
		rankingRepo: rankingRepo,
		userRepo:    userRepo,
	}
}

// GetLeaderboard returns a paginated list of rankings for a given dimension,
// optionally filtered by user type. page is 1-based.
func (s *RankingService) GetLeaderboard(ctx context.Context, dimension domain.RankDimension, userType *domain.UserType, page, perPage int) ([]*domain.UserRanking, int64, error) {
	// Convert 1-based page to offset/limit.
	offset := (page - 1) * perPage
	if offset < 0 {
		offset = 0
	}

	filter := repository.RankingFilter{
		Dimension: &dimension,
		UserType:  userType,
		Limit:     perPage,
		Offset:    offset,
	}

	return s.rankingRepo.List(ctx, filter)
}

// GetUserRanking returns all ranking dimensions for a specific user.
func (s *RankingService) GetUserRanking(ctx context.Context, userID string) ([]*domain.UserRanking, error) {
	return s.rankingRepo.GetByUser(ctx, userID)
}

// RefreshRankings triggers a refresh of the underlying materialized view.
func (s *RankingService) RefreshRankings(ctx context.Context) error {
	return s.rankingRepo.RefreshMaterializedView(ctx)
}
