package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLightManager struct {
	keylight.LightManager
}

func setupTestConfig(t *testing.T) *config.Config {
	// Create temporary directory for config
	tmpDir, err := os.MkdirTemp("", "keylightd-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Set XDG_CONFIG_HOME to temporary directory
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", oldXDG) })

	// Create config
	cfg, err := config.Load("test.yaml", "")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	return cfg
}

func TestNewServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{}
	cfg := setupTestConfig(t)
	server := New(logger, lights, cfg)
	assert.NotNil(t, server)
	assert.Equal(t, lights, server.lights)
	assert.Equal(t, cfg, server.cfg)
}

func TestServerStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{}
	cfg := setupTestConfig(t)
	server := New(logger, lights, cfg)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Test connection
	conn, err := net.Dial("unix", cfg.Server.UnixSocket)
	require.NoError(t, err)
	conn.Close()

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Stop(ctx)
	require.NoError(t, err)

	// Verify socket is removed
	_, err = os.Stat(cfg.Server.UnixSocket)
	assert.True(t, os.IsNotExist(err))
}
