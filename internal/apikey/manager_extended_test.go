package apikey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteAPIKey_Success(t *testing.T) {
	mgr, _ := newTestManager(t)

	created, err := mgr.CreateAPIKey("to-delete", 0)
	require.NoError(t, err)

	err = mgr.DeleteAPIKey(created.Key)
	require.NoError(t, err)

	// Key should no longer validate
	_, err = mgr.ValidateAPIKey(created.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteAPIKey_NotFound(t *testing.T) {
	mgr, _ := newTestManager(t)

	err := mgr.DeleteAPIKey("nonexistent-key-12345678")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteAPIKey_ThenRecreate(t *testing.T) {
	mgr, _ := newTestManager(t)

	created, err := mgr.CreateAPIKey("recreate-test", 0)
	require.NoError(t, err)

	err = mgr.DeleteAPIKey(created.Key)
	require.NoError(t, err)

	// Should be able to recreate with the same name
	created2, err := mgr.CreateAPIKey("recreate-test", 0)
	require.NoError(t, err)
	assert.NotEqual(t, created.Key, created2.Key, "new key should have a different value")
}

func TestListAPIKeys_Empty(t *testing.T) {
	mgr, _ := newTestManager(t)

	keys := mgr.ListAPIKeys()
	assert.Empty(t, keys)
}

func TestListAPIKeys_ReturnsAll(t *testing.T) {
	mgr, _ := newTestManager(t)

	_, err := mgr.CreateAPIKey("key-a", 0)
	require.NoError(t, err)
	_, err = mgr.CreateAPIKey("key-b", 0)
	require.NoError(t, err)
	_, err = mgr.CreateAPIKey("key-c", 0)
	require.NoError(t, err)

	keys := mgr.ListAPIKeys()
	assert.Len(t, keys, 3)

	names := make(map[string]bool)
	for _, k := range keys {
		names[k.Name] = true
	}
	assert.True(t, names["key-a"])
	assert.True(t, names["key-b"])
	assert.True(t, names["key-c"])
}

func TestListAPIKeys_AfterDeletion(t *testing.T) {
	mgr, _ := newTestManager(t)

	key1, err := mgr.CreateAPIKey("keep", 0)
	require.NoError(t, err)
	key2, err := mgr.CreateAPIKey("delete-me", 0)
	require.NoError(t, err)

	_ = key1 // keep this one
	err = mgr.DeleteAPIKey(key2.Key)
	require.NoError(t, err)

	keys := mgr.ListAPIKeys()
	assert.Len(t, keys, 1)
	assert.Equal(t, "keep", keys[0].Name)
}

func TestCreateAPIKey_DuplicateName(t *testing.T) {
	mgr, _ := newTestManager(t)

	_, err := mgr.CreateAPIKey("dup", 0)
	require.NoError(t, err)

	_, err = mgr.CreateAPIKey("dup", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
