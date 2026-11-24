package keylight

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/events"
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
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(b)),
			Header:     make(http.Header),
		}, nil
	}
	if req.Method == http.MethodPut && req.URL.Path == "/elgato/lights" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"success":true}`))),
			Header:     make(http.Header),
		}, nil
	}
	return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader([]byte{})), Header: make(http.Header)}, nil
}

func newTestManager(logger *slog.Logger) (*Manager, *http.Client) {
	m := NewManager(logger)
	mockClient := &http.Client{Transport: &mockRoundTripper{}}
	m.clients = make(map[string]DeviceClient)
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
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		require.NoError(t, err)
	}
}

func TestCleanupStaleDevices(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager, mockHTTP := newTestManager(logger)
	mockHTTP.Transport = probeRoundTripper(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })

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
	manager.cleanupStaleLights(context.Background(), 5*time.Minute)

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

type probeRoundTripper func(*http.Request) (*http.Response, error)

func (f probeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCleanupHTTPProbe(t *testing.T) {
	disabled, enabled := false, true
	for _, tc := range []struct {
		name         string
		option       *bool
		optionSerial string
		response     string
		keep         bool
		calls        int
	}{
		{"default enabled", nil, "serial", `{"numberOfLights":1,"lights":[{"on":1,"brightness":42,"temperature":250}]}`, true, 1},
		{"explicit enabled", &enabled, "serial", `{"numberOfLights":1,"lights":[{"on":1,"brightness":42,"temperature":250}]}`, true, 1},
		{"disabled", &disabled, "serial", "", false, 0},
		{"other serial unaffected", &disabled, "other", `{"numberOfLights":1,"lights":[{"on":1,"brightness":42,"temperature":250}]}`, true, 1},
		{"empty state", nil, "serial", `{"numberOfLights":0,"lights":[]}`, false, 1},
		{"invalid response", nil, "serial", "invalid", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			m := NewManager(logger, config.LightConfig{SerialNumber: tc.optionSerial, HTTPProbeOnDisconnect: tc.option})
			m.SetEventBus(events.NewBus())
			getEvents := collectEvents(m.eventBus)
			old := time.Now().Add(-10 * time.Minute)
			m.lights["light"] = Light{ID: "light", SerialNumber: "serial", IP: net.ParseIP("127.0.0.1"), Port: 9123, LastSeen: old}
			calls := 0
			hc := &http.Client{Transport: probeRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/elgato/lights", r.URL.Path)
				deadline, ok := r.Context().Deadline()
				assert.True(t, ok)
				assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(tc.response)), Header: make(http.Header)}, nil
			})}
			m.clients["light"] = NewKeyLightClient("127.0.0.1", 9123, logger, hc)
			m.cleanupStaleLights(context.Background(), time.Minute)
			assert.Equal(t, tc.calls, calls)
			light, exists := m.lights["light"]
			assert.Equal(t, tc.keep, exists)
			evts := getEvents()
			require.Len(t, evts, 1)
			if tc.keep {
				assert.True(t, light.LastSeen.After(old))
				assert.Equal(t, 42, light.Brightness)
				assert.True(t, light.On)
				assert.Equal(t, events.LightStateChanged, evts[0].Type)
			} else {
				assert.NotContains(t, m.clients, "light")
				assert.Equal(t, events.LightRemoved, evts[0].Type)
			}
		})
	}
}

func TestCleanupProbePreservesConcurrentRefreshAndCancellation(t *testing.T) {
	for _, cancelDuringProbe := range []bool{false, true} {
		t.Run(strconv.FormatBool(cancelDuringProbe), func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			m := NewManager(logger)
			old := time.Now().Add(-10 * time.Minute)
			m.lights["light"] = Light{ID: "light", IP: net.ParseIP("127.0.0.1"), LastSeen: old}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			hc := &http.Client{Transport: probeRoundTripper(func(r *http.Request) (*http.Response, error) {
				if cancelDuringProbe {
					cancel()
				} else {
					// Acquiring this lock also verifies probes do not hold the manager lock.
					m.mu.Lock()
					light := m.lights["light"]
					light.LastSeen = time.Now()
					light.Brightness = 77
					m.lights["light"] = light
					m.mu.Unlock()
				}
				return nil, errors.New("probe failed")
			})}
			m.clients["light"] = NewKeyLightClient("127.0.0.1", 9123, logger, hc)
			m.cleanupStaleLights(ctx, time.Minute)
			require.Contains(t, m.lights, "light")
			if !cancelDuringProbe {
				assert.Equal(t, 77, m.lights["light"].Brightness)
			}
		})
	}
}
