// Package ws provides a WebSocket hub for broadcasting real-time events
// to connected clients.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jmylchreest/keylightd/internal/events"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer (clients only send pings/pongs).
	maxMessageSize = 512

	// Size of the per-client send buffer.
	sendBufferSize = 64
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub manages a set of active WebSocket clients and broadcasts events.
type Hub struct {
	logger     *slog.Logger
	clients    map[*Client]struct{}
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	unsub      func() // unsubscribe from event bus
}

// NewHub creates a Hub and subscribes to the event bus.
func NewHub(logger *slog.Logger, bus *events.Bus) *Hub {
	h := &Hub{
		logger:     logger,
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	// Subscribe to the event bus; forward events to the broadcast channel.
	h.unsub = bus.Subscribe(func(e events.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			logger.Error("ws: failed to marshal event", "error", err)
			return
		}
		// Non-blocking send; if the broadcast channel is full, log and drop.
		select {
		case h.broadcast <- data:
		default:
			logger.Warn("ws: broadcast channel full, dropping event", "type", e.Type)
		}
	})

	return h
}

// Run starts the hub's main loop. It blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	defer h.unsub()
	h.logger.Info("ws: hub started")

	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			h.logger.Info("ws: hub stopped")
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			count := len(h.clients)
			h.mu.Unlock()
			h.logger.Info("ws: client connected", "clients", count)

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				close(c.send)
				delete(h.clients, c)
			}
			count := len(h.clients)
			h.mu.Unlock()
			h.logger.Info("ws: client disconnected", "clients", count)

		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Client buffer full — schedule disconnect.
					go func(cl *Client) {
						h.unregister <- cl
					}(c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// NewClient creates a new Client attached to this hub.
func (h *Hub) NewClient(conn *websocket.Conn) *Client {
	return &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
// A goroutine per client runs this method.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}

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

// ReadPump reads messages from the WebSocket connection.
// We don't expect clients to send meaningful data, but we must read
// to process control frames (ping/pong/close).
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
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.hub.logger.Debug("ws: read error", "error", err)
			}
			return
		}
		// Discard any client messages; this is a server-push-only endpoint.
	}
}
