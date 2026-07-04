package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Event is a state-change event streamed from the daemon. It mirrors the
// daemon's internal event format used by both the unix socket and WebSocket
// transports.
type Event struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Watcher is implemented by clients that support streaming state-change
// events. The returned channel is closed when ctx is done or the
// connection drops; callers are responsible for re-subscribing.
type Watcher interface {
	Watch(ctx context.Context) (<-chan Event, error)
}

var (
	_ Watcher = (*Client)(nil)
	_ Watcher = (*HTTPClient)(nil)
)

// Watch subscribes to daemon events over the unix socket.
func (c *Client) Watch(ctx context.Context) (<-chan Event, error) {
	conn, err := dial("unix", c.socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}

	// Deadline covers the handshake only; streaming reads must not time out.
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	if err := json.NewEncoder(conn).Encode(map[string]string{"action": "subscribe_events"}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send subscribe request: %w", err)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to read subscribe response: %w", err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to clear connection deadline: %w", err)
	}

	var ack map[string]any
	if err := json.Unmarshal(line, &ack); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("invalid subscribe response: %w", err)
	}
	if errMsg, ok := ack["error"].(string); ok {
		_ = conn.Close()
		return nil, fmt.Errorf("server error: %s", errMsg)
	}
	if subscribed, _ := ack["subscribed"].(bool); !subscribed {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected subscribe response: %s", strings.TrimSpace(string(line)))
	}

	events := make(chan Event, 64)
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	go func() {
		defer close(events)
		defer close(done)
		defer conn.Close()
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var e Event
			if err := json.Unmarshal(line, &e); err != nil {
				c.logger.Warn("watch: skipping malformed event", "error", err)
				continue
			}
			select {
			case events <- e:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}

// Watch subscribes to daemon events over the WebSocket endpoint.
func (c *HTTPClient) Watch(ctx context.Context) (<-chan Event, error) {
	wsURL := c.baseURL + "/api/v1/ws"
	switch {
	case strings.HasPrefix(wsURL, "https://"):
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	case strings.HasPrefix(wsURL, "http://"):
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	}

	header := http.Header{}
	if c.apiKey != "" {
		header.Set("X-API-Key", c.apiKey)
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header) //nolint:bodyclose // body closed below
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to connect to %s (status %d): %w", wsURL, resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to %s: %w", wsURL, err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	events := make(chan Event, 64)
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second))
			_ = conn.Close()
		case <-done:
		}
	}()

	go func() {
		defer close(events)
		defer close(done)
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var e Event
			if err := json.Unmarshal(msg, &e); err != nil {
				c.logger.Warn("watch: skipping malformed event", "error", err)
				continue
			}
			select {
			case events <- e:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}
