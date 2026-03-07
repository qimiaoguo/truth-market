// Package grpc implements the market-svc gRPC transport layer.
//
// MarketServer adapts the MarketServicer business-logic interface to the
// generated marketv1.MarketServiceServer contract. Every method delegates to
// the underlying service, converts domain types to proto messages, and
// translates pkg/errors sentinels to the appropriate gRPC status codes.
package grpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
	marketv1 "github.com/truthmarket/truth-market/proto/gen/go/market/v1"
)

// ---------------------------------------------------------------------------
// Service interface – the gRPC layer depends on this abstraction so that the
// concrete service implementation can be injected (and mocked in tests).
// ---------------------------------------------------------------------------

// CreateMarketRequest holds the input data required to create a new market.
type CreateMarketRequest struct {
	Title         string
	Description   string
	MarketType    domain.MarketType
	Category      string
	OutcomeLabels []string
	EndTime       time.Time
	CreatedBy     string
}

// MarketServicer defines the business operations for market management.
type MarketServicer interface {
	CreateMarket(ctx context.Context, req CreateMarketRequest) (*domain.Market, error)
	GetMarket(ctx context.Context, id string) (*domain.Market, []*domain.Outcome, error)
	ListMarkets(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error)
	UpdateMarketStatus(ctx context.Context, id string, status domain.MarketStatus) error
	ResolveMarket(ctx context.Context, marketID, winningOutcomeID string) error
}

// ---------------------------------------------------------------------------
// MarketServer
// ---------------------------------------------------------------------------

// MarketServer implements marketv1.MarketServiceServer by delegating to
// the MarketServicer interface.
type MarketServer struct {
	marketv1.UnimplementedMarketServiceServer
	marketService MarketServicer
}

// NewMarketServer constructs a MarketServer with the given service dependency.
func NewMarketServer(svc MarketServicer) *MarketServer {
	return &MarketServer{marketService: svc}
}

// ---------------------------------------------------------------------------
// gRPC method implementations
// ---------------------------------------------------------------------------

// CreateMarket creates a new prediction market.
func (s *MarketServer) CreateMarket(ctx context.Context, req *marketv1.CreateMarketRequest) (*marketv1.CreateMarketResponse, error) {
	svcReq := CreateMarketRequest{
		Title:         req.GetTitle(),
		Description:   req.GetDescription(),
		MarketType:    protoMarketTypeToDomain(req.GetMarketType()),
		Category:      req.GetCategory(),
		OutcomeLabels: req.GetOutcomeLabels(),
		CreatedBy:     req.GetCreatedBy(),
	}
	if req.GetEndTime() != nil {
		svcReq.EndTime = req.GetEndTime().AsTime()
	}

	market, err := s.marketService.CreateMarket(ctx, svcReq)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &marketv1.CreateMarketResponse{
		Market: domainMarketToProto(market, nil),
	}, nil
}

