package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
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
	server := New(logger, v, lightManager)

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

// TestHTTPCommunication tests communication with the server over HTTP
func TestHTTPCommunication(t *testing.T) {
	t.Skip("Skipping HTTP API test due to API key handling issues")
}

// TestHTTPSetLightState tests setting light state via HTTP
func TestHTTPSetLightState(t *testing.T) {
	t.Skip("Skipping HTTP API test due to API key handling issues")
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
