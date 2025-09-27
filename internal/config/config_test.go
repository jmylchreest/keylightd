package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadDefaults_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	cfg, err := Load("test.yaml", configPath)
	require.NoError(t, err)
	assert.Equal(t, 30, cfg.Config.Discovery.Interval)
	assert.Equal(t, ":9123", cfg.Config.API.ListenAddress)
}

func TestAPIKeyDisabledPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial load (will use defaults, create in-memory config)
	cfg, err := Load("config.yaml", configPath)
	require.NoError(t, err)

	// Add an API key
	key := APIKey{
		Key:  "testkey123456",
		Name: "test",
	}
	require.NoError(t, cfg.AddAPIKey(key))

	// Disable it via the provided method
	_, err = cfg.SetAPIKeyDisabledStatus("test", true) // using name
	require.NoError(t, err)

	// Save to disk
	require.NoError(t, cfg.Save())

	// Reload from disk
	cfgReloaded, err := Load("config.yaml", configPath)
	require.NoError(t, err)

	// Find by key string
	reloadedKey, found := cfgReloaded.FindAPIKey("testkey123456")
	require.True(t, found, "expected to find API key after reload")
	assert.True(t, reloadedKey.IsDisabled(), "expected API key to remain disabled after reload")
}

func TestSaveAndLoadConfig_WithTimeFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	// Create config and set a time field
	v := viper.New()
	v.SetConfigFile(configPath)
	cfg := New(v)
	now := time.Now().UTC().Truncate(time.Second)
	cfg.State.APIKeys = []APIKey{
		{
			Key:       "abc123",
			Name:      "test",
			CreatedAt: now,
			ExpiresAt: now.Add(24 * time.Hour),
		},
	}

	// Save config
	require.NoError(t, cfg.Save())

	// Load config again using yaml.Unmarshal (not Viper)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var loaded Config
	require.NoError(t, yaml.Unmarshal(data, &loaded))

	require.Len(t, loaded.State.APIKeys, 1)
	key := loaded.State.APIKeys[0]
	assert.Equal(t, "abc123", key.Key)
	assert.Equal(t, "test", key.Name)
	assert.WithinDuration(t, now, key.CreatedAt, time.Second)
	assert.WithinDuration(t, now.Add(24*time.Hour), key.ExpiresAt, time.Second)
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("not: [valid: yaml"), 0644))

	_, err := Load("bad.yaml", configPath)
	assert.Error(t, err)
}
