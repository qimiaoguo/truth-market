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
	listOrdersFn              func(ctx context.Context, filter tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error)
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

func (m *mockOrderService) ListOrders(ctx context.Context, filter tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error) {
	if m.listOrdersFn != nil {
		return m.listOrdersFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockOrderService) CancelAllOrdersByMarket(ctx context.Context, marketID string) (int64, error) {
	if m.cancelAllOrdersByMarketFn != nil {
		return m.cancelAllOrdersByMarketFn(ctx, marketID)
	}
	return 0, nil
}

type mockMintService struct {
	mintTokensFn   func(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error)
	getPositionsFn func(ctx context.Context, userID, marketID string) ([]*domain.Position, error)
}

func (m *mockMintService) MintTokens(ctx context.Context, userID, marketID string, quantity decimal.Decimal) ([]*domain.Position, error) {
	return m.mintTokensFn(ctx, userID, marketID, quantity)
}

func (m *mockMintService) GetPositions(ctx context.Context, userID, marketID string) ([]*domain.Position, error) {
	if m.getPositionsFn != nil {
		return m.getPositionsFn(ctx, userID, marketID)
	}
	return nil, nil
}

// mockOrderbookProvider implements tradinggrpc.OrderbookProvider for tests.
type mockOrderbookProvider struct {
	getOrderbookDepthFn func(outcomeID string, levels int) (bids, asks []tradinggrpc.OrderbookLevel)
}

func (m *mockOrderbookProvider) GetOrderbookDepth(outcomeID string, levels int) (bids, asks []tradinggrpc.OrderbookLevel) {
	if m.getOrderbookDepthFn != nil {
		return m.getOrderbookDepthFn(outcomeID, levels)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

type testEnv struct {
	client       tradingv1.TradingServiceClient
	orderSvc     *mockOrderService
	mintSvc      *mockMintService
	orderbookPrv *mockOrderbookProvider
	conn         *grpc.ClientConn
}

func newTestEnv(t *testing.T, opts ...tradinggrpc.Option) *testEnv {
	t.Helper()

	orderSvc := &mockOrderService{}
	mintSvc := &mockMintService{}
	obProvider := &mockOrderbookProvider{}

	allOpts := append([]tradinggrpc.Option{tradinggrpc.WithOrderbookProvider(obProvider)}, opts...)
	srv := tradinggrpc.NewTradingServer(orderSvc, mintSvc, allOpts...)

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
		client:       tradingv1.NewTradingServiceClient(conn),
		orderSvc:     orderSvc,
		mintSvc:      mintSvc,
		orderbookPrv: obProvider,
		conn:         conn,
	}
}

// ---------------------------------------------------------------------------
// Tests: PlaceOrder
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

// ---------------------------------------------------------------------------
// Tests: MintTokens
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Tests: ListOrders
// ---------------------------------------------------------------------------

func TestGRPC_ListOrders_ReturnsOrders(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	orders := []*domain.Order{
		{
			ID: "order-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-yes",
			Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.50),
			Quantity: decimal.NewFromFloat(10), FilledQty: decimal.Zero,
			Status: domain.OrderStatusOpen, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "order-2", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-no",
			Side: domain.OrderSideSell, Price: decimal.NewFromFloat(0.60),
			Quantity: decimal.NewFromFloat(5), FilledQty: decimal.NewFromFloat(5),
			Status: domain.OrderStatusFilled, CreatedAt: now, UpdatedAt: now,
		},
	}

	env.orderSvc.listOrdersFn = func(_ context.Context, filter tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error) {
		assert.Equal(t, "user-1", filter.UserID)
		assert.Equal(t, "market-1", filter.MarketID)
		return orders, int64(len(orders)), nil
	}

	resp, err := env.client.ListOrders(context.Background(), &tradingv1.ListOrdersRequest{
		UserId:   "user-1",
		MarketId: "market-1",
	})
	require.NoError(t, err)

	require.Len(t, resp.GetOrders(), 2)
	assert.Equal(t, int64(2), resp.GetTotal())

	assert.Equal(t, "order-1", resp.GetOrders()[0].GetId())
	assert.Equal(t, tradingv1.OrderSide_ORDER_SIDE_BUY, resp.GetOrders()[0].GetSide())
	assert.Equal(t, tradingv1.OrderStatus_ORDER_STATUS_OPEN, resp.GetOrders()[0].GetStatus())

	assert.Equal(t, "order-2", resp.GetOrders()[1].GetId())
	assert.Equal(t, tradingv1.OrderSide_ORDER_SIDE_SELL, resp.GetOrders()[1].GetSide())
	assert.Equal(t, tradingv1.OrderStatus_ORDER_STATUS_FILLED, resp.GetOrders()[1].GetStatus())
}

func TestGRPC_ListOrders_FilterByStatus(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	openOrder := &domain.Order{
		ID: "order-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-yes",
		Side: domain.OrderSideBuy, Price: decimal.NewFromFloat(0.50),
		Quantity: decimal.NewFromFloat(10), FilledQty: decimal.Zero,
		Status: domain.OrderStatusOpen, CreatedAt: now, UpdatedAt: now,
	}

	env.orderSvc.listOrdersFn = func(_ context.Context, filter tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error) {
		assert.Equal(t, "user-1", filter.UserID)
		assert.Equal(t, domain.OrderStatusOpen, filter.Status)
		return []*domain.Order{openOrder}, 1, nil
	}

	resp, err := env.client.ListOrders(context.Background(), &tradingv1.ListOrdersRequest{
		UserId: "user-1",
		Status: tradingv1.OrderStatus_ORDER_STATUS_OPEN,
	})
	require.NoError(t, err)

	require.Len(t, resp.GetOrders(), 1)
	assert.Equal(t, int64(1), resp.GetTotal())
	assert.Equal(t, "order-1", resp.GetOrders()[0].GetId())
}

func TestGRPC_ListOrders_EmptyResult(t *testing.T) {
	env := newTestEnv(t)

	env.orderSvc.listOrdersFn = func(_ context.Context, _ tradinggrpc.ListOrdersFilter) ([]*domain.Order, int64, error) {
		return nil, 0, nil
	}

	resp, err := env.client.ListOrders(context.Background(), &tradingv1.ListOrdersRequest{
		UserId: "user-1",
	})
	require.NoError(t, err)

	assert.Empty(t, resp.GetOrders())
	assert.Equal(t, int64(0), resp.GetTotal())
}

func TestGRPC_ListOrders_MissingUserId_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.ListOrders(context.Background(), &tradingv1.ListOrdersRequest{})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---------------------------------------------------------------------------
// Tests: GetPositions
// ---------------------------------------------------------------------------

func TestGRPC_GetPositions_ReturnsPositions(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	positions := []*domain.Position{
		{ID: "pos-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-yes",
			Quantity: decimal.NewFromFloat(50), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
		{ID: "pos-2", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-no",
			Quantity: decimal.NewFromFloat(100), AvgPrice: decimal.NewFromFloat(1), UpdatedAt: now},
	}

	env.mintSvc.getPositionsFn = func(_ context.Context, userID, marketID string) ([]*domain.Position, error) {
		assert.Equal(t, "user-1", userID)
		assert.Equal(t, "market-1", marketID)
		return positions, nil
	}

	resp, err := env.client.GetPositions(context.Background(), &tradingv1.GetPositionsRequest{
		UserId:   "user-1",
		MarketId: "market-1",
	})
	require.NoError(t, err)

	require.Len(t, resp.GetPositions(), 2)
	assert.Equal(t, "pos-1", resp.GetPositions()[0].GetId())
	assert.Equal(t, "o-yes", resp.GetPositions()[0].GetOutcomeId())
	assert.Equal(t, "50", resp.GetPositions()[0].GetQuantity())
	assert.Equal(t, "0.6", resp.GetPositions()[0].GetAvgPrice())

	assert.Equal(t, "pos-2", resp.GetPositions()[1].GetId())
	assert.Equal(t, "o-no", resp.GetPositions()[1].GetOutcomeId())
	assert.Equal(t, "100", resp.GetPositions()[1].GetQuantity())
}

func TestGRPC_GetPositions_AllMarkets(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	positions := []*domain.Position{
		{ID: "pos-1", UserID: "user-1", MarketID: "market-1", OutcomeID: "o-yes",
			Quantity: decimal.NewFromFloat(50), AvgPrice: decimal.NewFromFloat(0.60), UpdatedAt: now},
		{ID: "pos-3", UserID: "user-1", MarketID: "market-2", OutcomeID: "o-red",
			Quantity: decimal.NewFromFloat(20), AvgPrice: decimal.NewFromFloat(0.30), UpdatedAt: now},
	}

	env.mintSvc.getPositionsFn = func(_ context.Context, userID, marketID string) ([]*domain.Position, error) {
		assert.Equal(t, "user-1", userID)
		assert.Equal(t, "", marketID, "market_id should be empty when not specified")
		return positions, nil
	}

	resp, err := env.client.GetPositions(context.Background(), &tradingv1.GetPositionsRequest{
		UserId: "user-1",
	})
	require.NoError(t, err)

	require.Len(t, resp.GetPositions(), 2)
}

func TestGRPC_GetPositions_EmptyResult(t *testing.T) {
	env := newTestEnv(t)

	env.mintSvc.getPositionsFn = func(_ context.Context, _, _ string) ([]*domain.Position, error) {
		return nil, nil
	}

	resp, err := env.client.GetPositions(context.Background(), &tradingv1.GetPositionsRequest{
		UserId: "user-1",
	})
	require.NoError(t, err)

	assert.Empty(t, resp.GetPositions())
}

func TestGRPC_GetPositions_MissingUserId_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.GetPositions(context.Background(), &tradingv1.GetPositionsRequest{})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// ---------------------------------------------------------------------------
// Tests: GetOrderbook
// ---------------------------------------------------------------------------

func TestGRPC_GetOrderbook_ReturnsBidsAndAsks(t *testing.T) {
	env := newTestEnv(t)

	env.orderbookPrv.getOrderbookDepthFn = func(outcomeID string, levels int) (bids, asks []tradinggrpc.OrderbookLevel) {
		assert.Equal(t, "o-yes", outcomeID)
		assert.Equal(t, 20, levels, "default depth should be 20")
		bids = []tradinggrpc.OrderbookLevel{
			{Price: decimal.NewFromFloat(0.50), Quantity: decimal.NewFromFloat(100), Count: 3},
			{Price: decimal.NewFromFloat(0.45), Quantity: decimal.NewFromFloat(50), Count: 1},
		}
		asks = []tradinggrpc.OrderbookLevel{
			{Price: decimal.NewFromFloat(0.55), Quantity: decimal.NewFromFloat(80), Count: 2},
			{Price: decimal.NewFromFloat(0.60), Quantity: decimal.NewFromFloat(30), Count: 1},
		}
		return bids, asks
	}

	resp, err := env.client.GetOrderbook(context.Background(), &tradingv1.GetOrderbookRequest{
		MarketId:  "market-1",
		OutcomeId: "o-yes",
	})
	require.NoError(t, err)

	require.Len(t, resp.GetBids(), 2)
	assert.Equal(t, "0.5", resp.GetBids()[0].GetPrice())
	assert.Equal(t, "100", resp.GetBids()[0].GetQuantity())
	assert.Equal(t, int32(3), resp.GetBids()[0].GetOrderCount())

	assert.Equal(t, "0.45", resp.GetBids()[1].GetPrice())
	assert.Equal(t, "50", resp.GetBids()[1].GetQuantity())
	assert.Equal(t, int32(1), resp.GetBids()[1].GetOrderCount())

	require.Len(t, resp.GetAsks(), 2)
	assert.Equal(t, "0.55", resp.GetAsks()[0].GetPrice())
	assert.Equal(t, "80", resp.GetAsks()[0].GetQuantity())
	assert.Equal(t, int32(2), resp.GetAsks()[0].GetOrderCount())

	assert.Equal(t, "0.6", resp.GetAsks()[1].GetPrice())
	assert.Equal(t, "30", resp.GetAsks()[1].GetQuantity())
	assert.Equal(t, int32(1), resp.GetAsks()[1].GetOrderCount())
}

func TestGRPC_GetOrderbook_EmptyBook(t *testing.T) {
	env := newTestEnv(t)

	env.orderbookPrv.getOrderbookDepthFn = func(_ string, _ int) (bids, asks []tradinggrpc.OrderbookLevel) {
		return nil, nil
	}

	resp, err := env.client.GetOrderbook(context.Background(), &tradingv1.GetOrderbookRequest{
		MarketId:  "market-1",
		OutcomeId: "o-yes",
	})
	require.NoError(t, err)

	assert.Empty(t, resp.GetBids())
	assert.Empty(t, resp.GetAsks())
}

func TestGRPC_GetOrderbook_MissingOutcomeId_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	resp, err := env.client.GetOrderbook(context.Background(), &tradingv1.GetOrderbookRequest{
		MarketId: "market-1",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
