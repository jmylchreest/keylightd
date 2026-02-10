package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupIntegrationTest prepares a test environment with server and config
func setupIntegrationTest(t *testing.T) (*Server, *config.Config, string) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "keylight-integration-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// Setup socket path in temp dir
	socketPath := filepath.Join(tempDir, "keylightd.sock")

	// Create a minimal config
	v := config.New(nil)
	v.Config.Server.UnixSocket = socketPath
	v.Config.API.ListenAddress = "127.0.0.1:0" // Use random available port
	v.Config.Logging.Level = "debug"

	// Create a test light manager
	lightManager := &mockLightManager{
		lights: map[string]*keylight.Light{
			"test-light-1": {
				ID:          "test-light-1",
				Name:        "Test Light 1",
				Brightness:  50,
				Temperature: 5000,
				On:          true,
				LastSeen:    time.Now(),
			},
		},
	}

	// Create server with a simple logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server := New(logger, v, lightManager, VersionInfo{Version: "test", Commit: "abc1234", BuildDate: "2026-01-01T00:00:00Z"})

	return server, v, socketPath
}

// TestServerSocketStartStop tests that the server starts and stops correctly
func TestServerSocketStartStop(t *testing.T) {
	server, _, socketPath := setupIntegrationTest(t)

	// Start server
	err := server.Start()
	require.NoError(t, err, "Server should start without error")

	// Check that socket was created
	_, err = os.Stat(socketPath)
	require.NoError(t, err, "Socket file should exist")

	// Stop server
	server.Stop()

	// Give it a moment to clean up
	time.Sleep(50 * time.Millisecond)

	// Check that socket was removed
	_, err = os.Stat(socketPath)
	require.True(t, os.IsNotExist(err), "Socket file should be removed on shutdown")
}

// TestSocketCommunication tests communication with the server over a Unix socket
func TestSocketCommunication(t *testing.T) {
	server, _, socketPath := setupIntegrationTest(t)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Connect to socket
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err, "Should connect to socket")
	defer conn.Close()

	// Send a request to list lights
	req := map[string]string{
		"action": "list_lights",
	}
	err = json.NewEncoder(conn).Encode(req)
	require.NoError(t, err, "Should send request")

	// Read response
	var resp map[string]any
	err = json.NewDecoder(conn).Decode(&resp)
	require.NoError(t, err, "Should receive response")

	// Check response
	assert.Contains(t, resp, "lights", "Response should contain lights")
	lights, ok := resp["lights"].(map[string]any)
	require.True(t, ok, "Lights should be a map")
	assert.Contains(t, lights, "test-light-1", "Response should contain test light")
}

