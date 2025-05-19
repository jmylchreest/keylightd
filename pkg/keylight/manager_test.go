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
		resp := map[string]interface{}{
			"numberOfLights": 1,
			"lights": []map[string]interface{}{
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

	// Add a test light with mock HTTP client
	light := Light{
		ID:   "test-light",
		Name: "Test Light",
		IP:   net.ParseIP("192.168.1.1"),
		Port: 9123,
	}
	manager.lights[light.ID] = light
	manager.clients[light.ID] = NewKeyLightClient(light.IP.String(), light.Port, logger, mockHTTP)

	// Test setting on/off state
	err := manager.SetLightState("test-light", OnValue(true))
	require.NoError(t, err)

	// Test setting light brightness
	err = manager.SetLightState("test-light", BrightnessValue(50))
	require.NoError(t, err)

	// Test setting light temperature 
	err = manager.SetLightState("test-light", TemperatureValue(5000))
	require.NoError(t, err)

	// Test setting state for non-existent light
	err = manager.SetLightState("non-existent", OnValue(true))
	assert.Error(t, err)

	// Test input validation - brightness too high
	err = manager.SetLightState("test-light", BrightnessValue(500))
	assert.Error(t, err)

	// Test helper methods (these call SetLightState internally)
	err = manager.SetLightBrightness("test-light", 75)
	require.NoError(t, err)
	
	err = manager.SetLightTemperature("test-light", 4500)
	require.NoError(t, err)
	
	err = manager.SetLightPower("test-light", false)
	require.NoError(t, err)
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
