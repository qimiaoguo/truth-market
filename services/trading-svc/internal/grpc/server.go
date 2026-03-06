// Package grpc implements the trading-svc gRPC transport layer.
//
// TradingServer adapts the OrderServicer and MintServicer business-logic
// interfaces to the generated tradingv1.TradingServiceServer contract.
// Every method delegates to the underlying services, converts domain types
// to proto messages, and translates pkg/errors sentinels to gRPC status codes.
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
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
)

// ---------------------------------------------------------------------------
// Service interfaces – the gRPC layer depends on these abstractions so that
// concrete service implementations can be injected (and mocked in tests).
// ---------------------------------------------------------------------------

// PlaceOrderRequest holds the input data required to place an order.
type PlaceOrderRequest struct {
	UserID    string
	MarketID  string
	OutcomeID string
	Side      domain.OrderSide
	Price     decimal.Decimal
	Quantity  decimal.Decimal
}

// OrderServicer defines the business operations for order management.
type OrderServicer interface {
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*domain.Order, []*domain.Trade, error)
	CancelOrder(ctx context.Context, userID, orderID string) error
	GetOrder(ctx context.Context, orderID string) (*domain.Order, error)
	CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error)
}

// MintServicer defines the business operations for token minting.
type MintServicer interface {
	MintTokens(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error)
}

// ---------------------------------------------------------------------------
// TradingServer
// ---------------------------------------------------------------------------

// TradingServer implements tradingv1.TradingServiceServer by delegating to
// the OrderServicer and MintServicer interfaces.
type TradingServer struct {
	tradingv1.UnimplementedTradingServiceServer
	orderService OrderServicer
	mintService  MintServicer
}

// NewTradingServer constructs a TradingServer with the given service dependencies.
func NewTradingServer(orderSvc OrderServicer, mintSvc MintServicer) *TradingServer {
	return &TradingServer{
		orderService: orderSvc,
		mintService:  mintSvc,
	}
}

// ---------------------------------------------------------------------------
// gRPC method implementations
// ---------------------------------------------------------------------------

// PlaceOrder places a new limit order on the order book.
func (s *TradingServer) PlaceOrder(ctx context.Context, req *tradingv1.PlaceOrderRequest) (*tradingv1.PlaceOrderResponse, error) {
	price, err := decimal.NewFromString(req.GetPrice())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid price format")
	}
	qty, err := decimal.NewFromString(req.GetQuantity())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid quantity format")
	}

	svcReq := PlaceOrderRequest{
		UserID:    req.GetUserId(),
		MarketID:  req.GetMarketId(),
		OutcomeID: req.GetOutcomeId(),
		Side:      protoOrderSideToDomain(req.GetSide()),
		Price:     price,
		Quantity:  qty,
	}

	order, trades, err := s.orderService.PlaceOrder(ctx, svcReq)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &tradingv1.PlaceOrderResponse{
		Order: domainOrderToProto(order),
	}
	for _, t := range trades {
		resp.Trades = append(resp.Trades, domainTradeToProto(t))
	}

	return resp, nil
}