// setupHTTPIntegrationTest creates a server with a known API key and HTTP listener on a random port.
// It returns the server, the API key string, and the base URL (e.g., "http://127.0.0.1:PORT").
func setupHTTPIntegrationTest(t *testing.T) (*Server, string, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "keylight-http-integration-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	socketPath := filepath.Join(tempDir, "keylightd.sock")
	cfgPath := filepath.Join(tempDir, "config.yaml")

	// Load config (creates file, sets defaults)
	cfg, err := config.Load("config", cfgPath)
	require.NoError(t, err)

	cfg.Config.Server.UnixSocket = socketPath

	// Find a free port for the HTTP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	ln.Close() // Release it so the server can bind

	cfg.Config.API.ListenAddress = addr
	cfg.Config.Logging.Level = "debug"

	// Create an API key in the config before the server starts
	apiKeyStr, err := config.GenerateKey(32)
	require.NoError(t, err)
	err = cfg.AddAPIKey(config.APIKey{
		Key:       apiKeyStr,
		Name:      "test-key",
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	lightManager := &mockLightManager{
		lights: map[string]*keylight.Light{
			"test-light-1": {
				ID:          "test-light-1",
				Name:        "Test Light 1",
				Brightness:  50,
				Temperature: 5000,
				On:          true,
				LastSeen:    time.Now(),
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	server := New(logger, cfg, lightManager, VersionInfo{Version: "test", Commit: "abc1234", BuildDate: "2026-01-01T00:00:00Z"})

	return server, apiKeyStr, fmt.Sprintf("http://%s", addr)
}

// TestHTTPCommunication tests communication with the server over HTTP
func TestHTTPCommunication(t *testing.T) {
	server, apiKey, baseURL := setupHTTPIntegrationTest(t)

	err := server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	// Give server time to start HTTP listener
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("list lights with valid API key", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/lights", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var lights map[string]any
		require.NoError(t, json.Unmarshal(body, &lights))
		assert.Contains(t, lights, "test-light-1", "response should contain test light")
	})

	t.Run("list lights with X-API-Key header", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/lights", nil)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", apiKey)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("list lights without API key returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/lights", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("list lights with invalid API key returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/lights", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-key-1234")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("health endpoint is public", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/health", nil)
		require.NoError(t, err)
		// No API key

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("healthz probe endpoint is public", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/healthz", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("OpenAPI spec is public", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/openapi.json", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestHTTPSetLightState tests setting light state via HTTP
func TestHTTPSetLightState(t *testing.T) {
	server, apiKey, baseURL := setupHTTPIntegrationTest(t)

	err := server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("set brightness on existing light", func(t *testing.T) {
		body := `{"brightness": 75}`
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/lights/test-light-1/state", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]any
		require.NoError(t, json.Unmarshal(respBody, &result))
		assert.Equal(t, "ok", result["status"])
	})

	t.Run("set state on non-existent light returns error", func(t *testing.T) {
		body := `{"brightness": 50}`
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/lights/no-such-light/state", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return an error status (404 or 500 depending on handler)
		assert.True(t, resp.StatusCode >= 400, "expected error status code, got %d", resp.StatusCode)
	})

	t.Run("set multiple properties at once", func(t *testing.T) {
		body := `{"on": true, "brightness": 80}`
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/lights/test-light-1/state", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("set state without API key returns 401", func(t *testing.T) {
		body := `{"on": true}`
		req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/lights/test-light-1/state", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestConcurrentRequests tests handling multiple concurrent requests
func TestConcurrentRequests(t *testing.T) {
	server, _, socketPath := setupIntegrationTest(t)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Launch multiple concurrent socket connections
	const numRequests = 10
	errChan := make(chan error, numRequests)

	for range make([]struct{}, numRequests) {
		go func() {
			// Connect to socket
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				errChan <- err
				return
			}
			defer conn.Close()

			// Send a request to list lights
			req := map[string]string{
				"action": "list_lights",
			}
			err = json.NewEncoder(conn).Encode(req)
			if err != nil {
				errChan <- err
				return
			}

			// Read response
			var resp map[string]any
			err = json.NewDecoder(conn).Decode(&resp)
			if err != nil {
				errChan <- err
				return
			}

			// Signal success
			errChan <- nil
		}()
	}

	// Collect results
	for range make([]struct{}, numRequests) {
		select {
		case err := <-errChan:
			require.NoError(t, err, "Concurrent request should succeed")
		case <-ctx.Done():
			t.Fatal("Test timed out waiting for concurrent requests")
		}
	}
}

// TestServerShutdownGraceful verifies that Stop() terminates goroutines and prevents new connections.
func TestServerShutdownGraceful(t *testing.T) {
	server, _, socketPath := setupIntegrationTest(t)

	// Start server
	err := server.Start()
	require.NoError(t, err)
	// Give the server some time to start listener
	time.Sleep(50 * time.Millisecond)

	// Open a connection to ensure accept loop is active
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	_ = conn.Close()

	// Issue shutdown
	shutdownStart := time.Now()
	server.Stop()

	// After Stop(), the socket should be removed
	_, statErr := os.Stat(socketPath)
	require.True(t, os.IsNotExist(statErr), "socket should be removed after shutdown")

	// Attempt to connect again should fail quickly
	_, dialErr := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	require.Error(t, dialErr, "dial should fail after shutdown")

	// Shutdown should complete within a bounded time (sanity check)
	elapsed := time.Since(shutdownStart)
	if elapsed > 2*time.Second {
		t.Fatalf("shutdown exceeded expected time: %s", elapsed)
	}
}
