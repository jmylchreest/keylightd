package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func startTestHub(t *testing.T) (*Hub, *events.Bus, context.CancelFunc) {
	t.Helper()
	bus := events.NewBus()
	logger := testLogger()
	hub := NewHub(logger, bus)

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	// Give the hub's Run loop time to start
	time.Sleep(10 * time.Millisecond)

	return hub, bus, cancel
}

func startTestServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	logger := testLogger()
	server := httptest.NewServer(Handler(hub, logger))
	t.Cleanup(server.Close)
	return server
}

func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func dialWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL(server), nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

// --- Hub lifecycle tests ---

func TestNewHub_CreatesHub(t *testing.T) {
	bus := events.NewBus()
	logger := testLogger()
	hub := NewHub(logger, bus)

	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.broadcast)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.NotNil(t, hub.unsub)
}

func TestHub_RunAndStop(t *testing.T) {
	hub, _, cancel := startTestHub(t)
	defer cancel()

	assert.Equal(t, 0, hub.ClientCount())

	// Cancel should stop gracefully
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHub_ClientCount(t *testing.T) {
	hub, _, cancel := startTestHub(t)
	defer cancel()

	server := startTestServer(t, hub)

	assert.Equal(t, 0, hub.ClientCount())

	conn1 := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	conn2 := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 2, hub.ClientCount())

	// Close one connection
	conn1.Close()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	conn2.Close()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}

// --- Event broadcasting tests ---

func TestHub_BroadcastsEventToClients(t *testing.T) {
	hub, bus, cancel := startTestHub(t)
	defer cancel()

	server := startTestServer(t, hub)
	conn := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)

	// Publish an event via the bus
	bus.Publish(events.NewEvent(events.LightStateChanged, map[string]string{"id": "light-1"}))

	// Read the message from the WebSocket
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)

	// Parse the event
	var evt events.Event
	require.NoError(t, json.Unmarshal(msg, &evt))
	assert.Equal(t, events.LightStateChanged, evt.Type)

	var data map[string]string
	require.NoError(t, json.Unmarshal(evt.Data, &data))
	assert.Equal(t, "light-1", data["id"])
}

func TestHub_BroadcastsToMultipleClients(t *testing.T) {
	hub, bus, cancel := startTestHub(t)
	defer cancel()

	server := startTestServer(t, hub)
	conn1 := dialWS(t, server)
	conn2 := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)

	bus.Publish(events.NewEvent(events.LightDiscovered, map[string]string{"id": "new-light"}))

	var wg sync.WaitGroup
	wg.Add(2)

	readEvent := func(conn *websocket.Conn) events.Event {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		var evt events.Event
		require.NoError(t, json.Unmarshal(msg, &evt))
		return evt
	}

	var evt1, evt2 events.Event
	go func() { defer wg.Done(); evt1 = readEvent(conn1) }()
	go func() { defer wg.Done(); evt2 = readEvent(conn2) }()
	wg.Wait()

	assert.Equal(t, events.LightDiscovered, evt1.Type)
	assert.Equal(t, events.LightDiscovered, evt2.Type)
}

func TestHub_MultipleEventsInSequence(t *testing.T) {
	hub, bus, cancel := startTestHub(t)
	defer cancel()

	server := startTestServer(t, hub)
	conn := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)

	// Send multiple events
	eventTypes := []events.EventType{
		events.LightDiscovered,
		events.LightStateChanged,
		events.GroupCreated,
	}
	for _, et := range eventTypes {
		bus.Publish(events.NewEvent(et, nil))
	}

	// Read all events
	var received []events.EventType
	for i := 0; i < 3; i++ {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		var evt events.Event
		require.NoError(t, json.Unmarshal(msg, &evt))
		received = append(received, evt.Type)
	}

	assert.Equal(t, eventTypes, received)
}

// --- Handler tests ---

func TestHandler_UpgradesConnection(t *testing.T) {
	hub, _, cancel := startTestHub(t)
	defer cancel()

	server := startTestServer(t, hub)

	// Dial should succeed
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL(server), nil)
	require.NoError(t, err)
	defer conn.Close()

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func TestHandler_NonWebSocketRequest(t *testing.T) {
	hub, _, cancel := startTestHub(t)
	defer cancel()

	logger := testLogger()
	server := httptest.NewServer(Handler(hub, logger))
	defer server.Close()

	// A regular HTTP GET (not a WebSocket upgrade) should fail
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// gorilla/websocket returns 400 Bad Request for non-upgrade requests
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Hub shutdown tests ---

func TestHub_ShutdownClosesClients(t *testing.T) {
	hub, _, cancel := startTestHub(t)

	server := startTestServer(t, hub)
	conn := dialWS(t, server)
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, 1, hub.ClientCount())

	// Cancel the hub context — should close all client connections
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Try to read — should get an error (connection closed)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := conn.ReadMessage()
	assert.Error(t, err)
}

// --- NewClient tests ---

func TestNewClient(t *testing.T) {
	bus := events.NewBus()
	logger := testLogger()
	hub := NewHub(logger, bus)

	// We can't create a real websocket.Conn easily, but we can test the factory
	client := hub.NewClient(nil) // nil conn is okay for testing the struct fields
	assert.Equal(t, hub, client.hub)
	assert.Nil(t, client.conn)
	assert.NotNil(t, client.send)
	assert.Equal(t, sendBufferSize, cap(client.send))
}
