package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	cfg, err := Load("test.yaml", configPath)
	require.NoError(t, err)
	assert.Equal(t, 30, cfg.Discovery.Interval)
	assert.Equal(t, ":9123", cfg.API.ListenAddress)
}

func TestSaveAndLoadConfig_WithTimeFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	// Create config and set a time field
	v := viper.New()
	v.SetConfigFile(configPath)
	cfg := New(v)
	now := time.Now().UTC().Truncate(time.Second)
	cfg.API.APIKeys = []APIKey{
		{
			Key:       "abc123",
			Name:      "test",
			CreatedAt: now,
			ExpiresAt: now.Add(24 * time.Hour),
		},
	}

	// Save config
	require.NoError(t, cfg.Save())

	// Load config again
	cfg2, err := Load("test.yaml", configPath)
	require.NoError(t, err)
	require.Len(t, cfg2.API.APIKeys, 1)
	key := cfg2.API.APIKeys[0]
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