// CancelOrder cancels an existing order.
func (s *TradingServer) CancelOrder(ctx context.Context, req *tradingv1.CancelOrderRequest) (*tradingv1.CancelOrderResponse, error) {
	err := s.orderService.CancelOrder(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	// Retrieve the cancelled order for the response.
	order, err := s.orderService.GetOrder(ctx, req.GetOrderId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &tradingv1.CancelOrderResponse{
		Order: domainOrderToProto(order),
	}, nil
}

// GetOrder retrieves a single order by ID.
func (s *TradingServer) GetOrder(ctx context.Context, req *tradingv1.GetOrderRequest) (*tradingv1.GetOrderResponse, error) {
	order, err := s.orderService.GetOrder(ctx, req.GetOrderId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &tradingv1.GetOrderResponse{
		Order: domainOrderToProto(order),
	}, nil
}

// ListOrders is a placeholder for listing orders with filters.
func (s *TradingServer) ListOrders(_ context.Context, _ *tradingv1.ListOrdersRequest) (*tradingv1.ListOrdersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListOrders not yet implemented")
}

// MintTokens creates a complete set of outcome tokens for a market.
func (s *TradingServer) MintTokens(ctx context.Context, req *tradingv1.MintTokensRequest) (*tradingv1.MintTokensResponse, error) {
	qty, err := decimal.NewFromString(req.GetQuantity())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid quantity format")
	}

	positions, err := s.mintService.MintTokens(ctx, req.GetUserId(), req.GetMarketId(), qty)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &tradingv1.MintTokensResponse{
		Cost: qty.String(), // cost = quantity for a complete set
	}
	for _, p := range positions {
		resp.Positions = append(resp.Positions, domainPositionToProto(p))
	}

	return resp, nil
}

// GetPositions retrieves user positions for a market.
func (s *TradingServer) GetPositions(_ context.Context, _ *tradingv1.GetPositionsRequest) (*tradingv1.GetPositionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetPositions not yet implemented")
}

// GetOrderbook retrieves the order book depth for an outcome.
func (s *TradingServer) GetOrderbook(_ context.Context, _ *tradingv1.GetOrderbookRequest) (*tradingv1.GetOrderbookResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetOrderbook not yet implemented")
}

// CancelAllOrdersByMarket cancels all open orders for a market.
func (s *TradingServer) CancelAllOrdersByMarket(ctx context.Context, req *tradingv1.CancelAllOrdersByMarketRequest) (*tradingv1.CancelAllOrdersByMarketResponse, error) {
	count, err := s.orderService.CancelAllOrdersByMarket(ctx, req.GetMarketId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &tradingv1.CancelAllOrdersByMarketResponse{
		CancelledCount: count,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// domainOrderToProto converts a domain.Order to the protobuf Order message.
func domainOrderToProto(o *domain.Order) *tradingv1.Order {
	if o == nil {
		return nil
	}
	pb := &tradingv1.Order{
		Id:             o.ID,
		UserId:         o.UserID,
		MarketId:       o.MarketID,
		OutcomeId:      o.OutcomeID,
		Side:           domainOrderSideToProto(o.Side),
		Price:          o.Price.String(),
		Quantity:       o.Quantity.String(),
		FilledQuantity: o.FilledQty.String(),
		Status:         domainOrderStatusToProto(o.Status),
		CreatedAt:      timestamppb.New(o.CreatedAt),
		UpdatedAt:      timestamppb.New(o.UpdatedAt),
	}
	return pb
}

// domainTradeToProto converts a domain.Trade to the protobuf Trade message.
func domainTradeToProto(t *domain.Trade) *tradingv1.Trade {
	if t == nil {
		return nil
	}
	return &tradingv1.Trade{
		Id:           t.ID,
		MarketId:     t.MarketID,
		OutcomeId:    t.OutcomeID,
		MakerOrderId: t.MakerOrderID,
		TakerOrderId: t.TakerOrderID,
		MakerUserId:  t.MakerUserID,
		TakerUserId:  t.TakerUserID,
		Price:        t.Price.String(),
		Quantity:     t.Quantity.String(),
		CreatedAt:    timestamppb.New(t.CreatedAt),
	}
}

// domainPositionToProto converts a domain.Position to the protobuf Position message.
func domainPositionToProto(p *domain.Position) *tradingv1.Position {
	if p == nil {
		return nil
	}
	return &tradingv1.Position{
		Id:        p.ID,
		UserId:    p.UserID,
		MarketId:  p.MarketID,
		OutcomeId: p.OutcomeID,
		Quantity:  p.Quantity.String(),
		AvgPrice:  p.AvgPrice.String(),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
}

// protoOrderSideToDomain converts a proto OrderSide to the domain type.
func protoOrderSideToDomain(s tradingv1.OrderSide) domain.OrderSide {
	switch s {
	case tradingv1.OrderSide_ORDER_SIDE_BUY:
		return domain.OrderSideBuy
	case tradingv1.OrderSide_ORDER_SIDE_SELL:
		return domain.OrderSideSell
	default:
		return domain.OrderSideBuy
	}
}

// domainOrderSideToProto converts a domain OrderSide to the proto type.
func domainOrderSideToProto(s domain.OrderSide) tradingv1.OrderSide {
	switch s {
	case domain.OrderSideBuy:
		return tradingv1.OrderSide_ORDER_SIDE_BUY
	case domain.OrderSideSell:
		return tradingv1.OrderSide_ORDER_SIDE_SELL
	default:
		return tradingv1.OrderSide_ORDER_SIDE_UNSPECIFIED
	}
}

// domainOrderStatusToProto converts a domain OrderStatus to the proto type.
func domainOrderStatusToProto(s domain.OrderStatus) tradingv1.OrderStatus {
	switch s {
	case domain.OrderStatusOpen:
		return tradingv1.OrderStatus_ORDER_STATUS_OPEN
	case domain.OrderStatusPartial:
		return tradingv1.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED
	case domain.OrderStatusFilled:
		return tradingv1.OrderStatus_ORDER_STATUS_FILLED
	case domain.OrderStatusCancelled:
		return tradingv1.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return tradingv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

// toGRPCError translates an application error (pkg/errors.AppError) to the
// corresponding gRPC status error.
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
