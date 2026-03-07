package service_test

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/truthmarket/truth-market/infra/postgres"
	"github.com/truthmarket/truth-market/infra/testutil"
	"github.com/truthmarket/truth-market/pkg/domain"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/matching"
	"github.com/truthmarket/truth-market/services/trading-svc/internal/service"
)

// Shared test infrastructure — initialised once by TestMain.
var (
	testPool     *pgxpool.Pool
	testUserRepo *postgres.UserRepo
	testOrderRepo *postgres.OrderRepo
	testTradeRepo *postgres.TradeRepo
	testPositionRepo *postgres.PositionRepo
	testMarketRepo *postgres.MarketRepo
	testOutcomeRepo *postgres.OutcomeRepo
	testTxManager *postgres.PgTxManager
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	ctx := context.Background()

	// Start postgres container.
	dsn, cleanup, err := testutil.PostgresContainer(ctx)
	if err != nil {
		panic("failed to start postgres container: " + err.Error())
	}
	defer cleanup()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic("failed to create pool: " + err.Error())
	}
	defer pool.Close()

	// Run all migrations.
	migrationsDir := filepath.Join("..", "..", "..", "..", "migrations")
	if err := runMigrations(ctx, pool, migrationsDir); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	testPool = pool
	testUserRepo = postgres.NewUserRepo(pool)
	testOrderRepo = postgres.NewOrderRepo(pool)
	testTradeRepo = postgres.NewTradeRepo(pool)
	testPositionRepo = postgres.NewPositionRepo(pool)
	testMarketRepo = postgres.NewMarketRepo(pool)
	testOutcomeRepo = postgres.NewOutcomeRepo(pool)
	testTxManager = postgres.NewTxManager(pool)

	os.Exit(m.Run())
}

// runMigrations reads and executes all *.up.sql files in order.
func runMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(absDir, "*.up.sql"))
	if err != nil {
		return err
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return err
		}
	}
	return nil
}

// truncateAll removes all rows so each test starts clean.
func truncateAll(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	// Order matters due to foreign keys.
	tables := []string{"trades", "positions", "orders", "outcomes", "markets", "users"}
	for _, tbl := range tables {
		_, err := testPool.Exec(ctx, "TRUNCATE TABLE "+tbl+" CASCADE")
		require.NoError(t, err)
	}
}

// setupMarket creates a user (admin), a market, and outcomes. Returns IDs.
func setupMarket(t *testing.T, ctx context.Context) (adminID, marketID, outcomeAID, outcomeBID string) {
	t.Helper()

	admin := testutil.NewUser(testutil.WithAdmin())
	require.NoError(t, testUserRepo.Create(ctx, admin))

	market, outcomes := testutil.NewBinaryMarket(admin.ID)
	market.Status = domain.MarketStatusOpen
	require.NoError(t, testMarketRepo.Create(ctx, market))
	require.NoError(t, testOutcomeRepo.CreateBatch(ctx, outcomes))

	return admin.ID, market.ID, outcomes[0].ID, outcomes[1].ID
}

// TestIntegration_FullTradingFlow verifies the complete flow:
// 1. Place sell order (requires position) → funds/position locked
// 2. Place crossing buy order → match executes
// 3. Trade persisted, balances settled, positions updated, orders filled
func TestIntegration_FullTradingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateAll(t)

	ctx := context.Background()
	_, marketID, outcomeID, _ := setupMarket(t, ctx)

	// Create seller with position (simulating prior mint).
	seller := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, seller))
	sellerPos := &domain.Position{
		ID:        uuid.New().String(),
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Quantity:  decimal.NewFromInt(10),
		AvgPrice:  decimal.NewFromFloat(0.50),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, testPositionRepo.Upsert(ctx, sellerPos))

	// Create buyer with balance.
	buyer := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, buyer))

	// Create matching engine and order service.
	engine := matching.NewRegistry()
	orderSvc := service.NewOrderService(
		testOrderRepo, testUserRepo, testPositionRepo, testTradeRepo, testTxManager, engine,
	)

	// ── Seller places sell @0.70, qty=10 ──
	sellOrder, sellTrades, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideSell,
		Price:     decimal.NewFromFloat(0.70),
		Quantity:  decimal.NewFromInt(10),
	})
	require.NoError(t, err)
	assert.Empty(t, sellTrades, "no trades yet — sell is resting")
	assert.Equal(t, domain.OrderStatusOpen, sellOrder.Status)

	// Verify seller's position was reduced.
	sellerPosAfterSell, err := testPositionRepo.GetByUserAndOutcome(ctx, seller.ID, outcomeID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.Zero, sellerPosAfterSell.Quantity, "seller position reduced to 0")

	// ── Buyer places buy @0.80, qty=10 → should match with sell @0.70 ──
	buyOrder, buyTrades, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    buyer.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.80),
		Quantity:  decimal.NewFromInt(10),
	})
	require.NoError(t, err)
	require.Len(t, buyTrades, 1, "should produce exactly 1 trade")

	trade := buyTrades[0]

	// ── Verify trade ──
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(0.70), trade.Price, "trade executes at maker (sell) price")
	testutil.AssertDecimalEqual(t, decimal.NewFromInt(10), trade.Quantity, "full quantity traded")

	// ── Verify buyer order status ──
	buyOrderDB, err := testOrderRepo.GetByID(ctx, buyOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusFilled, buyOrderDB.Status)
	testutil.AssertDecimalEqual(t, decimal.NewFromInt(10), buyOrderDB.FilledQty, "buyer fully filled")

	// ── Verify seller order status ──
	sellOrderDB, err := testOrderRepo.GetByID(ctx, sellOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusFilled, sellOrderDB.Status)
	testutil.AssertDecimalEqual(t, decimal.NewFromInt(10), sellOrderDB.FilledQty, "seller fully filled")

	// ── Verify buyer balance ──
	// Initial: 1000. Locked: 0.80 * 10 = 8.0. Refund: (0.80-0.70)*10 = 1.0.
	// Final balance = 1000 - 8.0 + 1.0 = 993.0. Locked = 0.
	buyerAfter, err := testUserRepo.GetByID(ctx, buyer.ID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(993.0), buyerAfter.Balance, "buyer balance after trade")
	testutil.AssertDecimalEqual(t, decimal.Zero, buyerAfter.LockedBalance, "buyer locked released")

	// ── Verify seller balance ──
	// Initial: 1000. Proceeds: 0.70 * 10 = 7.0.
	// Final balance = 1000 + 7.0 = 1007.0.
	sellerAfter, err := testUserRepo.GetByID(ctx, seller.ID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(1007.0), sellerAfter.Balance, "seller receives proceeds")

	// ── Verify buyer has position ──
	buyerPos, err := testPositionRepo.GetByUserAndOutcome(ctx, buyer.ID, outcomeID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.NewFromInt(10), buyerPos.Quantity, "buyer position created")
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(0.70), buyerPos.AvgPrice, "buyer avg price = trade price")
}

