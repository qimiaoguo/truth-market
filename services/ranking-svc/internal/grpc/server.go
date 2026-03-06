// Package grpc implements the ranking-svc gRPC transport layer.
//
// RankingServer adapts the RankingServicer and PortfolioServicer business-logic
// interfaces to the generated rankingv1.RankingServiceServer contract. Every
// method delegates to the underlying services, converts domain types to proto
// messages, and translates pkg/errors sentinels to gRPC status codes.
package grpc

import (
	"context"
	"errors"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
)

// ---------------------------------------------------------------------------
// Service interfaces – the gRPC layer depends on these abstractions so that
// concrete service implementations can be injected (and mocked in tests).
// ---------------------------------------------------------------------------

// RankingServicer defines the business operations for ranking queries.
type RankingServicer interface {
	GetLeaderboard(ctx context.Context, dimension domain.RankDimension, userType *domain.UserType, page, perPage int) ([]*domain.UserRanking, int64, error)
	GetUserRanking(ctx context.Context, userID string) ([]*domain.UserRanking, error)
	RefreshRankings(ctx context.Context) error
}

// PortfolioServicer defines the business operations for portfolio queries.
type PortfolioServicer interface {
	GetPortfolio(ctx context.Context, userID string) (*Portfolio, error)
}

// ---------------------------------------------------------------------------
// Transport-layer types
// ---------------------------------------------------------------------------

// Portfolio holds portfolio summary data used by the gRPC transport layer.
type Portfolio struct {
	TotalValue    decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Positions     []PortfolioPosition
}

// PortfolioPosition represents a single position in the portfolio.
type PortfolioPosition struct {
	MarketID  string
	OutcomeID string
	Quantity  decimal.Decimal
	AvgPrice  decimal.Decimal
	Value     decimal.Decimal
}

// ---------------------------------------------------------------------------
// RankingServer
// ---------------------------------------------------------------------------

// RankingServer implements rankingv1.RankingServiceServer by delegating to
// the RankingServicer and PortfolioServicer interfaces.
type RankingServer struct {
	rankingv1.UnimplementedRankingServiceServer
	rankingService   RankingServicer
	portfolioService PortfolioServicer
}

// NewRankingServer constructs a RankingServer with the given service dependencies.
func NewRankingServer(rankingSvc RankingServicer, portfolioSvc PortfolioServicer) *RankingServer {
	return &RankingServer{
		rankingService:   rankingSvc,
		portfolioService: portfolioSvc,
	}
}

// ---------------------------------------------------------------------------
// gRPC method implementations
// ---------------------------------------------------------------------------

// GetLeaderboard returns a paginated leaderboard for the requested dimension.
func (s *RankingServer) GetLeaderboard(ctx context.Context, req *rankingv1.GetLeaderboardRequest) (*rankingv1.GetLeaderboardResponse, error) {
	dimension := protoDimensionToDomain(req.GetDimension())
	userType := protoUserTypeFilterToDomain(req.GetUserType())

	page := int(req.GetPage())
	perPage := int(req.GetPerPage())
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}

	rankings, total, err := s.rankingService.GetLeaderboard(ctx, dimension, userType, page, perPage)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoRankings := make([]*rankingv1.UserRanking, len(rankings))
	for i, r := range rankings {
		protoRankings[i] = domainRankingToProto(r)
	}

	return &rankingv1.GetLeaderboardResponse{
		Rankings: protoRankings,
		Total:    total,
	}, nil
}

