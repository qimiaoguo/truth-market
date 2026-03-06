package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers: mock Hub that records operations
// ---------------------------------------------------------------------------

// mockHub records Hub method calls so tests can assert that the Client's
// ReadPump dispatches messages correctly. The method signatures match those
// defined in hub_test.go (SubscribeMarket, UnsubscribeMarket, SubscribeUser,
// Unregister, Register, etc.).
type mockHub struct {
	mu sync.Mutex

	subscribedMarkets   []subscribeCall
	unsubscribedMarkets []unsubscribeCall
	subscribedUsers     []subscribeCall
	registered          []*Client
	unregistered        []*Client
}

type subscribeCall struct {
	client *Client
	id     string // market ID or user ID (without the "market:" / "user:" prefix)
}

type unsubscribeCall struct {
	client *Client
	id     string
}

func (h *mockHub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registered = append(h.registered, client)
}

func (h *mockHub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.unregistered = append(h.unregistered, client)
}

func (h *mockHub) SubscribeMarket(client *Client, marketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribedMarkets = append(h.subscribedMarkets, subscribeCall{client: client, id: marketID})
}

func (h *mockHub) UnsubscribeMarket(client *Client, marketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.unsubscribedMarkets = append(h.unsubscribedMarkets, unsubscribeCall{client: client, id: marketID})
}

func (h *mockHub) SubscribeUser(client *Client, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribedUsers = append(h.subscribedUsers, subscribeCall{client: client, id: userID})
}

func (h *mockHub) getSubscribedMarkets() []subscribeCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]subscribeCall, len(h.subscribedMarkets))
	copy(dst, h.subscribedMarkets)
	return dst
}

func (h *mockHub) getUnsubscribedMarkets() []unsubscribeCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]unsubscribeCall, len(h.unsubscribedMarkets))
	copy(dst, h.unsubscribedMarkets)
	return dst
}

func (h *mockHub) getSubscribedUsers() []subscribeCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]subscribeCall, len(h.subscribedUsers))
	copy(dst, h.subscribedUsers)
	return dst
}

func (h *mockHub) getUnregistered() []*Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]*Client, len(h.unregistered))
	copy(dst, h.unregistered)
	return dst
}

// ---------------------------------------------------------------------------
// Test helpers: WebSocket test server
// ---------------------------------------------------------------------------

// upgrader used by the test server to upgrade HTTP connections.
var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsTestEnv bundles a test server, mock hub, and the resulting client/conn
// pair so that tests can interact with the WebSocket from both sides.
type wsTestEnv struct {
	server     *httptest.Server
	hub        *mockHub
	client     *Client
	clientConn *websocket.Conn // the browser-side connection
}

// newWSTestEnv spins up an httptest server that upgrades to WebSocket,
// creates a Client with NewClient, and returns both ends of the connection.
func newWSTestEnv(t *testing.T, userID string) *wsTestEnv {
	t.Helper()

	hub := &mockHub{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to upgrade websocket: %v", err)
		}
		// The test will access the client through the env struct. We store
		// it via closure below.
		_ = conn // conn is captured by the env setup
	}))

	// Connect the "browser" side.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	browserConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		browserConn.Close()
		server.Close()
	})

	return &wsTestEnv{
		server:     server,
		hub:        hub,
		clientConn: browserConn,
	}
}

// newWSTestEnvFull creates both sides and also wires up a real Client backed
// by a server-side websocket.Conn. This uses a channel to pass the server-
// side connection out of the HTTP handler.
func newWSTestEnvFull(t *testing.T, userID string) *wsTestEnv {
	t.Helper()

	hub := &mockHub{}
	serverConnCh := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to upgrade websocket: %v", err)
		}
		serverConnCh <- conn
	}))

	// Connect the "browser" side.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	browserConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Wait for the server-side connection.
	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server-side websocket connection")
	}

	client := NewClient(hub, serverConn, userID)

	t.Cleanup(func() {
		browserConn.Close()
		server.Close()
	})

	return &wsTestEnv{
		server:     server,
		hub:        hub,
		client:     client,
		clientConn: browserConn,
	}
}

// ---------------------------------------------------------------------------
// TestNewClient
// ---------------------------------------------------------------------------

func TestNewClient_SetsFields(t *testing.T) {
	env := newWSTestEnvFull(t, "user-123")

	assert.NotEmpty(t, env.client.ID, "client should have a generated ID")
	assert.Equal(t, "user-123", env.client.UserID)
	assert.NotNil(t, env.client.Send, "Send channel should be initialized")
}

func TestNewClient_GeneratesUniqueIDs(t *testing.T) {
	env1 := newWSTestEnvFull(t, "user-a")
	env2 := newWSTestEnvFull(t, "user-b")

	assert.NotEqual(t, env1.client.ID, env2.client.ID,
		"each client should have a unique ID")
}

// ---------------------------------------------------------------------------
// TestClient_ReadPump_ParsesSubscribeMessage
// ---------------------------------------------------------------------------

