package client

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func pipeDialer(conn net.Conn) func(network, address string) (net.Conn, error) {
	return func(network, address string) (net.Conn, error) {
		return conn, nil
	}
}

func writeEvent(t *testing.T, w interface{ Write([]byte) (int, error) }, eventType string) {
	t.Helper()
	e := Event{Type: eventType, Timestamp: time.Now(), Data: json.RawMessage(`{"id":"light-1"}`)}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		t.Fatalf("write event: %v", err)
	}
}

func TestClient_Watch(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	newWatchServer := func(t *testing.T, serverConn net.Conn) {
		t.Helper()
		go func() {
			reader := bufio.NewReader(serverConn)
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var req map[string]string
			if err := json.Unmarshal(line, &req); err != nil || req["action"] != "subscribe_events" {
				serverConn.Close()
				return
			}
			ack, _ := json.Marshal(map[string]any{"status": "ok", "subscribed": true})
			_, _ = serverConn.Write(append(ack, '\n'))
		}()
	}

	t.Run("receives events until server closes", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()

		newWatchServer(t, serverConn)

		c := New(logger, "/tmp/fake.sock")
		events, err := c.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch failed: %v", err)
		}

		writeEvent(t, serverConn, "light.state_changed")
		writeEvent(t, serverConn, "group.updated")

		e1 := <-events
		if e1.Type != "light.state_changed" {
			t.Fatalf("unexpected event type: %s", e1.Type)
		}
		e2 := <-events
		if e2.Type != "group.updated" {
			t.Fatalf("unexpected event type: %s", e2.Type)
		}

		serverConn.Close()
		select {
		case _, ok := <-events:
			if ok {
				t.Fatal("expected channel close after server disconnect")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for channel close")
		}
	})

	t.Run("context cancel closes channel", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()

		newWatchServer(t, serverConn)

		ctx, cancel := context.WithCancel(context.Background())
		c := New(logger, "/tmp/fake.sock")
		events, err := c.Watch(ctx)
		if err != nil {
			t.Fatalf("Watch failed: %v", err)
		}

		cancel()
		select {
		case _, ok := <-events:
			if ok {
				t.Fatal("expected channel close after cancel")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for channel close")
		}
	})

	t.Run("context cancel during handshake", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()

		go func() {
			// Swallow the subscribe request, never send the ack.
			_, _ = bufio.NewReader(serverConn).ReadBytes('\n')
		}()

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			_, err := New(logger, "/tmp/fake.sock").Watch(ctx)
			errCh <- err
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("expected handshake error")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Watch did not return promptly after cancel")
		}
	})

	t.Run("server error response", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()

		go func() {
			reader := bufio.NewReader(serverConn)
			_, _ = reader.ReadBytes('\n')
			resp, _ := json.Marshal(map[string]any{"error": "not supported"})
			_, _ = serverConn.Write(append(resp, '\n'))
		}()

		c := New(logger, "/tmp/fake.sock")
		if _, err := c.Watch(context.Background()); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Fatalf("expected server error, got: %v", err)
		}
	})

	t.Run("malformed event is skipped", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()

		newWatchServer(t, serverConn)

		c := New(logger, "/tmp/fake.sock")
		events, err := c.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch failed: %v", err)
		}

		if _, err := serverConn.Write([]byte("{not json\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
		writeEvent(t, serverConn, "light.removed")

		e := <-events
		if e.Type != "light.removed" {
			t.Fatalf("unexpected event type: %s", e.Type)
		}
		serverConn.Close()
	})
}

func TestHTTPClient_Watch(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	upgrader := websocket.Upgrader{}

	t.Run("receives events over websocket", func(t *testing.T) {
		gotKey := make(chan string, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotKey <- r.Header.Get("X-API-Key")
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			e := Event{Type: "light.discovered", Timestamp: time.Now(), Data: json.RawMessage(`{"id":"light-1"}`)}
			data, _ := json.Marshal(e)
			_ = conn.WriteMessage(websocket.TextMessage, data)
			// Hold the connection open until the client disconnects.
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}))
		defer srv.Close()

		c := NewHTTP(logger, srv.URL, "secret")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		events, err := c.Watch(ctx)
		if err != nil {
			t.Fatalf("Watch failed: %v", err)
		}
		if key := <-gotKey; key != "secret" {
			t.Fatalf("expected API key header, got %q", key)
		}

		e := <-events
		if e.Type != "light.discovered" {
			t.Fatalf("unexpected event type: %s", e.Type)
		}

		cancel()
		select {
		case _, ok := <-events:
			if ok {
				t.Fatal("expected channel close after cancel")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for channel close")
		}
	})

	t.Run("dial failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		c := NewHTTP(logger, srv.URL, "")
		if _, err := c.Watch(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})
}

// Every Watch goroutine must exit once its ctx is canceled or the peer
// disconnects; the count must settle back to baseline afterwards.
func TestWatch_NoGoroutineLeaks(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	baseline := runtime.NumGoroutine()

	// Unix: cancel while streaming.
	func() {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()
		go func() {
			reader := bufio.NewReader(serverConn)
			_, _ = reader.ReadBytes('\n')
			ack, _ := json.Marshal(map[string]any{"status": "ok", "subscribed": true})
			_, _ = serverConn.Write(append(ack, '\n'))
			_, _ = reader.ReadBytes('\n') // block until closed
		}()
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := New(logger, "/tmp/fake.sock").Watch(ctx)
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		cancel()
		for range ch {
		}
		serverConn.Close()
	}()

	// Unix: server closes while streaming.
	func() {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()
		go func() {
			reader := bufio.NewReader(serverConn)
			_, _ = reader.ReadBytes('\n')
			ack, _ := json.Marshal(map[string]any{"status": "ok", "subscribed": true})
			_, _ = serverConn.Write(append(ack, '\n'))
			serverConn.Close()
		}()
		ch, err := New(logger, "/tmp/fake.sock").Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		for range ch {
		}
	}()

	// Unix: handshake rejected.
	func() {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()
		go func() {
			reader := bufio.NewReader(serverConn)
			_, _ = reader.ReadBytes('\n')
			resp, _ := json.Marshal(map[string]any{"error": "nope"})
			_, _ = serverConn.Write(append(resp, '\n'))
		}()
		if _, err := New(logger, "/tmp/fake.sock").Watch(context.Background()); err == nil {
			t.Fatal("expected handshake error")
		}
		serverConn.Close()
	}()

	// Unix: cancel during handshake.
	func() {
		clientConn, serverConn := net.Pipe()
		oldDial := dial
		dial = pipeDialer(clientConn)
		defer func() { dial = oldDial }()
		go func() {
			_, _ = bufio.NewReader(serverConn).ReadBytes('\n')
		}()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		if _, err := New(logger, "/tmp/fake.sock").Watch(ctx); err == nil {
			t.Fatal("expected handshake error")
		}
		serverConn.Close()
	}()

	// WebSocket: cancel while streaming, then server close on a second watch.
	// httptest stops tracking hijacked conns, so the server side must be
	// closed explicitly rather than via CloseClientConnections.
	func() {
		upgrader := websocket.Upgrader{}
		serverConns := make(chan *websocket.Conn, 2)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			serverConns <- conn
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}))
		defer srv.Close()

		c := NewHTTP(logger, srv.URL, "")
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := c.Watch(ctx)
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		<-serverConns
		cancel()
		for range ch {
		}

		ch2, err := c.Watch(context.Background())
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		(<-serverConns).Close()
		for range ch2 {
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	t.Fatalf("goroutines leaked: baseline %d, now %d\n%s", baseline, runtime.NumGoroutine(), buf[:n])
}
