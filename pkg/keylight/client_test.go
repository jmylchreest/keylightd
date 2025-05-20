package keylight

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPServer creates a test server with predefined responses
func mockHTTPServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/elgato/accessory-info":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"productName":         "Elgato Key Light",
				"hardwareBoardType":   2,
				"firmwareBuildNumber": 123,
				"firmwareVersion":     "1.0.3",
				"serialNumber":        "KL12345678",
				"displayName":         "Office Key Light",
				"features":            []string{"lights"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/elgato/lights":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"numberOfLights": 1,
				"lights": []map[string]any{
					{
						"on":          1,
						"brightness":  50,
						"temperature": 200,
					},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/elgato/lights":
			// Decode and validate request
			var reqBody struct {
				Lights []map[string]any `json:"lights"`
			}
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			// Verify request format
			if len(reqBody.Lights) == 0 {
				http.Error(w, "no lights in request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func TestNewKeyLightClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	// Test with default HTTP client
	client := NewKeyLightClient("192.168.1.100", 9123, logger)
	assert.NotNil(t, client)
	assert.Equal(t, "http://192.168.1.100:9123/elgato", client.baseURL)
	assert.NotNil(t, client.httpClient)
	
	// Test with custom HTTP client
	customHTTP := &http.Client{}
	client = NewKeyLightClient("192.168.1.100", 9123, logger, customHTTP)
	assert.NotNil(t, client)
	assert.Equal(t, customHTTP, client.httpClient)
}

func TestGetAccessoryInfo(t *testing.T) {
	server := mockHTTPServer(t)
	defer server.Close()
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	// Extract host and port from test server
	client := NewKeyLightClient(server.URL[7:], 0, logger, server.Client())
	// Override baseURL to use test server
	client.baseURL = server.URL + "/elgato"
	
	// Test successful request
	info, err := client.GetAccessoryInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Elgato Key Light", info.ProductName)
	assert.Equal(t, 2, info.HardwareBoardType)
	assert.Equal(t, 123, info.FirmwareBuildNumber)
	assert.Equal(t, "1.0.3", info.FirmwareVersion)
	assert.Equal(t, "KL12345678", info.SerialNumber)
	assert.Equal(t, "Office Key Light", info.DisplayName)
	
	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	_, err = client.GetAccessoryInfo(ctx)
	assert.Error(t, err)
	
	// Test invalid URL
	badClient := NewKeyLightClient("invalid:url", 9123, logger)
	_, err = badClient.GetAccessoryInfo(context.Background())
	assert.Error(t, err)
}

func TestGetLightState(t *testing.T) {
	server := mockHTTPServer(t)
	defer server.Close()
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewKeyLightClient(server.URL[7:], 0, logger, server.Client())
	client.baseURL = server.URL + "/elgato"
	
	// Test successful request
	state, err := client.GetLightState(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, state.NumberOfLights)
	assert.Len(t, state.Lights, 1)
	assert.Equal(t, 1, state.Lights[0].On)
	assert.Equal(t, 50, state.Lights[0].Brightness)
	assert.Equal(t, 200, state.Lights[0].Temperature)
	
	// Test server error
	badPathClient := NewKeyLightClient(server.URL[7:], 0, logger, server.Client())
	badPathClient.baseURL = server.URL + "/nonexistent"
	_, err = badPathClient.GetLightState(context.Background())
	assert.Error(t, err)
}

func TestSetLightState(t *testing.T) {
	server := mockHTTPServer(t)
	defer server.Close()
	
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	
	client := NewKeyLightClient(server.URL[7:], 0, logger, server.Client())
	client.baseURL = server.URL + "/elgato"
	
	// Test successful request
	err := client.SetLightState(context.Background(), true, 75, 250)
	require.NoError(t, err)
	
	// Test with invalid URL
	badClient := NewKeyLightClient("invalid:url", 9123, logger)
	err = badClient.SetLightState(context.Background(), true, 75, 250)
	assert.Error(t, err)
}

func TestClientWithServerErrors(t *testing.T) {
	// Server that always returns 500 error
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewKeyLightClient(errorServer.URL[7:], 0, logger, errorServer.Client())
	client.baseURL = errorServer.URL + "/elgato"
	
	// Test accessory info with server error
	_, err := client.GetAccessoryInfo(context.Background())
	assert.Error(t, err)
	
	// Test get state with server error
	_, err = client.GetLightState(context.Background())
	assert.Error(t, err)
	
	// Test set state with server error
	err = client.SetLightState(context.Background(), true, 75, 250)
	assert.Error(t, err)
}

func TestClientWithMalformedResponses(t *testing.T) {
	// Server that returns invalid JSON
	badJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{not valid json"))
	}))
	defer badJSONServer.Close()
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	
	client := NewKeyLightClient(badJSONServer.URL[7:], 0, logger, badJSONServer.Client())
	client.baseURL = badJSONServer.URL + "/elgato"
	
	// Test accessory info with malformed response
	_, err := client.GetAccessoryInfo(context.Background())
	assert.Error(t, err)
	
	// Test get state with malformed response
	_, err = client.GetLightState(context.Background())
	assert.Error(t, err)
}