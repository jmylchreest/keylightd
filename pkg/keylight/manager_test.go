package keylight

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager := NewManager(logger)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.lights)
	assert.NotNil(t, manager.stopChan)
	assert.NotNil(t, manager.eventChan)
}

func TestLightManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager := NewManager(logger)

	// Test setting light state
	err := manager.SetLightState("non-existent", true)
	assert.ErrorIs(t, err, ErrLightNotFound)

	// Test setting light brightness (non-existent light)
	err = manager.SetLightBrightness("non-existent", 50)
	assert.ErrorIs(t, err, ErrLightNotFound)

	// Test setting invalid brightness (non-existent light, should still return ErrLightNotFound)
	err = manager.SetLightBrightness("test", 150)
	assert.ErrorIs(t, err, ErrLightNotFound)

	// Test setting light temperature (non-existent light)
	err = manager.SetLightTemperature("non-existent", 5000)
	assert.ErrorIs(t, err, ErrLightNotFound)

	// Test setting invalid temperature (non-existent light, should still return ErrLightNotFound)
	err = manager.SetLightTemperature("test", 1000)
	assert.ErrorIs(t, err, ErrLightNotFound)
}

func TestDiscovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	manager := NewManager(logger)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start discovery (should succeed)
	err := manager.StartDiscovery(ctx, time.Second)
	require.NoError(t, err)

	// Start discovery again (should fail)
	err = manager.StartDiscovery(ctx, time.Second)
	assert.Error(t, err)

	// Stop discovery (should succeed)
	err = manager.StopDiscovery()
	require.NoError(t, err)

	// Stop discovery again (should fail)
	err = manager.StopDiscovery()
	assert.Error(t, err)
}
