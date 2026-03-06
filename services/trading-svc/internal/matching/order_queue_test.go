package matching

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestOrder creates a minimal Order with the given ID at a fixed price level.
// Useful for queue tests where price is uniform and FIFO ordering matters.
func newTestOrder(id string, side domain.OrderSide, price float64, qty float64) *domain.Order {
	return &domain.Order{
		ID:        id,
		UserID:    "user-" + id,
		MarketID:  "market-1",
		OutcomeID: "outcome-1",
		Side:      side,
		Price:     decimal.NewFromFloat(price),
		Quantity:  decimal.NewFromFloat(qty),
		FilledQty: decimal.Zero,
		Status:    domain.OrderStatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Tests: OrderQueue FIFO behaviour
// ---------------------------------------------------------------------------

func TestOrderQueue_EnqueueDequeue_FIFO(t *testing.T) {
	q := NewOrderQueue()

	o1 := newTestOrder("order-1", domain.OrderSideBuy, 0.50, 10)
	o2 := newTestOrder("order-2", domain.OrderSideBuy, 0.50, 5)
	o3 := newTestOrder("order-3", domain.OrderSideBuy, 0.50, 8)

	q.Enqueue(o1)
	q.Enqueue(o2)
	q.Enqueue(o3)

	// Dequeue should return orders in the exact insertion order (FIFO).
	got1 := q.Dequeue()
	require.NotNil(t, got1, "first dequeue should return an order")
	assert.Equal(t, "order-1", got1.ID, "first dequeued order should be order-1")

	got2 := q.Dequeue()
	require.NotNil(t, got2, "second dequeue should return an order")
	assert.Equal(t, "order-2", got2.ID, "second dequeued order should be order-2")

	got3 := q.Dequeue()
	require.NotNil(t, got3, "third dequeue should return an order")
	assert.Equal(t, "order-3", got3.ID, "third dequeued order should be order-3")

	// Queue should now be empty.
	assert.Nil(t, q.Dequeue(), "dequeue on empty queue should return nil")
}

func TestOrderQueue_Peek_DoesNotRemove(t *testing.T) {
	q := NewOrderQueue()

	o1 := newTestOrder("order-1", domain.OrderSideBuy, 0.50, 10)
	o2 := newTestOrder("order-2", domain.OrderSideBuy, 0.50, 5)

	q.Enqueue(o1)
	q.Enqueue(o2)

	// Peek should return the head of the queue without removing it.
	peeked := q.Peek()
	require.NotNil(t, peeked, "peek should return an order when queue is non-empty")
	assert.Equal(t, "order-1", peeked.ID, "peek should return the first enqueued order")

	// Peek a second time -- same order should still be at the head.
	peeked2 := q.Peek()
	require.NotNil(t, peeked2, "second peek should still return an order")
	assert.Equal(t, "order-1", peeked2.ID, "peek should not remove the head order")

	// Length should be unchanged after peeks.
	assert.Equal(t, 2, q.Len(), "peek should not change the queue length")
}

func TestOrderQueue_Len_TracksSize(t *testing.T) {
	q := NewOrderQueue()

	assert.Equal(t, 0, q.Len(), "newly created queue should have length 0")

	q.Enqueue(newTestOrder("order-1", domain.OrderSideBuy, 0.50, 10))
	assert.Equal(t, 1, q.Len(), "after one enqueue, length should be 1")

	q.Enqueue(newTestOrder("order-2", domain.OrderSideBuy, 0.50, 5))
	assert.Equal(t, 2, q.Len(), "after two enqueues, length should be 2")

	q.Enqueue(newTestOrder("order-3", domain.OrderSideBuy, 0.50, 8))
	assert.Equal(t, 3, q.Len(), "after three enqueues, length should be 3")

	q.Dequeue()
	assert.Equal(t, 2, q.Len(), "after one dequeue, length should be 2")

	q.Dequeue()
	assert.Equal(t, 1, q.Len(), "after two dequeues, length should be 1")

	q.Dequeue()
	assert.Equal(t, 0, q.Len(), "after three dequeues, length should be 0")

	// Dequeue on empty should not go negative.
	q.Dequeue()
	assert.Equal(t, 0, q.Len(), "dequeue on empty queue should keep length at 0")
}

func TestOrderQueue_Remove_ByID(t *testing.T) {
	q := NewOrderQueue()

	o1 := newTestOrder("order-1", domain.OrderSideBuy, 0.50, 10)
	o2 := newTestOrder("order-2", domain.OrderSideBuy, 0.50, 5)
	o3 := newTestOrder("order-3", domain.OrderSideBuy, 0.50, 8)

	q.Enqueue(o1)
	q.Enqueue(o2)
	q.Enqueue(o3)

	// Remove the middle order.
	removed := q.Remove("order-2")
	assert.True(t, removed, "remove should return true when order is found")
	assert.Equal(t, 2, q.Len(), "length should decrease after removal")

	// FIFO order should be maintained: order-1, then order-3.
	got1 := q.Dequeue()
	require.NotNil(t, got1)
	assert.Equal(t, "order-1", got1.ID, "first remaining order should be order-1")

	got2 := q.Dequeue()
	require.NotNil(t, got2)
	assert.Equal(t, "order-3", got2.ID, "second remaining order should be order-3")

	// Removing a non-existent order should return false.
	removed = q.Remove("order-nonexistent")
	assert.False(t, removed, "remove should return false when order ID does not exist")

	// Removing an already-removed order should return false.
	removed = q.Remove("order-2")
	assert.False(t, removed, "remove should return false for an already-removed order")
}

func TestOrderQueue_DequeueEmpty_ReturnsNil(t *testing.T) {
	q := NewOrderQueue()

	// Dequeue on a freshly created empty queue.
	result := q.Dequeue()
	assert.Nil(t, result, "dequeue on empty queue should return nil")

	// Peek on empty queue should also return nil.
	peeked := q.Peek()
	assert.Nil(t, peeked, "peek on empty queue should return nil")

	// Enqueue and drain, then dequeue again.
	q.Enqueue(newTestOrder("order-1", domain.OrderSideBuy, 0.50, 10))
	q.Dequeue()

	result = q.Dequeue()
	assert.Nil(t, result, "dequeue on drained queue should return nil")
}
