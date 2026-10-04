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

func TestSaveDoesNotFollowPredictableTempSymlink(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	victimPath := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victimPath, []byte("unchanged"), 0600))
	require.NoError(t, os.Symlink(victimPath, configPath+".tmp"))
	v := viper.New()
	v.SetConfigFile(configPath)
	require.NoError(t, New(v).Save())
	data, err := os.ReadFile(victimPath)
	require.NoError(t, err)
	assert.Equal(t, "unchanged", string(data))
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
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

func TestGenerateKeyLengthAndCharset(t *testing.T) {
	for _, length := range []int{-1, 0, 1, 32, 257} {
		key, err := GenerateKey(length)
		require.NoError(t, err)
		wantLength := length
		if wantLength <= 0 {
			wantLength = DefaultKeyLength
		}
		require.Len(t, key, wantLength)
		for _, char := range key {
			assert.Contains(t, DefaultKeyCharset, string(char))
		}
	}
}

func TestAPIKeyExpiration(t *testing.T) {
	for _, tt := range []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"never", time.Time{}, false},
		{"expired", time.Now().Add(-time.Hour), true},
		{"valid", time.Now().Add(time.Hour), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			key := APIKey{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.want, key.IsExpired())
		})
	}
}

func TestAPIKeyLifecycle(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := Load("config", configPath)
	require.NoError(t, err)
	first := APIKey{Key: "first-secret", Name: "first"}
	second := APIKey{Key: "second-secret", Name: "second"}
	cfg.SetAPIKeys([]APIKey{first})
	require.NoError(t, cfg.AddAPIKey(second))
	require.Error(t, cfg.AddAPIKey(APIKey{Key: first.Key, Name: "different"}))
	require.Error(t, cfg.AddAPIKey(APIKey{Key: "different-secret", Name: first.Name}))
	keys := cfg.GetAPIKeys()
	require.Len(t, keys, 2)
	keys[0].Disabled = true
	assert.False(t, cfg.GetAPIKeys()[0].Disabled, "returned keys must not mutate config")
	lastUsed := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, cfg.UpdateAPIKeyLastUsed(second.Key, lastUsed))
	updated, found := cfg.FindAPIKey(second.Key)
	require.True(t, found)
	assert.Equal(t, lastUsed, updated.LastUsedAt)
	require.Error(t, cfg.UpdateAPIKeyLastUsed("missing", lastUsed))
	assert.False(t, cfg.DeleteAPIKey("missing"))
	assert.True(t, cfg.DeleteAPIKey(first.Key))
	_, found = cfg.FindAPIKey(first.Key)
	assert.False(t, found)
	require.NoError(t, cfg.Save())
	reloaded, err := Load("config", configPath)
	require.NoError(t, err)
	require.Equal(t, []APIKey{*updated}, reloaded.GetAPIKeys())
	assert.True(t, cfg.DeleteAPIKey(second.Key))
	assert.Empty(t, cfg.GetAPIKeys())
}

func TestSaveFailurePreservesConfigAndCleansTempFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	// A directory at the destination forces the atomic rename to fail.
	require.NoError(t, os.Mkdir(configPath, 0700))
	v := viper.New()
	v.SetConfigFile(configPath)
	require.ErrorContains(t, New(v).Save(), "error replacing config file")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary config file must be removed on failure")
	assert.Equal(t, "config.yaml", entries[0].Name())
	assert.True(t, entries[0].IsDir())
}

func TestSaveWithoutConfigPath(t *testing.T) {
	require.ErrorContains(t, New(viper.New()).Save(), "no config file path set")
}
