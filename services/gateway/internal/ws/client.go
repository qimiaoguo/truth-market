package ws

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// writeWait is the maximum time allowed for a write to the peer.
	writeWait = 10 * time.Second

	// pongWait is the maximum time to wait for a pong response from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is the interval at which pings are sent to the peer. Must be
	// less than pongWait so the peer has time to respond.
	pingPeriod = 10 * time.Second

	// maxMessageSize is the maximum message size in bytes accepted from the peer.
	maxMessageSize = 4096

	// sendBufferSize is the buffer size for the Client.Send channel.
	sendBufferSize = 256
)

// HubInterface defines the subset of Hub methods that a Client depends on.
// This allows tests to supply a mock implementation.
type HubInterface interface {
	Register(client *Client)
	Unregister(client *Client)
	SubscribeMarket(client *Client, marketID string)
	UnsubscribeMarket(client *Client, marketID string)
	SubscribeUser(client *Client, userID string)
}

// NOTE: The Client struct is defined in hub.go. The fields conn and hub are
// added here as they are used by the ReadPump/WritePump methods but the
// struct definition itself lives in hub.go to keep hub-related types together.

// NewClient creates a new Client with a unique ID, wired to the provided hub
// and WebSocket connection.
func NewClient(hub HubInterface, conn *websocket.Conn, userID string) *Client {
	return &Client{
		ID:     uuid.New().String(),
		UserID: userID,
		Send:   make(chan []byte, sendBufferSize),
		conn:   conn,
		hub:    hub,
	}
}

// ReadPump reads messages from the WebSocket connection and dispatches them to
// the appropriate hub methods. It runs in its own goroutine and blocks until
// the connection is closed or an error occurs. On exit it unregisters the
// client from the hub and closes the connection.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		msg, err := ParseWSMessage(data)
		if err != nil {
			// Silently ignore malformed messages.
			continue
		}

		c.handleMessage(msg)
	}
}

// handleMessage routes a parsed WebSocket message to the appropriate handler
// based on its Type field.
func (c *Client) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case MessageTypeSubscribe:
		c.handleSubscribe(msg.Topic)
	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(msg.Topic)
	default:
		// Unknown message types are silently ignored.
	}
}

// handleSubscribe processes a subscribe message by parsing the topic and
// calling the appropriate hub subscription method.
func (c *Client) handleSubscribe(topic string) {
	prefix, id := splitTopic(topic)
	switch prefix {
	case "market":
		c.hub.SubscribeMarket(c, id)
	case "user":
		c.hub.SubscribeUser(c, id)
	}
}

// handleUnsubscribe processes an unsubscribe message by parsing the topic and
// calling the appropriate hub unsubscription method.
func (c *Client) handleUnsubscribe(topic string) {
	prefix, id := splitTopic(topic)
	switch prefix {
	case "market":
		c.hub.UnsubscribeMarket(c, id)
	}
}

// splitTopic splits a topic string of the form "prefix:id" into its two parts.
// For example, "market:mkt-42" returns ("market", "mkt-42").
func splitTopic(topic string) (string, string) {
	parts := strings.SplitN(topic, ":", 2)
	if len(parts) != 2 {
		return "", topic
	}
	return parts[0], parts[1]
}

// WritePump writes messages from the Send channel to the WebSocket connection.
// It also sends periodic ping frames to keep the connection alive. It runs in
// its own goroutine and exits when the Send channel is closed or a write error
// occurs.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				// The Send channel has been closed; send a close frame.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
