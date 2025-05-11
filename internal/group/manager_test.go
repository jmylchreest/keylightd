package group

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLightManager struct {
	keylight.LightManager
	lights map[string]*keylight.Light
}

func (m *mockLightManager) GetLight(id string) (*keylight.Light, error) {
	light, exists := m.lights[id]
	if !exists {
		return nil, keylight.ErrLightNotFound
	}
	return light, nil
}

func (m *mockLightManager) SetLightState(id string, property string, value interface{}) error {
	_, exists := m.lights[id]
	if !exists {
		return keylight.ErrLightNotFound
	}
	return nil
}

func (m *mockLightManager) SetLightBrightness(id string, brightness int) error {
	return m.SetLightState(id, "brightness", brightness)
}

func (m *mockLightManager) SetLightTemperature(id string, temperature int) error {
	return m.SetLightState(id, "temperature", temperature)
}

func (m *mockLightManager) SetLightPower(id string, on bool) error {
	return m.SetLightState(id, "on", on)
}

func (m *mockLightManager) GetLights() map[string]*keylight.Light {
	return m.lights
}

func (m *mockLightManager) GetDiscoveredLights() []*keylight.Light {
	lights := make([]*keylight.Light, 0, len(m.lights))
	for _, light := range m.lights {
		lights = append(lights, light)
	}
	return lights
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

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{lights: make(map[string]*keylight.Light)}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.groups)
}

func TestGroupManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
			"light2": {ID: "light2", Name: "Light 2"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	// Test creating group
	group, err := manager.CreateGroup("test-group", []string{"light1", "light2"})
	require.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, "test-group", group.Name)
	assert.Len(t, group.Lights, 2)

	// Test creating group with non-existent light
	_, err = manager.CreateGroup("invalid-group", []string{"non-existent"})
	assert.Error(t, err)

	// Test getting group
	retrieved, err := manager.GetGroup(group.ID)
	require.NoError(t, err)
	assert.Equal(t, group, retrieved)

	// Test getting non-existent group
	_, err = manager.GetGroup("non-existent")
	assert.Error(t, err)

	// Test getting all groups
	groups := manager.GetGroups()
	assert.Len(t, groups, 1)
	assert.Equal(t, group, groups[0])

	// Test deleting group
	err = manager.DeleteGroup(group.ID)
	require.NoError(t, err)

	// Test deleting non-existent group
	err = manager.DeleteGroup("non-existent")
	assert.Error(t, err)
}

func TestGroupOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
			"light2": {ID: "light2", Name: "Light 2"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	// Create a group
	group, err := manager.CreateGroup("test-group", []string{"light1", "light2"})
	require.NoError(t, err)

	// Test setting group state
	err = manager.SetGroupState(group.ID, true)
	require.NoError(t, err)

	// Test setting group brightness
	err = manager.SetGroupBrightness(group.ID, 50)
	require.NoError(t, err)

	// Test setting group temperature
	err = manager.SetGroupTemperature(group.ID, 5000)
	require.NoError(t, err)

	// Test operations on non-existent group
	err = manager.SetGroupState("non-existent", true)
	assert.Error(t, err)

	err = manager.SetGroupBrightness("non-existent", 50)
	assert.Error(t, err)

	err = manager.SetGroupTemperature("non-existent", 5000)
	assert.Error(t, err)
}