func TestClient_ReadPump_ParsesSubscribeMessage(t *testing.T) {
	env := newWSTestEnvFull(t, "user-sub")

	// Start the read pump in a goroutine (it blocks until the connection closes).
	go env.client.ReadPump()

	// Send a subscribe message from the "browser" side.
	msg := WSMessage{
		Type:  "subscribe",
		Topic: "market:mkt-42",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	err = env.clientConn.WriteMessage(websocket.TextMessage, data)
	require.NoError(t, err)

	// Allow time for the read pump goroutine to process.
	assert.Eventually(t, func() bool {
		calls := env.hub.getSubscribedMarkets()
		return len(calls) == 1
	}, 2*time.Second, 50*time.Millisecond, "hub.SubscribeMarket should be called once")

	calls := env.hub.getSubscribedMarkets()
	assert.Equal(t, "mkt-42", calls[0].id)
	assert.Equal(t, env.client, calls[0].client)
}

func TestClient_ReadPump_ParsesSubscribeUserTopic(t *testing.T) {
	env := newWSTestEnvFull(t, "user-99")

	go env.client.ReadPump()

	msg := WSMessage{
		Type:  "subscribe",
		Topic: "user:user-99",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	err = env.clientConn.WriteMessage(websocket.TextMessage, data)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		calls := env.hub.getSubscribedUsers()
		return len(calls) == 1
	}, 2*time.Second, 50*time.Millisecond)

	calls := env.hub.getSubscribedUsers()
	assert.Equal(t, "user-99", calls[0].id)
}

// ---------------------------------------------------------------------------
// TestClient_ReadPump_ParsesUnsubscribeMessage
// ---------------------------------------------------------------------------

func TestClient_ReadPump_ParsesUnsubscribeMessage(t *testing.T) {
	env := newWSTestEnvFull(t, "user-unsub")

	go env.client.ReadPump()

	msg := WSMessage{
		Type:  "unsubscribe",
		Topic: "market:mkt-42",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	err = env.clientConn.WriteMessage(websocket.TextMessage, data)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		calls := env.hub.getUnsubscribedMarkets()
		return len(calls) == 1
	}, 2*time.Second, 50*time.Millisecond, "hub.UnsubscribeMarket should be called once")

	calls := env.hub.getUnsubscribedMarkets()
	assert.Equal(t, "mkt-42", calls[0].id)
	assert.Equal(t, env.client, calls[0].client)
}

// ---------------------------------------------------------------------------
// TestClient_ReadPump_InvalidJSON_SendsError
// ---------------------------------------------------------------------------

func TestClient_ReadPump_InvalidJSON_DoesNotPanic(t *testing.T) {
	env := newWSTestEnvFull(t, "user-bad")

	go env.client.ReadPump()

	// Send garbage that is not valid JSON.
	err := env.clientConn.WriteMessage(websocket.TextMessage, []byte(`not json at all`))
	require.NoError(t, err)

	// The read pump should not panic and should not subscribe anything.
	time.Sleep(200 * time.Millisecond)

	calls := env.hub.getSubscribedMarkets()
	assert.Empty(t, calls, "no market subscriptions should occur for invalid messages")
	userCalls := env.hub.getSubscribedUsers()
	assert.Empty(t, userCalls, "no user subscriptions should occur for invalid messages")
}

// ---------------------------------------------------------------------------
// TestClient_ReadPump_UnknownType_Ignored
// ---------------------------------------------------------------------------

