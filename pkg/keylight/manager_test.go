package keylight

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRoundTripper implements http.RoundTripper for testing
// It returns canned responses for /elgato/lights and /elgato/lights PUT

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && req.URL.Path == "/elgato/lights" {
		resp := map[string]any{
			"numberOfLights": 1,
			"lights": []map[string]any{
				{
					"on":          1,
					"brightness":  50,
					"temperature": 200,
				},
			},
		}
		b, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(b)),
			Header:     make(http.Header),
		}, nil
	}
	if req.Method == http.MethodPut && req.URL.Path == "/elgato/lights" {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"success":true}`))),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewReader([]byte{})), Header: make(http.Header)}, nil
}

func newTestManager(logger *slog.Logger) (*Manager, *http.Client) {
	m := NewManager(logger)
	mockClient := &http.Client{Transport: &mockRoundTripper{}}
	m.clients = make(map[string]*KeyLightClient)
	m.lights = make(map[string]Light)
	return m, mockClient
}

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, _ := newTestManager(logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.lights)
}

func TestLightManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, mockHTTP := newTestManager(logger)
	ctx := context.Background()

	// Add a test light with mock HTTP client
	light := Light{
		ID:   "test-light",
		Name: "Test Light",
		IP:   net.ParseIP("192.168.1.1"),
		Port: 9123,
	}
	manager.lights[light.ID] = light
	manager.clients[light.ID] = NewKeyLightClient(light.IP.String(), light.Port, logger, mockHTTP)

	// Test getting light by ID
	retrievedLight, err := manager.GetLight(ctx, "test-light")
	require.NoError(t, err)
	assert.Equal(t, light.ID, retrievedLight.ID)
	assert.Equal(t, light.Name, retrievedLight.Name)

	// Test getting non-existent light
	_, err = manager.GetLight(ctx, "non-existent")
	assert.Error(t, err)

	// Test setting on/off state
	err = manager.SetLightState(ctx, "test-light", OnValue(true))
	require.NoError(t, err)

	// Test setting light brightness
	err = manager.SetLightState(ctx, "test-light", BrightnessValue(50))
	require.NoError(t, err)

	// Test setting light temperature
	err = manager.SetLightState(ctx, "test-light", TemperatureValue(5000))
	require.NoError(t, err)

	// Test setting state for non-existent light
	err = manager.SetLightState(ctx, "non-existent", OnValue(true))
	assert.Error(t, err)

	// Test input validation - brightness too high
	err = manager.SetLightState(ctx, "test-light", BrightnessValue(500))
	assert.Error(t, err)

	// Test helper methods (these call SetLightState internally)
	err = manager.SetLightBrightness(ctx, "test-light", 75)
	require.NoError(t, err)

	err = manager.SetLightTemperature(ctx, "test-light", 4500)
	require.NoError(t, err)

	err = manager.SetLightPower(ctx, "test-light", false)
	require.NoError(t, err)

	// Test GetLights and GetDiscoveredLights
	lights := manager.GetLights()
	assert.NotEmpty(t, lights)
	assert.Contains(t, lights, "test-light")

	discoveredLights := manager.GetDiscoveredLights()
	assert.Len(t, discoveredLights, 1)
	assert.Equal(t, light.ID, discoveredLights[0].ID)
}

func TestDiscovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, _ := newTestManager(logger)

	// Test discovery with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := manager.DiscoverLights(ctx, 5*time.Second)
	// Discovery may timeout, which is expected in tests
	if err != nil && err != context.DeadlineExceeded {
		require.NoError(t, err)
	}
}

func TestCleanupStaleDevices(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, mockHTTP := newTestManager(logger)

	// Add a test light with stale timestamp
	staleLight := Light{
		ID:       "stale-light",
		Name:     "Stale Light",
		IP:       net.ParseIP("192.168.1.2"),
		Port:     9123,
		LastSeen: time.Now().Add(-10 * time.Minute), // 10 minutes in the past
	}
	freshLight := Light{
		ID:       "fresh-light",
		Name:     "Fresh Light",
		IP:       net.ParseIP("192.168.1.3"),
		Port:     9123,
		LastSeen: time.Now(), // Just now
	}

	manager.lights[staleLight.ID] = staleLight
	manager.lights[freshLight.ID] = freshLight
	manager.clients[staleLight.ID] = NewKeyLightClient(staleLight.IP.String(), staleLight.Port, logger, mockHTTP)
	manager.clients[freshLight.ID] = NewKeyLightClient(freshLight.IP.String(), freshLight.Port, logger, mockHTTP)

	// Run cleanup with 5 minute timeout
	manager.cleanupStaleLights(5 * time.Minute)

	// Stale light should be removed, fresh light should remain
	assert.NotContains(t, manager.lights, staleLight.ID)
	assert.NotContains(t, manager.clients, staleLight.ID)
	assert.Contains(t, manager.lights, freshLight.ID)
	assert.Contains(t, manager.clients, freshLight.ID)
}

func TestAddLight(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, _ := newTestManager(logger)

	// Create a light to add
	newLight := Light{
		ID:   "new-light",
		Name: "New Light",
		IP:   net.ParseIP("192.168.1.4"),
		Port: 9123,
	}

	// Add the light
	manager.AddLight(context.Background(), newLight)

	// Verify the light was added
	assert.Contains(t, manager.lights, newLight.ID)
	assert.Contains(t, manager.clients, newLight.ID)

	// Test adding a light that already exists (should update)
	updatedLight := newLight
	updatedLight.Name = "Updated Light"
	manager.AddLight(context.Background(), updatedLight)

	// Verify the light was updated
	assert.Equal(t, "Updated Light", manager.lights[newLight.ID].Name)
}

func TestStartCleanupWorker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, _ := newTestManager(logger)

	// Create a context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start the cleanup worker with minimal intervals
	manager.StartCleanupWorker(ctx, 10*time.Millisecond, 5*time.Minute)

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)

	// Cancel the context to stop the worker
	cancel()

	// Give it a moment to stop
	time.Sleep(20 * time.Millisecond)

	// Test with invalid interval (should use default)
	ctx2 := t.Context()

	manager.StartCleanupWorker(ctx2, -1*time.Second, 5*time.Minute)

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)
}
