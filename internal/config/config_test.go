package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Save original environment variable
	oldInterval := os.Getenv("KEYLIGHT_DISCOVERY_INTERVAL")
	defer os.Setenv("KEYLIGHT_DISCOVERY_INTERVAL", oldInterval)

	// Test loading with default values
	cfg, err := Load("test.yaml", "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 30, cfg.Discovery.Interval)

	// Test loading with environment variable
	os.Setenv("KEYLIGHT_DISCOVERY_INTERVAL", "60")
	cfg, err = Load("test.yaml", "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 60, cfg.Discovery.Interval)
}

func TestSave(t *testing.T) {
	// Create temporary directory for config
	tmpDir, err := os.MkdirTemp("", "keylightd-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Set XDG_CONFIG_HOME to temporary directory
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", oldXDG)

	// Create config
	cfg, err := Load("test.yaml", "")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Save config
	err = cfg.Save("test.yaml")
	require.NoError(t, err)

	// Load config again to verify
	cfg2, err := Load("test.yaml", "")
	require.NoError(t, err)
	assert.Equal(t, cfg.Discovery.Interval, cfg2.Discovery.Interval)
}
