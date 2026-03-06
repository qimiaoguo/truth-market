package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection to the gateway.
type Client struct {
	ID     string
	UserID string
	Send   chan []byte
	conn   *websocket.Conn
	hub    HubInterface
}

// commandType enumerates the kinds of internal hub commands processed by the
// Run loop.
type commandType int

const (
	cmdSubscribeMarket commandType = iota
	cmdUnsubscribeMarket
	cmdSubscribeUser
	cmdBroadcastMarket
	cmdBroadcastUser
)

// hubCommand is a message sent to the Hub's Run goroutine so that all
// mutations to shared state are serialised on a single goroutine.
type hubCommand struct {
	typ      commandType
	client   *Client
	marketID string
	userID   string
	message  []byte
}

// Hub maintains the set of active clients and manages topic subscriptions.
// All mutating operations are funnelled through the Run goroutine to avoid
// the need for explicit locking on the subscription maps.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	commands   chan hubCommand

	// clients is the set of currently connected clients.
	clients map[*Client]bool

	// marketSubs maps a marketID to the set of clients subscribed to it.
	marketSubs map[string]map[*Client]bool

	// userSubs maps a userID to the set of clients subscribed to it.
	userSubs map[string]map[*Client]bool

	// mu protects the Clients() read that can be called from any goroutine.
	mu sync.RWMutex

	// clientCount is an atomic-friendly count kept in sync with clients map.
	clientCount int
}

// NewHub allocates and returns a new Hub. Callers must invoke Run in a
// separate goroutine before registering clients.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		stop:       make(chan struct{}),
		commands:   make(chan hubCommand, 256),
		clients:    make(map[*Client]bool),
		marketSubs: make(map[string]map[*Client]bool),
		userSubs:   make(map[string]map[*Client]bool),
	}
}

// Run is the hub's main event loop. It serialises all register, unregister,
// and subscription operations on a single goroutine so that no mutex is
// required for the subscription maps. It blocks until Stop is called.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			client.hub = h
			h.mu.Lock()
			h.clientCount = len(h.clients)
			h.mu.Unlock()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)

				// Remove from all market subscriptions.
				for mktID, subs := range h.marketSubs {
					delete(subs, client)
					if len(subs) == 0 {
						delete(h.marketSubs, mktID)
					}
				}

				// Remove from all user subscriptions.
				for uid, subs := range h.userSubs {
					delete(subs, client)
					if len(subs) == 0 {
						delete(h.userSubs, uid)
					}
				}

				h.mu.Lock()
				h.clientCount = len(h.clients)
				h.mu.Unlock()
			}

		case cmd := <-h.commands:
			h.processCommand(cmd)

		case <-h.stop:
			return
		}
	}
}

// processCommand handles a single hub command inside the Run loop.
func (h *Hub) processCommand(cmd hubCommand) {
	switch cmd.typ {
	case cmdSubscribeMarket:
		subs, ok := h.marketSubs[cmd.marketID]
		if !ok {
			subs = make(map[*Client]bool)
			h.marketSubs[cmd.marketID] = subs
		}
		subs[cmd.client] = true

	case cmdUnsubscribeMarket:
		if subs, ok := h.marketSubs[cmd.marketID]; ok {
			delete(subs, cmd.client)
			if len(subs) == 0 {
				delete(h.marketSubs, cmd.marketID)
			}
		}

	case cmdSubscribeUser:
		// Only authenticated clients (non-empty UserID) may subscribe to
		// user channels.
		if cmd.client.UserID == "" {
			return
		}
		subs, ok := h.userSubs[cmd.userID]
		if !ok {
			subs = make(map[*Client]bool)
			h.userSubs[cmd.userID] = subs
		}
		subs[cmd.client] = true

	case cmdBroadcastMarket:
		if subs, ok := h.marketSubs[cmd.marketID]; ok {
			for client := range subs {
				select {
				case client.Send <- cmd.message:
				default:
					// Skip slow clients to avoid blocking the hub.
				}
			}
		}

	case cmdBroadcastUser:
		if subs, ok := h.userSubs[cmd.userID]; ok {
			for client := range subs {
				select {
				case client.Send <- cmd.message:
				default:
					// Skip slow clients.
				}
			}
		}
	}
}

// Stop signals the hub's Run goroutine to exit.
func (h *Hub) Stop() {
	select {
	case h.stop <- struct{}{}:
	default:
	}
}

// Register queues a client for registration with the hub. The actual
// registration is processed asynchronously by the Run goroutine.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister queues a client for removal from the hub. The Send channel will
// be closed once the Run goroutine processes the unregistration.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// SubscribeMarket adds the client to the subscription set for the given
// market ID. Processed asynchronously by the Run goroutine.
func (h *Hub) SubscribeMarket(client *Client, marketID string) {
	h.commands <- hubCommand{
		typ:      cmdSubscribeMarket,
		client:   client,
		marketID: marketID,
	}
}

// UnsubscribeMarket removes the client from the subscription set for the
// given market ID. Processed asynchronously by the Run goroutine.
func (h *Hub) UnsubscribeMarket(client *Client, marketID string) {
	h.commands <- hubCommand{
		typ:      cmdUnsubscribeMarket,
		client:   client,
		marketID: marketID,
	}
}

// SubscribeUser adds the client to the subscription set for the given user
// ID. The client must be authenticated (non-empty UserID); unauthenticated
// clients are silently ignored.
func (h *Hub) SubscribeUser(client *Client, userID string) {
	h.commands <- hubCommand{
		typ:    cmdSubscribeUser,
		client: client,
		userID: userID,
	}
}

// BroadcastMarket sends a message to all clients subscribed to the given
// market ID. Slow clients whose Send channels are full are skipped to prevent
// the hub from blocking.
func (h *Hub) BroadcastMarket(marketID string, msg []byte) {
	h.commands <- hubCommand{
		typ:      cmdBroadcastMarket,
		marketID: marketID,
		message:  msg,
	}
}

// BroadcastUser sends a message to all clients subscribed to the given user
// ID. Slow clients whose Send channels are full are skipped.
func (h *Hub) BroadcastUser(userID string, msg []byte) {
	h.commands <- hubCommand{
		typ:     cmdBroadcastUser,
		userID:  userID,
		message: msg,
	}
}

// Clients returns the number of currently registered clients. It is safe to
// call from any goroutine.
func (h *Hub) Clients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clientCount
}
