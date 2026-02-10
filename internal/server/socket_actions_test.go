package server

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"log/slog"
)

// setupSocketTest creates a server with lights and returns the socket path and cleanup.
func setupSocketTest(t *testing.T) (*Server, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "keylight-socket-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	socketPath := filepath.Join(tempDir, "keylightd.sock")
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg, err := config.Load("config", cfgPath)
	require.NoError(t, err)

	cfg.Config.Server.UnixSocket = socketPath
	cfg.Config.API.ListenAddress = "" // No HTTP for these tests
	cfg.Config.Logging.Level = "debug"

	lightManager := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light-1": {
				ID:          "light-1",
				Name:        "Test Light 1",
				Brightness:  50,
				Temperature: 5000,
				On:          true,
				LastSeen:    time.Now(),
			},
			"light-2": {
				ID:          "light-2",
				Name:        "Test Light 2",
				Brightness:  75,
				Temperature: 4000,
				On:          false,
				LastSeen:    time.Now(),
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server := New(logger, cfg, lightManager)

	err = server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	time.Sleep(50 * time.Millisecond)
	return server, socketPath
}

// socketRequest sends a JSON request and reads the JSON response.
func socketRequest(t *testing.T, socketPath string, req map[string]any) map[string]any {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	err = json.NewEncoder(conn).Encode(req)
	require.NoError(t, err)

	var resp map[string]any
	err = json.NewDecoder(conn).Decode(&resp)
	require.NoError(t, err)

	return resp
}

// socketRequestKeepConn sends a request on an existing connection and reads the response.
func socketRequestKeepConn(t *testing.T, conn net.Conn, req map[string]any) map[string]any {
	t.Helper()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	err := json.NewEncoder(conn).Encode(req)
	require.NoError(t, err)

	var resp map[string]any
	err = json.NewDecoder(conn).Decode(&resp)
	require.NoError(t, err)

	return resp
}

// --- Ping ---

func TestSocketAction_Ping(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{"action": "ping"})
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "pong", resp["message"])
}

func TestSocketAction_PingWithID(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{"action": "ping", "id": "req-123"})
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "pong", resp["message"])
	assert.Equal(t, "req-123", resp["id"])
}

// --- Get Light ---

func TestSocketAction_GetLight(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "get_light",
		"data":   map[string]any{"id": "light-1"},
	})
	assert.Equal(t, "ok", resp["status"])
	light, ok := resp["light"].(map[string]any)
	require.True(t, ok, "light should be a map")
	assert.Equal(t, "light-1", light["id"])
	assert.Equal(t, "Test Light 1", light["name"])
}

func TestSocketAction_GetLight_NotFound(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "get_light",
		"data":   map[string]any{"id": "no-such-light"},
	})
	assert.Contains(t, resp, "error")
}

func TestSocketAction_GetLight_MissingID(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "get_light",
		"data":   map[string]any{},
	})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"], "missing light ID")
}

// --- Set Light State ---

func TestSocketAction_SetLightState_SingleProperty(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "set_light_state",
		"data": map[string]any{
			"id":       "light-1",
			"property": "brightness",
			"value":    float64(80),
		},
	})
	assert.Equal(t, "ok", resp["status"])
}

func TestSocketAction_SetLightState_MultiProperty(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "set_light_state",
		"data": map[string]any{
			"id":         "light-1",
			"on":         true,
			"brightness": float64(60),
		},
	})
	assert.Equal(t, "ok", resp["status"])
}

func TestSocketAction_SetLightState_MissingID(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "set_light_state",
		"data":   map[string]any{"brightness": float64(50)},
	})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"], "missing id")
}

func TestSocketAction_SetLightState_MissingProperties(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "set_light_state",
		"data":   map[string]any{"id": "light-1"},
	})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"], "missing property")
}

// --- Groups ---

func TestSocketAction_CreateAndListGroups(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	// Create a group
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	createResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "create_group",
		"data": map[string]any{
			"name":   "Office",
			"lights": []any{"light-1", "light-2"},
		},
	})
	assert.Equal(t, "ok", createResp["status"])
	createdGroup, ok := createResp["group"].(map[string]any)
	require.True(t, ok)
	groupID := createdGroup["id"].(string)
	assert.NotEmpty(t, groupID)
	assert.Equal(t, "Office", createdGroup["name"])

	// List groups
	listResp := socketRequestKeepConn(t, conn, map[string]any{"action": "list_groups"})
	assert.Equal(t, "ok", listResp["status"])
	groups, ok := listResp["groups"].([]any)
	require.True(t, ok)
	assert.Len(t, groups, 1)

	// Get group
	getResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "get_group",
		"data":   map[string]any{"id": groupID},
	})
	assert.Equal(t, "ok", getResp["status"])
	group, ok := getResp["group"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Office", group["name"])

	// Set group lights
	setLightsResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "set_group_lights",
		"data": map[string]any{
			"id":     groupID,
			"lights": []any{"light-1"},
		},
	})
	assert.Equal(t, "ok", setLightsResp["status"])

	// Delete group
	deleteResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "delete_group",
		"data":   map[string]any{"id": groupID},
	})
	assert.Equal(t, "ok", deleteResp["status"])

	// Verify deleted
	getResp2 := socketRequestKeepConn(t, conn, map[string]any{
		"action": "get_group",
		"data":   map[string]any{"id": groupID},
	})
	assert.Contains(t, getResp2, "error")
}

