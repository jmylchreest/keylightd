package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Save original environment variables
	origInterval := os.Getenv("KEYLIGHTD_DISCOVERY_INTERVAL")
	defer os.Setenv("KEYLIGHTD_DISCOVERY_INTERVAL", origInterval)

	// Clear environment variable
	os.Unsetenv("KEYLIGHTD_DISCOVERY_INTERVAL")

	// Test loading with no config file
	config, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, getRuntimeSocketPath(), config.Server.UnixSocket)
	assert.Equal(t, 30, config.Discovery.Interval)
	assert.Equal(t, "info", config.Logging.Level)

	// Test loading with environment variables
	os.Setenv("KEYLIGHTD_DISCOVERY_INTERVAL", "60")

	config, err = Load()
	require.NoError(t, err)
	assert.Equal(t, 60, config.Discovery.Interval)
}

func TestSave(t *testing.T) {
	config := &Config{}
	config.Server.UnixSocket = getRuntimeSocketPath()
	config.Discovery.Interval = 30
	config.Logging.Level = "info"

	// Create temporary directory for config
	tmpDir, err := os.MkdirTemp("", "keylightd-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Set XDG_CONFIG_HOME to temporary directory
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", oldXDG)

	// Save config
	err = config.Save()
	require.NoError(t, err)

	// Verify config file exists
	configPath := filepath.Join(tmpDir, "keylightd", "keylightd.yaml")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load config and verify values
	loadedConfig, err := Load()
	require.NoError(t, err)
	assert.Equal(t, config.Server.UnixSocket, loadedConfig.Server.UnixSocket)
	assert.Equal(t, config.Discovery.Interval, loadedConfig.Discovery.Interval)
	assert.Equal(t, config.Logging.Level, loadedConfig.Logging.Level)
}