// GetMarket retrieves a market and its outcomes.
func (s *MarketServer) GetMarket(ctx context.Context, req *marketv1.GetMarketRequest) (*marketv1.GetMarketResponse, error) {
	market, outcomes, err := s.marketService.GetMarket(ctx, req.GetMarketId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &marketv1.GetMarketResponse{
		Market: domainMarketToProto(market, outcomes),
	}, nil
}

// ListMarkets returns a paginated list of markets.
func (s *MarketServer) ListMarkets(ctx context.Context, req *marketv1.ListMarketsRequest) (*marketv1.ListMarketsResponse, error) {
	filter := repository.MarketFilter{}

	if req.GetStatus() != marketv1.MarketStatus_MARKET_STATUS_UNSPECIFIED {
		ds := protoMarketStatusToDomain(req.GetStatus())
		filter.Status = &ds
	}
	if req.GetCategory() != "" {
		cat := req.GetCategory()
		filter.Category = &cat
	}

	// Convert page/perPage to limit/offset.
	perPage := int(req.GetPerPage())
	page := int(req.GetPage())
	if perPage > 0 {
		filter.Limit = perPage
	}
	if page > 0 {
		filter.Offset = (page - 1) * perPage
	}

	markets, total, err := s.marketService.ListMarkets(ctx, filter)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoMarkets := make([]*marketv1.Market, len(markets))
	for i, m := range markets {
		_, outcomes, err := s.marketService.GetMarket(ctx, m.ID)
		if err != nil {
			protoMarkets[i] = domainMarketToProto(m, nil)
			continue
		}
		protoMarkets[i] = domainMarketToProto(m, outcomes)
	}

	return &marketv1.ListMarketsResponse{
		Markets: protoMarkets,
		Total:   total,
	}, nil
}

// UpdateMarketStatus transitions a market to a new status.
func (s *MarketServer) UpdateMarketStatus(ctx context.Context, req *marketv1.UpdateMarketStatusRequest) (*marketv1.UpdateMarketStatusResponse, error) {
	err := s.marketService.UpdateMarketStatus(ctx, req.GetMarketId(), protoMarketStatusToDomain(req.GetStatus()))
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &marketv1.UpdateMarketStatusResponse{}, nil
}

// ResolveMarket resolves a closed market by selecting the winning outcome.
func (s *MarketServer) ResolveMarket(ctx context.Context, req *marketv1.ResolveMarketRequest) (*marketv1.ResolveMarketResponse, error) {
	err := s.marketService.ResolveMarket(ctx, req.GetMarketId(), req.GetWinningOutcomeId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &marketv1.ResolveMarketResponse{}, nil
}

// CancelMarket cancels a market. (Placeholder for future implementation.)
func (s *MarketServer) CancelMarket(ctx context.Context, req *marketv1.CancelMarketRequest) (*marketv1.CancelMarketResponse, error) {
	err := s.marketService.UpdateMarketStatus(ctx, req.GetMarketId(), domain.MarketStatusCancelled)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &marketv1.CancelMarketResponse{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// domainMarketToProto converts a domain.Market (and optional outcomes) to the
// protobuf Market message.
func domainMarketToProto(m *domain.Market, outcomes []*domain.Outcome) *marketv1.Market {
	if m == nil {
		return nil
	}
	pb := &marketv1.Market{
		Id:          m.ID,
		Title:       m.Title,
		Description: m.Description,
		Category:    m.Category,
		MarketType:  domainMarketTypeToProto(m.MarketType),
		Status:      domainMarketStatusToProto(m.Status),
		CreatedBy:   m.CreatorID,
		CreatedAt:   timestamppb.New(m.CreatedAt),
	}
	if m.ClosesAt != nil {
		pb.EndTime = timestamppb.New(*m.ClosesAt)
	}

	for _, o := range outcomes {
		pb.Outcomes = append(pb.Outcomes, domainOutcomeToProto(o))
	}

	return pb
}

// domainOutcomeToProto converts a domain.Outcome to the protobuf Outcome message.
func domainOutcomeToProto(o *domain.Outcome) *marketv1.Outcome {
	if o == nil {
		return nil
	}
	return &marketv1.Outcome{
		Id:       o.ID,
		MarketId: o.MarketID,
		Label:    o.Label,
		Index:    int32(o.Index),
		IsWinner: o.IsWinner,
	}
}

// protoMarketTypeToDomain converts a proto MarketType to the domain type.
func protoMarketTypeToDomain(t marketv1.MarketType) domain.MarketType {
	switch t {
	case marketv1.MarketType_MARKET_TYPE_BINARY:
		return domain.MarketTypeBinary
	case marketv1.MarketType_MARKET_TYPE_MULTI:
		return domain.MarketTypeMulti
	default:
		return domain.MarketTypeBinary
	}
}

// domainMarketTypeToProto converts a domain MarketType to the proto type.
func domainMarketTypeToProto(t domain.MarketType) marketv1.MarketType {
	switch t {
	case domain.MarketTypeBinary:
		return marketv1.MarketType_MARKET_TYPE_BINARY
	case domain.MarketTypeMulti:
		return marketv1.MarketType_MARKET_TYPE_MULTI
	default:
		return marketv1.MarketType_MARKET_TYPE_UNSPECIFIED
	}
}

// domainMarketStatusToProto converts a domain MarketStatus to the proto status.
func domainMarketStatusToProto(s domain.MarketStatus) marketv1.MarketStatus {
	switch s {
	case domain.MarketStatusDraft:
		return marketv1.MarketStatus_MARKET_STATUS_DRAFT
	case domain.MarketStatusOpen:
		return marketv1.MarketStatus_MARKET_STATUS_OPEN
	case domain.MarketStatusClosed:
		return marketv1.MarketStatus_MARKET_STATUS_CLOSED
	case domain.MarketStatusResolved:
		return marketv1.MarketStatus_MARKET_STATUS_RESOLVED
	case domain.MarketStatusCancelled:
		return marketv1.MarketStatus_MARKET_STATUS_CANCELLED
	default:
		return marketv1.MarketStatus_MARKET_STATUS_UNSPECIFIED
	}
}

// protoMarketStatusToDomain converts a proto MarketStatus to the domain status.
func protoMarketStatusToDomain(s marketv1.MarketStatus) domain.MarketStatus {
	switch s {
	case marketv1.MarketStatus_MARKET_STATUS_DRAFT:
		return domain.MarketStatusDraft
	case marketv1.MarketStatus_MARKET_STATUS_OPEN:
		return domain.MarketStatusOpen
	case marketv1.MarketStatus_MARKET_STATUS_CLOSED:
		return domain.MarketStatusClosed
	case marketv1.MarketStatus_MARKET_STATUS_RESOLVED:
		return domain.MarketStatusResolved
	case marketv1.MarketStatus_MARKET_STATUS_CANCELLED:
		return domain.MarketStatusCancelled
	default:
		return domain.MarketStatusDraft
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
