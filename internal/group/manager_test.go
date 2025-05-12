package group

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/viper"
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

	// Create config file path
	configPath := filepath.Join(tmpDir, "test.yaml")

	// Create config
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(configPath)
	v.SetDefault("config.server.unix_socket", filepath.Join(tmpDir, "keylightd.sock"))
	v.SetDefault("config.discovery.interval", 30)
	v.SetDefault("config.logging.level", "info")
	v.SetDefault("config.logging.format", "text")
	v.SetDefault("config.discovery.cleanup_interval", 60)
	v.SetDefault("config.discovery.cleanup_timeout", 180)
	v.SetDefault("config.api.listen_address", ":9123")
	v.SetDefault("state.api_keys", []config.APIKey{})

	// Create and save initial config
	cfg := config.New(v)
	err = cfg.Save()
	require.NoError(t, err)

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

func TestGroupLightsJSONAlwaysArray(t *testing.T) {
	cases := []struct {
		name  string
		group Group
		want  string
	}{
		{
			name:  "non-empty lights",
			group: Group{ID: "g1", Name: "Test", Lights: []string{"a", "b"}},
			want:  `{"id":"g1","name":"Test","lights":["a","b"]}`,
		},
		{
			name:  "empty lights",
			group: Group{ID: "g2", Name: "Empty", Lights: []string{}},
			want:  `{"id":"g2","name":"Empty","lights":[]}`,
		},
		{
			name:  "nil lights",
			group: Group{ID: "g3", Name: "Nil", Lights: nil},
			want:  `{"id":"g3","name":"Nil","lights":[]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(&tc.group)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if string(b) != tc.want {
				t.Errorf("got %s, want %s", string(b), tc.want)
			}
		})
	}
}

func TestGetGroupsByKeys_MultiGroupAndByName(t *testing.T) {
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

	// Create groups with same name and different names
	g1, err := manager.CreateGroup("office", []string{"light1"})
	require.NoError(t, err)
	g2, err := manager.CreateGroup("office", []string{"light2"})
	require.NoError(t, err)
	g3, err := manager.CreateGroup("studio", []string{"light1", "light2"})
	require.NoError(t, err)

	// Test by ID
	groups, notFound := manager.GetGroupsByKeys(g1.ID)
	assert.Len(t, groups, 1)
	assert.Equal(t, g1.ID, groups[0].ID)
	assert.Empty(t, notFound)

	// Test by name (multiple groups)
	groups, notFound = manager.GetGroupsByKeys("office")
	assert.Len(t, groups, 2)
	ids := []string{groups[0].ID, groups[1].ID}
	assert.Contains(t, ids, g1.ID)
	assert.Contains(t, ids, g2.ID)
	assert.Empty(t, notFound)

	// Test by comma-separated IDs/names (deduplication)
	keyStr := g1.ID + ",office,studio,notfound"
	groups, notFound = manager.GetGroupsByKeys(keyStr)
	assert.Len(t, groups, 3) // g1, g2, g3 (g1 only once)
	ids = []string{groups[0].ID, groups[1].ID, groups[2].ID}
	assert.Contains(t, ids, g1.ID)
	assert.Contains(t, ids, g2.ID)
	assert.Contains(t, ids, g3.ID)
	assert.Equal(t, []string{"notfound"}, notFound)
}
