package main

import (
	"context"

	"github.com/truthmarket/truth-market/pkg/domain"
	tradinggrpc "github.com/truthmarket/truth-market/services/trading-svc/internal/grpc"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/service"
)

// orderServiceAdapter bridges service.OrderService to grpc.OrderServicer by
// converting between the two packages' request/filter types, which are
// structurally identical but declared in separate packages.
type orderServiceAdapter struct {
	svc *service.OrderService
}

var _ tradinggrpc.OrderServicer = (*orderServiceAdapter)(nil)

func (a *orderServiceAdapter) PlaceOrder(ctx context.Context, req tradinggrpc.PlaceOrderRequest) (*domain.Order, []*domain.Trade, error) {
	return a.svc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    req.UserID,
		MarketID:  req.MarketID,
		OutcomeID: req.OutcomeID,
		Side:      req.Side,
		Price:     req.Price,
		Quantity:  req.Quantity,
	})
}

func (a *orderServiceAdapter) CancelOrder(ctx context.Context, userID, orderID string) error {
	return a.svc.CancelOrder(ctx, userID, orderID)
}

func (a *orderServiceAdapter) GetOrder(ctx context.Context, orderID string) (*domain.Order, error) {
	return a.svc.GetOrder(ctx, orderID)
}

func (a *orderServiceAdapter) ListOrders(ctx context.Context, filter tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error) {
	return a.svc.ListOrders(ctx, service.ListOrdersFilter{
		UserID:   filter.UserID,
		MarketID: filter.MarketID,
		Status:   filter.Status,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	})
}

func (a *orderServiceAdapter) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	return a.svc.CancelAllOrdersByMarket(ctx, marketID)
}
