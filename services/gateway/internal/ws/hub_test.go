package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: creates a Hub, starts it, and returns a cleanup function.
func setupHub(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		hub.Stop()
	})
	return hub
}

// Helper: creates a Client with a buffered Send channel wired to the given hub.
func newTestClient(id string, userID string) *Client {
	return &Client{
		ID:     id,
		UserID: userID,
		Send:   make(chan []byte, 256),
	}
}

// receiveWithTimeout reads a single message from the client's Send channel,
// failing the test if nothing arrives within the timeout.
func receiveWithTimeout(t *testing.T, c *Client, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-c.Send:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message on client Send channel")
		return nil
	}
}

// assertNoMessage verifies that nothing is received on the client's Send
// channel within the given duration.
func assertNoMessage(t *testing.T, c *Client, wait time.Duration) {
	t.Helper()
	select {
	case msg := <-c.Send:
		t.Fatalf("expected no message but received: %s", string(msg))
	case <-time.After(wait):
		// Good -- nothing received.
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHub_RegisterClient(t *testing.T) {
	hub := setupHub(t)
	client := newTestClient("client-1", "")

	hub.Register(client)

	// Allow the hub goroutine to process the registration.
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond, "expected 1 registered client")
}

func TestHub_UnregisterClient_CleansUp(t *testing.T) {
	hub := setupHub(t)
	client := newTestClient("client-1", "")

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond)

	hub.Unregister(client)

	// After unregister, the client count should drop to zero.
	require.Eventually(t, func() bool {
		return hub.Clients() == 0
	}, time.Second, 10*time.Millisecond, "expected 0 registered clients after unregister")

	// The Send channel should be closed so that any pending reader unblocks.
	_, open := <-client.Send
	assert.False(t, open, "expected client Send channel to be closed after unregister")
}

func TestHub_Subscribe_MarketChannel(t *testing.T) {
	hub := setupHub(t)
	client := newTestClient("client-1", "")

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond)

	hub.SubscribeMarket(client, "market-1")

	// Allow subscription to be processed.
	time.Sleep(50 * time.Millisecond)

	payload := []byte(`{"type":"price_update","market_id":"market-1","price":"0.65"}`)
	hub.BroadcastMarket("market-1", payload)

	msg := receiveWithTimeout(t, client, time.Second)
	assert.Equal(t, payload, msg)
}

func TestHub_Subscribe_UserChannel_RequiresAuth(t *testing.T) {
	hub := setupHub(t)

	// An unauthenticated client has an empty UserID.
	unauthClient := newTestClient("client-unauth", "")

	hub.Register(unauthClient)
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond)

	// Attempting to subscribe an unauthenticated client to a user channel
	// should fail or be silently ignored -- the client must NOT receive
	// messages broadcast on that user channel.
	hub.SubscribeUser(unauthClient, "user-42")

	// Allow time for the (rejected) subscription attempt to be processed.
	time.Sleep(50 * time.Millisecond)

	payload := []byte(`{"type":"order_filled","user_id":"user-42","amount":"100"}`)
	hub.BroadcastUser("user-42", payload)

	assertNoMessage(t, unauthClient, 200*time.Millisecond)
}

func TestHub_BroadcastMarket_OnlyToSubscribedClients(t *testing.T) {
	hub := setupHub(t)

	subscribedClient := newTestClient("client-sub", "")
	bystander := newTestClient("client-bystander", "")

	hub.Register(subscribedClient)
	hub.Register(bystander)
	require.Eventually(t, func() bool {
		return hub.Clients() == 2
	}, time.Second, 10*time.Millisecond)

	hub.SubscribeMarket(subscribedClient, "market-1")
	// bystander is NOT subscribed to any market.

	time.Sleep(50 * time.Millisecond)

	payload := []byte(`{"type":"price_update","market_id":"market-1","price":"0.72"}`)
	hub.BroadcastMarket("market-1", payload)

	// The subscribed client should receive the message.
	msg := receiveWithTimeout(t, subscribedClient, time.Second)
	assert.Equal(t, payload, msg)

	// The bystander should NOT receive anything.
	assertNoMessage(t, bystander, 200*time.Millisecond)
}

func TestHub_BroadcastMarket_NotSentToOtherMarkets(t *testing.T) {
	hub := setupHub(t)

	client := newTestClient("client-1", "")

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond)

	// Client subscribes to market-1 only.
	hub.SubscribeMarket(client, "market-1")
	time.Sleep(50 * time.Millisecond)

	// Broadcast to market-2 -- client should NOT get it.
	payload := []byte(`{"type":"price_update","market_id":"market-2","price":"0.30"}`)
	hub.BroadcastMarket("market-2", payload)

	assertNoMessage(t, client, 200*time.Millisecond)
}

func TestHub_UnsubscribeMarket(t *testing.T) {
	hub := setupHub(t)

	client := newTestClient("client-1", "")

	hub.Register(client)
	require.Eventually(t, func() bool {
		return hub.Clients() == 1
	}, time.Second, 10*time.Millisecond)

	hub.SubscribeMarket(client, "market-1")
	time.Sleep(50 * time.Millisecond)

	// Verify the subscription is active by sending a message.
	earlyPayload := []byte(`{"type":"price_update","seq":1}`)
	hub.BroadcastMarket("market-1", earlyPayload)
	msg := receiveWithTimeout(t, client, time.Second)
	assert.Equal(t, earlyPayload, msg)

	// Now unsubscribe.
	hub.UnsubscribeMarket(client, "market-1")
	time.Sleep(50 * time.Millisecond)

	// After unsubscribing, broadcasts to market-1 should NOT reach the client.
	latePayload := []byte(`{"type":"price_update","seq":2}`)
	hub.BroadcastMarket("market-1", latePayload)

	assertNoMessage(t, client, 200*time.Millisecond)
}

func TestHub_SlowClient_DoesNotBlockHub(t *testing.T) {
	hub := setupHub(t)

	// slowClient has a tiny buffer so it fills up quickly.
	slowClient := &Client{
		ID:     "slow-client",
		UserID: "",
		Send:   make(chan []byte, 1), // intentionally small buffer
	}
	fastClient := newTestClient("fast-client", "")

	hub.Register(slowClient)
	hub.Register(fastClient)
	require.Eventually(t, func() bool {
		return hub.Clients() == 2
	}, time.Second, 10*time.Millisecond)

	hub.SubscribeMarket(slowClient, "market-1")
	hub.SubscribeMarket(fastClient, "market-1")
	time.Sleep(50 * time.Millisecond)

	// Fill the slow client's buffer.
	slowClient.Send <- []byte("filler")

	// Now broadcast several messages. The hub must not block on the slow
	// client and must still deliver to the fast client in a timely manner.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 5; i++ {
			hub.BroadcastMarket("market-1", []byte(`{"seq":`+string(rune('0'+i))+`}`))
		}
		close(done)
	}()

	// The broadcasts should complete without hanging, even though slowClient
	// cannot accept messages.
	select {
	case <-done:
		// Success -- hub did not block.
	case <-time.After(2 * time.Second):
		t.Fatal("hub BroadcastMarket blocked due to slow client")
	}

	// The fast client should have received messages.
	received := 0
	deadline := time.After(time.Second)
	for {
		select {
		case <-fastClient.Send:
			received++
			if received >= 5 {
				goto doneReceiving
			}
		case <-deadline:
			goto doneReceiving
		}
	}
doneReceiving:
	assert.GreaterOrEqual(t, received, 1, "fast client should have received at least one broadcast despite slow client")
}
