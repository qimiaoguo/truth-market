package matching

import (
	"container/list"

	"github.com/truthmarket/truth-market/pkg/domain"
)

// OrderQueue is a FIFO queue of orders at a single price level.
// It supports O(1) enqueue/dequeue and O(n) removal by order ID.
type OrderQueue struct {
	orders *list.List
	index  map[string]*list.Element
}

// NewOrderQueue creates an empty order queue.
func NewOrderQueue() *OrderQueue {
	return &OrderQueue{
		orders: list.New(),
		index:  make(map[string]*list.Element),
	}
}

// Enqueue adds an order to the back of the queue.
func (q *OrderQueue) Enqueue(order *domain.Order) {
	elem := q.orders.PushBack(order)
	q.index[order.ID] = elem
}

// Dequeue removes and returns the order at the front of the queue.
// Returns nil if the queue is empty.
func (q *OrderQueue) Dequeue() *domain.Order {
	front := q.orders.Front()
	if front == nil {
		return nil
	}
	q.orders.Remove(front)
	order := front.Value.(*domain.Order)
	delete(q.index, order.ID)
	return order
}

// Peek returns the order at the front of the queue without removing it.
// Returns nil if the queue is empty.
func (q *OrderQueue) Peek() *domain.Order {
	front := q.orders.Front()
	if front == nil {
		return nil
	}
	return front.Value.(*domain.Order)
}

// Remove removes the order with the given ID from anywhere in the queue.
// Returns true if the order was found and removed, false otherwise.
func (q *OrderQueue) Remove(orderID string) bool {
	elem, ok := q.index[orderID]
	if !ok {
		return false
	}
	q.orders.Remove(elem)
	delete(q.index, orderID)
	return true
}

// Len returns the number of orders currently in the queue.
func (q *OrderQueue) Len() int {
	return q.orders.Len()
}

// Orders returns all orders in the queue in FIFO order (front to back).
func (q *OrderQueue) Orders() []*domain.Order {
	result := make([]*domain.Order, 0, q.orders.Len())
	for e := q.orders.Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(*domain.Order))
	}
	return result
}