func TestSocketAction_CreateGroup_MissingName(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "create_group",
		"data":   map[string]any{"lights": []any{"light-1"}},
	})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"], "missing name")
}

// --- Set Group State ---

func TestSocketAction_SetGroupState(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Create group
	createResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "create_group",
		"data":   map[string]any{"name": "studio", "lights": []any{"light-1"}},
	})
	groupID := createResp["group"].(map[string]any)["id"].(string)

	// Set state using single property mode
	stateResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "set_group_state",
		"data":   map[string]any{"id": groupID, "property": "on", "value": true},
	})
	assert.Equal(t, "ok", stateResp["status"])

	// Set state using multi-property mode
	multiResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "set_group_state",
		"data":   map[string]any{"id": groupID, "on": true, "brightness": float64(80)},
	})
	assert.Equal(t, "ok", multiResp["status"])
}

// --- API Key actions ---

func TestSocketAction_APIKeyLifecycle(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Create API key
	addResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_add",
		"data":   map[string]any{"name": "test-socket-key"},
	})
	assert.Equal(t, "ok", addResp["status"])
	keyData, ok := addResp["key"].(map[string]any)
	require.True(t, ok)
	keyStr := keyData["key"].(string)
	assert.NotEmpty(t, keyStr)

	// List API keys
	listResp := socketRequestKeepConn(t, conn, map[string]any{"action": "apikey_list"})
	assert.Equal(t, "ok", listResp["status"])
	keys, ok := listResp["keys"].([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(keys), 1)

	// Set disabled status
	disableResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_set_disabled_status",
		"data":   map[string]any{"key_or_name": "test-socket-key", "disabled": true},
	})
	assert.Equal(t, "ok", disableResp["status"])

	// Re-enable
	enableResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_set_disabled_status",
		"data":   map[string]any{"key_or_name": "test-socket-key", "disabled": false},
	})
	assert.Equal(t, "ok", enableResp["status"])

	// Delete API key
	deleteResp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_delete",
		"data":   map[string]any{"key": keyStr},
	})
	assert.Equal(t, "ok", deleteResp["status"])
}

func TestSocketAction_APIKeyAdd_DuplicateName(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_add",
		"data":   map[string]any{"name": "dup-key"},
	})
	resp := socketRequestKeepConn(t, conn, map[string]any{
		"action": "apikey_add",
		"data":   map[string]any{"name": "dup-key"},
	})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"].(string), "already exists")
}

func TestSocketAction_APIKeyAdd_WithExpiration(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{
		"action": "apikey_add",
		"data":   map[string]any{"name": "expiring-key", "expires_in": "720h"},
	})
	assert.Equal(t, "ok", resp["status"])
}

// --- Health ---

func TestSocketAction_Health(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{"action": "health"})
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "ok", resp["health"])
}

// --- Unknown action ---

func TestSocketAction_UnknownAction(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	resp := socketRequest(t, socketPath, map[string]any{"action": "foobar"})
	assert.Contains(t, resp, "error")
	assert.Contains(t, resp["error"], "unknown action")
}

// --- Multiple requests on same connection ---

func TestSocketAction_MultipleRequestsSameConnection(t *testing.T) {
	_, socketPath := setupSocketTest(t)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Send multiple requests on the same connection
	for i := 0; i < 5; i++ {
		resp := socketRequestKeepConn(t, conn, map[string]any{"action": "ping"})
		assert.Equal(t, "ok", resp["status"])
		assert.Equal(t, "pong", resp["message"])
	}
}

// --- Subscribe events ---

func TestSocketAction_SubscribeEvents(t *testing.T) {
	server, socketPath := setupSocketTest(t)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Subscribe to events
	err = json.NewEncoder(conn).Encode(map[string]any{"action": "subscribe_events"})
	require.NoError(t, err)

	// Read the ack
	var ack map[string]any
	err = json.NewDecoder(conn).Decode(&ack)
	require.NoError(t, err)
	assert.Equal(t, "ok", ack["status"])
	assert.Equal(t, true, ack["subscribed"])

	// Publish an event via the server's event bus
	server.eventBus.Publish(events.NewEvent(events.LightStateChanged, map[string]string{"id": "light-1"}))

	// Read the streamed event
	var evt map[string]any
	err = json.NewDecoder(conn).Decode(&evt)
	require.NoError(t, err)
	assert.Equal(t, "light.state_changed", evt["type"])
}