// GetUserRanking returns the user's rank across all dimensions.
func (s *RankingServer) GetUserRanking(ctx context.Context, req *rankingv1.GetUserRankingRequest) (*rankingv1.GetUserRankingResponse, error) {
	rankings, err := s.rankingService.GetUserRanking(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	ranks := make([]*rankingv1.DimensionRank, len(rankings))
	for i, r := range rankings {
		ranks[i] = &rankingv1.DimensionRank{
			Dimension: domainDimensionToProto(r.Dimension),
			Rank:      int64(r.Rank),
			Value:     r.Value.String(),
		}
	}

	return &rankingv1.GetUserRankingResponse{
		Ranks: ranks,
	}, nil
}

// GetPortfolio returns the user's portfolio summary and positions.
func (s *RankingServer) GetPortfolio(ctx context.Context, req *rankingv1.GetPortfolioRequest) (*rankingv1.GetPortfolioResponse, error) {
	portfolio, err := s.portfolioService.GetPortfolio(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	positions := make([]*rankingv1.PortfolioPosition, len(portfolio.Positions))
	for i, p := range portfolio.Positions {
		positions[i] = &rankingv1.PortfolioPosition{
			MarketId:  p.MarketID,
			OutcomeId: p.OutcomeID,
			Quantity:  p.Quantity.String(),
			AvgPrice:  p.AvgPrice.String(),
		}
	}

	return &rankingv1.GetPortfolioResponse{
		TotalValue:    portfolio.TotalValue.String(),
		UnrealizedPnl: portfolio.UnrealizedPnL.String(),
		Positions:     positions,
	}, nil
}

// RefreshRankings triggers a recalculation of all rankings.
func (s *RankingServer) RefreshRankings(ctx context.Context, _ *rankingv1.RefreshRankingsRequest) (*rankingv1.RefreshRankingsResponse, error) {
	err := s.rankingService.RefreshRankings(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &rankingv1.RefreshRankingsResponse{
		UpdatedCount: 0,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// domainDimensionToProto converts a domain.RankDimension to the proto enum.
func domainDimensionToProto(d domain.RankDimension) rankingv1.RankDimension {
	switch d {
	case domain.RankDimensionTotalAssets:
		return rankingv1.RankDimension_RANK_DIMENSION_TOTAL_ASSETS
	case domain.RankDimensionPnL:
		return rankingv1.RankDimension_RANK_DIMENSION_PNL
	case domain.RankDimensionVolume:
		return rankingv1.RankDimension_RANK_DIMENSION_VOLUME
	case domain.RankDimensionWinRate:
		return rankingv1.RankDimension_RANK_DIMENSION_WIN_RATE
	default:
		return rankingv1.RankDimension_RANK_DIMENSION_UNSPECIFIED
	}
}

// protoDimensionToDomain converts a proto RankDimension to the domain type.
func protoDimensionToDomain(d rankingv1.RankDimension) domain.RankDimension {
	switch d {
	case rankingv1.RankDimension_RANK_DIMENSION_TOTAL_ASSETS:
		return domain.RankDimensionTotalAssets
	case rankingv1.RankDimension_RANK_DIMENSION_PNL:
		return domain.RankDimensionPnL
	case rankingv1.RankDimension_RANK_DIMENSION_VOLUME:
		return domain.RankDimensionVolume
	case rankingv1.RankDimension_RANK_DIMENSION_WIN_RATE:
		return domain.RankDimensionWinRate
	default:
		return domain.RankDimensionTotalAssets
	}
}

// protoUserTypeFilterToDomain converts a proto UserTypeFilter to a *domain.UserType.
// Returns nil for ALL or UNSPECIFIED (meaning no filter).
func protoUserTypeFilterToDomain(f rankingv1.UserTypeFilter) *domain.UserType {
	switch f {
	case rankingv1.UserTypeFilter_USER_TYPE_FILTER_HUMAN:
		ut := domain.UserTypeHuman
		return &ut
	case rankingv1.UserTypeFilter_USER_TYPE_FILTER_AGENT:
		ut := domain.UserTypeAgent
		return &ut
	default:
		return nil
	}
}

// domainRankingToProto converts a domain.UserRanking to the proto UserRanking message.
func domainRankingToProto(r *domain.UserRanking) *rankingv1.UserRanking {
	if r == nil {
		return nil
	}
	return &rankingv1.UserRanking{
		UserId:    r.UserID,
		UserType:  string(r.UserType),
		Rank:      int64(r.Rank),
		Value:     r.Value.String(),
		UpdatedAt: timestamppb.New(r.UpdatedAt),
	}
}

// toGRPCError translates an application error (pkg/errors.AppError) to the
// corresponding gRPC status error. Unknown error codes fall through to
// codes.Internal.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, err.Error())
	}

	switch appErr.Code {
	case apperrors.ErrNotFound.Code:
		return status.Error(codes.NotFound, appErr.Message)
	case apperrors.ErrUnauthorized.Code:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case apperrors.ErrForbidden.Code:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case apperrors.ErrBadRequest.Code:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case apperrors.ErrConflict.Code:
		return status.Error(codes.AlreadyExists, appErr.Message)
	case apperrors.ErrInternalError.Code:
		return status.Error(codes.Internal, appErr.Message)
	case apperrors.ErrInsufficientBalance.Code:
		return status.Error(codes.FailedPrecondition, appErr.Message)
	case apperrors.ErrMarketClosed.Code:
		return status.Error(codes.FailedPrecondition, appErr.Message)
	case apperrors.ErrInvalidPrice.Code:
		return status.Error(codes.InvalidArgument, appErr.Message)
	default:
		return status.Error(codes.Internal, appErr.Message)
	}
}