// TestIntegration_PartialFill verifies partial matching:
// Sell 10 vs Buy 5 → 5 filled, 5 remain on sell side.
func TestIntegration_PartialFill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateAll(t)

	ctx := context.Background()
	_, marketID, outcomeID, _ := setupMarket(t, ctx)

	seller := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, seller))
	sellerPos := &domain.Position{
		ID:        uuid.New().String(),
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Quantity:  decimal.NewFromInt(10),
		AvgPrice:  decimal.NewFromFloat(0.50),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, testPositionRepo.Upsert(ctx, sellerPos))

	buyer := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, buyer))

	engine := matching.NewRegistry()
	orderSvc := service.NewOrderService(
		testOrderRepo, testUserRepo, testPositionRepo, testTradeRepo, testTxManager, engine,
	)

	// Sell 10 @0.60
	sellOrder, _, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideSell,
		Price:     decimal.NewFromFloat(0.60),
		Quantity:  decimal.NewFromInt(10),
	})
	require.NoError(t, err)

	// Buy 5 @0.70
	buyOrder, trades, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    buyer.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.70),
		Quantity:  decimal.NewFromInt(5),
	})
	require.NoError(t, err)
	require.Len(t, trades, 1)

	testutil.AssertDecimalEqual(t, decimal.NewFromInt(5), trades[0].Quantity, "partial fill qty=5")

	// Buy order fully filled.
	buyDB, err := testOrderRepo.GetByID(ctx, buyOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusFilled, buyDB.Status)

	// Sell order partially filled.
	sellDB, err := testOrderRepo.GetByID(ctx, sellOrder.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusPartial, sellDB.Status)
	testutil.AssertDecimalEqual(t, decimal.NewFromInt(5), sellDB.FilledQty, "sell partially filled")
}

// TestIntegration_NoMatch verifies no crossing:
// Buy @0.30 vs Sell @0.50 → no trade, both orders rest.
func TestIntegration_NoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	truncateAll(t)

	ctx := context.Background()
	_, marketID, outcomeID, _ := setupMarket(t, ctx)

	seller := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, seller))
	sellerPos := &domain.Position{
		ID:        uuid.New().String(),
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Quantity:  decimal.NewFromInt(10),
		AvgPrice:  decimal.NewFromFloat(0.50),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, testPositionRepo.Upsert(ctx, sellerPos))

	buyer := testutil.NewUser(testutil.WithBalance(1000))
	require.NoError(t, testUserRepo.Create(ctx, buyer))

	engine := matching.NewRegistry()
	orderSvc := service.NewOrderService(
		testOrderRepo, testUserRepo, testPositionRepo, testTradeRepo, testTxManager, engine,
	)

	// Sell @0.50
	_, sellTrades, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    seller.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideSell,
		Price:     decimal.NewFromFloat(0.50),
		Quantity:  decimal.NewFromInt(10),
	})
	require.NoError(t, err)
	assert.Empty(t, sellTrades)

	// Buy @0.30 — does NOT cross sell @0.50.
	_, buyTrades, err := orderSvc.PlaceOrder(ctx, service.PlaceOrderRequest{
		UserID:    buyer.ID,
		MarketID:  marketID,
		OutcomeID: outcomeID,
		Side:      domain.OrderSideBuy,
		Price:     decimal.NewFromFloat(0.30),
		Quantity:  decimal.NewFromInt(5),
	})
	require.NoError(t, err)
	assert.Empty(t, buyTrades, "no match — prices don't cross")

	// Verify buyer balance: 1000 - (0.30 * 5) = 998.50, locked = 1.50.
	buyerAfter, err := testUserRepo.GetByID(ctx, buyer.ID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(998.50), buyerAfter.Balance, "buyer balance after lock")
	testutil.AssertDecimalEqual(t, decimal.NewFromFloat(1.50), buyerAfter.LockedBalance, "buyer funds locked")

	// Verify seller position reduced.
	sellerPosAfter, err := testPositionRepo.GetByUserAndOutcome(ctx, seller.ID, outcomeID)
	require.NoError(t, err)
	testutil.AssertDecimalEqual(t, decimal.Zero, sellerPosAfter.Quantity, "seller position reduced for sell order")
}
