package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	watchHandshakeTimeout = 10 * time.Second
	// The daemon pings every ~54s; a stale read deadline detects dead peers.
	wsReadTimeout  = 90 * time.Second
	wsWriteTimeout = 10 * time.Second
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

// streamEvents decodes raw messages from next into a channel of events.
// closeConn unblocks next on ctx cancellation and runs again when the read
// loop exits; it must be safe to call concurrently and repeatedly.
func streamEvents(ctx context.Context, logger *slog.Logger, next func() ([]byte, error), closeConn func()) <-chan Event {
	events := make(chan Event, 64)
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			closeConn()
		case <-done:
		}
	}()

	go func() {
		defer close(events)
		defer close(done)
		defer closeConn()
		for {
			msg, err := next()
			if err != nil {
				return
			}
			var e Event
			if err := json.Unmarshal(msg, &e); err != nil {
				logger.Warn("watch: skipping malformed event", "error", err)
				continue
			}
			select {
			case events <- e:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events
}

// Watch subscribes to daemon events over the unix socket.
func (c *Client) Watch(ctx context.Context) (<-chan Event, error) {
	conn, err := dial("unix", c.socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}

	reader := bufio.NewReader(conn)
	if err := subscribeEvents(ctx, conn, reader); err != nil {
		_ = conn.Close()
		return nil, err
	}

	next := func() ([]byte, error) { return reader.ReadBytes('\n') }
	return streamEvents(ctx, c.logger, next, func() { _ = conn.Close() }), nil
}

// subscribeEvents performs the subscribe_events handshake. The deadline
// bounds a hung daemon; the goroutine unblocks reads on ctx cancellation.
func subscribeEvents(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(watchHandshakeTimeout)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}
	if err := json.NewEncoder(conn).Encode(map[string]string{"action": "subscribe_events"}); err != nil {
		return fmt.Errorf("failed to send subscribe request: %w", err)
	}
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("failed to read subscribe response: %w", err)
	}
	// Best-effort: if this fails the conn is already dead and the stream
	// loop will surface it as a closed channel.
	_ = conn.SetDeadline(time.Time{})

	var ack map[string]any
	if err := json.Unmarshal(line, &ack); err != nil {
		return fmt.Errorf("invalid subscribe response: %w", err)
	}
	if errMsg, ok := ack["error"].(string); ok {
		return fmt.Errorf("server error: %s", errMsg)
	}
	if subscribed, _ := ack["subscribed"].(bool); !subscribed {
		return fmt.Errorf("unexpected subscribe response: %s", strings.TrimSpace(string(line)))
	}
	return nil
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

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header) //nolint:bodyclose // closed below
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("failed to connect to %s (status %d): %w", wsURL, resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to %s: %w", wsURL, err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	conn.SetPingHandler(func(msg string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(msg), time.Now().Add(wsWriteTimeout))
	})

	next := func() ([]byte, error) {
		_, msg, err := conn.ReadMessage()
		if err == nil {
			_ = conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
		}
		return msg, err
	}
	closeConn := func() {
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		_ = conn.Close()
	}
	return streamEvents(ctx, c.logger, next, closeConn), nil
}
