package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLightManager struct {
	keylight.LightManager
}

func TestNewServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager := &mockLightManager{}
	config := &Config{
		UnixSocket: "/tmp/test.sock",
		APIKeys:    []string{"test-key"},
		AllowLocal: true,
	}

	server := New(logger, manager, config)
	assert.NotNil(t, server)
	assert.Equal(t, logger, server.logger)
	assert.Equal(t, manager, server.manager)
	assert.Equal(t, config, server.config)
}

func TestServerStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager := &mockLightManager{}
	config := &Config{
		UnixSocket: "/tmp/test.sock",
		APIKeys:    []string{"test-key"},
		AllowLocal: true,
	}

	server := New(logger, manager, config)
	require.NotNil(t, server)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test connection
	conn, err := net.Dial("unix", config.UnixSocket)
	require.NoError(t, err)
	conn.Close()

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(ctx)
	require.NoError(t, err)

	// Clean up
	os.Remove(config.UnixSocket)
}