func TestClient_ReadPump_UnknownType_Ignored(t *testing.T) {
	env := newWSTestEnvFull(t, "user-unk")

	go env.client.ReadPump()

	msg := WSMessage{
		Type:  "unknown_type",
		Topic: "market:mkt-1",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	err = env.clientConn.WriteMessage(websocket.TextMessage, data)
	require.NoError(t, err)

	// Unknown message types should be silently ignored.
	time.Sleep(200 * time.Millisecond)

	assert.Empty(t, env.hub.getSubscribedMarkets())
	assert.Empty(t, env.hub.getSubscribedUsers())
	assert.Empty(t, env.hub.getUnsubscribedMarkets())
}

// ---------------------------------------------------------------------------
// TestClient_WritePump_SendsToWebSocket
// ---------------------------------------------------------------------------

func TestClient_WritePump_SendsToWebSocket(t *testing.T) {
	env := newWSTestEnvFull(t, "user-write")

	// Start the write pump.
	go env.client.WritePump()

	// Build an event message to push through the Send channel.
	eventMsg := WSMessage{
		Type:    "event",
		Topic:   "market:mkt-1",
		Payload: json.RawMessage(`{"price":0.82}`),
	}
	data, err := json.Marshal(eventMsg)
	require.NoError(t, err)

	// Push onto the client's Send channel.
	env.client.Send <- data

	// Read from the browser-side connection.
	env.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, received, err := env.clientConn.ReadMessage()
	require.NoError(t, err)

	var decoded WSMessage
	err = json.Unmarshal(received, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "event", decoded.Type)
	assert.Equal(t, "market:mkt-1", decoded.Topic)
	assert.Contains(t, string(decoded.Payload), "0.82")
}

func TestClient_WritePump_MultipleMessages(t *testing.T) {
	env := newWSTestEnvFull(t, "user-multi")

	go env.client.WritePump()

	// Send three messages rapidly.
	for i := 0; i < 3; i++ {
		msg := WSMessage{
			Type:  "event",
			Topic: "market:mkt-1",
		}
		data, err := json.Marshal(msg)
		require.NoError(t, err)
		env.client.Send <- data
	}

	// Read all three from the browser side.
	env.clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 3; i++ {
		_, received, err := env.clientConn.ReadMessage()
		require.NoError(t, err, "should receive message %d", i+1)

		var decoded WSMessage
		err = json.Unmarshal(received, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "event", decoded.Type)
	}
}

// ---------------------------------------------------------------------------
// TestClient_WritePump_ClosedChannel_Stops
// ---------------------------------------------------------------------------

func TestClient_WritePump_ClosedChannel_Stops(t *testing.T) {
	env := newWSTestEnvFull(t, "user-close-ch")

	done := make(chan struct{})
	go func() {
		env.client.WritePump()
		close(done)
	}()

	// Close the Send channel to signal the write pump to exit.
	close(env.client.Send)

	select {
	case <-done:
		// WritePump exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("WritePump did not exit after Send channel was closed")
	}
}

// ---------------------------------------------------------------------------
// TestClient_Heartbeat_PingPong
// ---------------------------------------------------------------------------

func TestClient_Heartbeat_PingPong(t *testing.T) {
	env := newWSTestEnvFull(t, "user-ping")

	// Set up a pong handler on the browser side to record that pongs arrive.
	pongReceived := make(chan struct{}, 10)
	env.clientConn.SetPingHandler(func(appData string) error {
		pongReceived <- struct{}{}
		// Respond with a standard pong.
		return env.clientConn.WriteControl(
			websocket.PongMessage,
			[]byte(appData),
			time.Now().Add(time.Second),
		)
	})

	// Start both pumps -- ReadPump typically handles pong receipt, WritePump sends pings.
	go env.client.ReadPump()
	go env.client.WritePump()

	// Start a reader on the browser side so that control frames are processed.
	go func() {
		for {
			_, _, err := env.clientConn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Wait for at least one ping from the server.
	select {
	case <-pongReceived:
		// Server sent a ping and we received it.
	case <-time.After(15 * time.Second):
		t.Fatal("did not receive a ping from the server within timeout")
	}
}

// ---------------------------------------------------------------------------
// TestClient_Disconnect_UnregistersFromHub
// ---------------------------------------------------------------------------

func TestClient_Disconnect_UnregistersFromHub(t *testing.T) {
	env := newWSTestEnvFull(t, "user-disconnect")

	readDone := make(chan struct{})
	go func() {
		env.client.ReadPump()
		close(readDone)
	}()

	// Close the browser-side connection to simulate a disconnect.
	env.clientConn.Close()

	// Wait for ReadPump to detect the closure and unregister.
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("ReadPump did not exit after connection closed")
	}

	unregistered := env.hub.getUnregistered()
	require.Len(t, unregistered, 1, "client should unregister from hub on disconnect")
	assert.Equal(t, env.client, unregistered[0])
}

func TestClient_Disconnect_MultipleSubscriptions_UnregistersOnce(t *testing.T) {
	env := newWSTestEnvFull(t, "user-multi-disc")

	go env.client.ReadPump()

	// Subscribe to two topics first.
	for _, topic := range []string{"market:mkt-1", "market:mkt-2"} {
		msg := WSMessage{Type: "subscribe", Topic: topic}
		data, err := json.Marshal(msg)
		require.NoError(t, err)
		err = env.clientConn.WriteMessage(websocket.TextMessage, data)
		require.NoError(t, err)
	}

	// Wait for subscriptions to be processed.
	assert.Eventually(t, func() bool {
		return len(env.hub.getSubscribedMarkets()) == 2
	}, 2*time.Second, 50*time.Millisecond)

	// Close the connection.
	env.clientConn.Close()

	// Wait for unregister.
	assert.Eventually(t, func() bool {
		return len(env.hub.getUnregistered()) >= 1
	}, 2*time.Second, 50*time.Millisecond)

	// Should unregister exactly once.
	unregistered := env.hub.getUnregistered()
	assert.Len(t, unregistered, 1, "client should unregister exactly once regardless of subscription count")
}

// ---------------------------------------------------------------------------
// TestClient_SendChannelBufferSize
// ---------------------------------------------------------------------------

func TestClient_SendChannelBuffered(t *testing.T) {
	env := newWSTestEnvFull(t, "user-buf")

	// The Send channel should be buffered so that hub broadcasts don't block.
	// We should be able to write at least a few messages without a reader.
	for i := 0; i < 5; i++ {
		select {
		case env.client.Send <- []byte(`{"type":"event"}`):
			// success
		case <-time.After(100 * time.Millisecond):
			if i == 0 {
				t.Fatal("Send channel should accept at least one message without blocking")
			}
			// It's acceptable if the buffer is smaller than 5; we just need it
			// to be non-zero.
			return
		}
	}
}
