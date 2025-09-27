package apikey

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestManager creates a Manager with a temp config file.
func newTestManager(t *testing.T) (*Manager, *config.Config) {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := config.Load("config.yaml", cfgPath)
	require.NoError(t, err, "failed to load initial config")

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(cfg, logger)
	return mgr, cfg
}

func TestValidateAPIKey_DisabledRejected(t *testing.T) {
	mgr, cfg := newTestManager(t)

	created, err := mgr.CreateAPIKey("disabled-test", 0)
	require.NoError(t, err, "failed to create API key for test")

	// Disable by name (manager persists)
	_, err = mgr.SetAPIKeyDisabledStatus("disabled-test", true)
	require.NoError(t, err, "failed to disable API key")

	// Ensure disabled persisted in config state
	k, found := cfg.FindAPIKey(created.Key)
	require.True(t, found, "expected to find key in config after disable")
	assert.True(t, k.Disabled, "expected key to be disabled")

	// Attempt validation - should fail
	_, err = mgr.ValidateAPIKey(created.Key)
	require.Error(t, err, "expected validation to fail for disabled key")
	assert.True(t, strings.Contains(err.Error(), "disabled"), "error should mention disabled")

	// LastUsedAt should remain zero value after failed validation
	assert.True(t, k.LastUsedAt.IsZero(), "LastUsedAt should not be updated on disabled key validation attempt")

	// Re-enable
	_, err = mgr.SetAPIKeyDisabledStatus("disabled-test", false)
	require.NoError(t, err, "failed to re-enable API key")

	// Validate again - should succeed and update LastUsedAt
	validated, err := mgr.ValidateAPIKey(created.Key)
	require.NoError(t, err, "expected validation to succeed after re-enabling")
	assert.False(t, validated.LastUsedAt.IsZero(), "expected LastUsedAt to be set after successful validation")
}

func TestValidateAPIKey_Expiration(t *testing.T) {
	mgr, _ := newTestManager(t)

	created, err := mgr.CreateAPIKey("expiring", 50*time.Millisecond)
	require.NoError(t, err, "failed to create expiring key")

	// Immediately valid
	_, err = mgr.ValidateAPIKey(created.Key)
	require.NoError(t, err, "expected key to be valid before expiration")

	// Wait for expiration
	time.Sleep(75 * time.Millisecond)

	_, err = mgr.ValidateAPIKey(created.Key)
	require.Error(t, err, "expected key to be expired")
	assert.True(t, strings.Contains(err.Error(), "expired"), "error should mention expired")
}

func TestValidateAPIKey_UpdatesLastUsed(t *testing.T) {
	mgr, _ := newTestManager(t)

	created, err := mgr.CreateAPIKey("usage", 0)
	require.NoError(t, err, "failed to create key")

	// First validation
	valid1, err := mgr.ValidateAPIKey(created.Key)
	require.NoError(t, err, "first validation failed unexpectedly")
	firstUsed := valid1.LastUsedAt
	assert.False(t, firstUsed.IsZero(), "first validation should set LastUsedAt")

	// Sleep to ensure timestamp difference (resolution)
	time.Sleep(10 * time.Millisecond)

	// Second validation
	valid2, err := mgr.ValidateAPIKey(created.Key)
	require.NoError(t, err, "second validation failed unexpectedly")
	secondUsed := valid2.LastUsedAt

	assert.True(t, secondUsed.After(firstUsed) || secondUsed.Equal(firstUsed),
		"second LastUsedAt should be >= first (monotonic). first=%s second=%s", firstUsed, secondUsed)
}

func TestSetAPIKeyDisabledStatus_ByNameAndKey(t *testing.T) {
	mgr, cfg := newTestManager(t)

	created, err := mgr.CreateAPIKey("dual", 0)
	require.NoError(t, err)

	// Disable via name
	updated, err := mgr.SetAPIKeyDisabledStatus("dual", true)
	require.NoError(t, err)
	assert.True(t, updated.Disabled)

	// Disable via key string (idempotent set true again)
	updated2, err := mgr.SetAPIKeyDisabledStatus(created.Key, true)
	require.NoError(t, err)
	assert.True(t, updated2.Disabled)

	// Enable via key string
	updated3, err := mgr.SetAPIKeyDisabledStatus(created.Key, false)
	require.NoError(t, err)
	assert.False(t, updated3.Disabled)

	// Confirm persisted state
	reloaded, found := cfg.FindAPIKey(created.Key)
	require.True(t, found)
	assert.False(t, reloaded.Disabled)
}

func TestValidateAPIKey_NotFound(t *testing.T) {
	mgr, _ := newTestManager(t)

	_, err := mgr.ValidateAPIKey("nope")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not found"))
}
