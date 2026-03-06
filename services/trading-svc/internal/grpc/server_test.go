package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	tradingv1 "github.com/truthmarket/truth-market/proto/gen/go/trading/v1"
	tradinggrpc "github.com/truthmarket/truth-market/services/trading-svc/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock services
// ---------------------------------------------------------------------------

type mockOrderService struct {
	placeOrderFn              func(ctx context.Context, req tradinggrpc.PlaceOrderRequest) (*domain.Order, []*domain.Trade, error)
	cancelOrderFn             func(ctx context.Context, userID, orderID string) error
	getOrderFn                func(ctx context.Context, orderID string) (*domain.Order, error)
	cancelAllOrdersByMarketFn func(ctx context.Context, marketID string) (int64, error)
}

func (m *mockOrderService) PlaceOrder(ctx context.Context, req tradinggrpc.PlaceOrderRequest) (*domain.Order, []*domain.Trade, error) {
	return m.placeOrderFn(ctx, req)
}

func (m *mockOrderService) CancelOrder(ctx context.Context, userID, orderID string) error {
	return m.cancelOrderFn(ctx, userID, orderID)
}

func (m *mockOrderService) GetOrder(ctx context.Context, orderID string) (*domain.Order, error) {
	return m.getOrderFn(ctx, orderID)
}

func (m *mockOrderService) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	if m.cancelAllOrdersByMarketFn != nil {
		return m.cancelAllOrdersByMarketFn(ctx, marketID)
	}
	return 0, nil
}

type mockMintService struct {
	mintTokensFn func(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error)
}

func (m *mockMintService) MintTokens(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error) {
	return m.mintTokensFn(ctx, userID, marketID, quantity)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

type testEnv struct {
	client   tradingv1.TradingServiceClient
	orderSvc *mockOrderService
	mintSvc  *mockMintService
	conn     *grpc.ClientConn
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	orderSvc := &mockOrderService{}
	mintSvc := &mockMintService{}

	srv := tradinggrpc.NewTradingServer(orderSvc, mintSvc)

	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	tradingv1.RegisterTradingServiceServer(gs, srv)

	go func() {
		if err := gs.Serve(lis); err != nil {
			// expected on GracefulStop
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		gs.GracefulStop()
	})

	return &testEnv{
		client:   tradingv1.NewTradingServiceClient(conn),
		orderSvc: orderSvc,
		mintSvc:  mintSvc,
		conn:     conn,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGRPC_PlaceOrder_Returns201(t *testing.T) {
	env := newTestEnv(t)

	order := &domain.Order{
		ID:        "order-1",
		UserID:    "user-1",
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromFloat(10),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
	}

	env.orderSvc.placeOrderFn = func(_ context.Context, req tradinggrpc.PlaceOrderRequest) (*domain.Order, []*domain.Trade, error) {
		assert.Equal(t, "user-1", req.UserID)
		assert.Equal(t, "market-1", req.MarketID)
		assert.Equal(t, "outcome-1", req.OutcomeID)
		assert.Equal(t, domain.OrderSideBuy, req.Side)
		assert.True(t, req.Price.Equal(decimal.NewFromFloat(0.50)))
		assert.True(t, req.Quantity.Equal(decimal.NewFromFloat(10)))
		return order, nil, nil
	}

	resp, err := env.client.PlaceOrder(context.Background(), &tradingv1.PlaceOrderRequest{
		UserId:    "user-1",
		MarketId:  "market-1",
		OutcomeId: "outcome-1",
		Side:      tradingv1.OrderSide_ORDER_SIDE_BUY,
		Price:     "0.50",
		Quantity:  "10",
	})
	require.NoError(t, err)

	assert.Equal(t, "order-1", resp.GetOrder().GetId())
	assert.Equal(t, "user-1", resp.GetOrder().GetUserId())
	assert.Equal(t, tradingv1.OrderSide_ORDER_SIDE_BUY, resp.GetOrder().GetSide())
	assert.Equal(t, "0.5", resp.GetOrder().GetPrice())
	assert.Equal(t, "10", resp.GetOrder().GetQuantity())
	assert.Equal(t, tradingv1.OrderStatus_ORDER_STATUS_OPEN, resp.GetOrder().GetStatus())
	assert.Empty(t, resp.GetTrades(), "no trades for unmatched order")
}

func TestGRPC_PlaceOrder_InvalidPrice_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.orderSvc.placeOrderFn = func(_ context.Context, _ tradinggrpc.PlaceOrderRequest) (*domain.Order, []*domain.Trade, error) {
		return nil, nil, apperrors.ErrInvalidPrice
	}

	resp, err := env.client.PlaceOrder(context.Background(), &tradingv1.PlaceOrderRequest{
		UserId:    "user-1",
		MarketId:  "market-1",
		OutcomeId: "outcome-1",
		Side:      tradingv1.OrderSide_ORDER_SIDE_BUY,
		Price:     "1.50",
		Quantity:  "10",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPC_MintTokens_ReturnsPositions(t *testing.T) {
	env := newTestEnv(t)

	positions := []*domain.Position{
		{ID: "pos-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-yes", Quantity: decimal.NewFromFloat(10), AvgPrice: decimal.NewFromFloat(1)},
		{ID: "pos-2", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-no", Quantity: decimal.NewFromFloat(10), AvgPrice: decimal.NewFromFloat(1)},
	}

	env.mintSvc.mintTokensFn = func(_ context.Context, userID, marketID string, qty decimal.Decimal) ([]*domain.Position, error) {
		assert.Equal(t, "user-1", userID)
		assert.Equal(t, "market-1", marketID)
		assert.True(t, qty.Equal(decimal.NewFromFloat(10)))
		return positions, nil
	}

	resp, err := env.client.MintTokens(context.Background(), &tradingv1.MintTokensRequest{
		UserId:   "user-1",
		MarketId: "market-1",
		Quantity: "10",
	})
	require.NoError(t, err)

	require.Len(t, resp.GetPositions(), 2)
	assert.Equal(t, "pos-1", resp.GetPositions()[0].GetId())
	assert.Equal(t, "o-yes", resp.GetPositions()[0].GetOutcomeId())
	assert.Equal(t, "10", resp.GetPositions()[0].GetQuantity())
	assert.Equal(t, "pos-2", resp.GetPositions()[1].GetId())
	assert.Equal(t, "o-no", resp.GetPositions()[1].GetOutcomeId())
}

func TestGRPC_MintTokens_InsufficientBalance_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.mintSvc.mintTokensFn = func(_ context.Context, _, _ string, _ decimal.Decimal) ([]*domain.Position, error) {
		return nil, apperrors.ErrInsufficientBalance
	}

	resp, err := env.client.MintTokens(context.Background(), &tradingv1.MintTokensRequest{
		UserId:   "user-1",
		MarketId: "market-1",
		Quantity: "1000",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}
