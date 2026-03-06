package redis

import (
	"context"
	"fmt"
	"strconv"

	goredis "github.com/redis/go-redis/v9"
)

const orderbookKeyPrefix = "orderbook:"

// PriceLevel represents a single price level in an order book with its
// aggregated quantity.
type PriceLevel struct {
	Price    float64
	Quantity float64
}

// OrderBookCache provides a Redis sorted-set-backed cache for order book
// snapshots, enabling fast retrieval of price levels by market and side.
type OrderBookCache struct {
	client *goredis.Client
}

// NewOrderBookCache returns a new OrderBookCache backed by the given Redis
// client.
func NewOrderBookCache(client *goredis.Client) *OrderBookCache {
	return &OrderBookCache{client: client}
}

func orderbookKey(marketID, outcomeID, side string) string {
	return fmt.Sprintf("%s%s:%s:%s", orderbookKeyPrefix, marketID, outcomeID, side)
}

// UpdateLevel sets the quantity for a given price level in the order book. If
// the quantity is zero, the level is removed.
func (c *OrderBookCache) UpdateLevel(ctx context.Context, marketID, outcomeID, side string, price float64, qty float64) error {
	key := orderbookKey(marketID, outcomeID, side)
	member := strconv.FormatFloat(price, 'f', -1, 64)

	if qty <= 0 {
		return c.client.ZRem(ctx, key, member).Err()
	}

	return c.client.ZAdd(ctx, key, goredis.Z{
		Score:  qty,
		Member: member,
	}).Err()
}

// GetSide returns all price levels for one side of the order book, ordered by
// price ascending.
func (c *OrderBookCache) GetSide(ctx context.Context, marketID, outcomeID, side string) ([]PriceLevel, error) {
	key := orderbookKey(marketID, outcomeID, side)

	results, err := c.client.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	levels := make([]PriceLevel, 0, len(results))
	for _, z := range results {
		price, err := strconv.ParseFloat(z.Member.(string), 64)
		if err != nil {
			return nil, fmt.Errorf("parse price level: %w", err)
		}
		levels = append(levels, PriceLevel{
			Price:    price,
			Quantity: z.Score,
		})
	}

	return levels, nil
}

// RemoveLevel removes a specific price level from the order book.
func (c *OrderBookCache) RemoveLevel(ctx context.Context, marketID, outcomeID, side string, price float64) error {
	key := orderbookKey(marketID, outcomeID, side)
	member := strconv.FormatFloat(price, 'f', -1, 64)
	return c.client.ZRem(ctx, key, member).Err()
}
