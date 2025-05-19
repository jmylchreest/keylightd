package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLightManager struct {
	lights map[string]*keylight.Light
}

func (m *mockLightManager) AddLight(light keylight.Light) {
	if m.lights == nil {
		m.lights = make(map[string]*keylight.Light)
	}
	m.lights[light.ID] = &light
}

func (m *mockLightManager) RemoveLight(id string) {
	delete(m.lights, id)
}

func (m *mockLightManager) GetLight(id string) (*keylight.Light, error) {
	light, ok := m.lights[id]
	if !ok {
		return nil, fmt.Errorf("light %s not found", id)
	}
	return light, nil
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

func (m *mockLightManager) SetLightBrightness(id string, brightness int) error {
	light, err := m.GetLight(id)
	if err != nil {
		return err
	}
	light.Brightness = brightness
	return nil
}

func (m *mockLightManager) SetLightTemperature(id string, temperature int) error {
	light, err := m.GetLight(id)
	if err != nil {
		return err
	}
	light.Temperature = temperature
	return nil
}

func (m *mockLightManager) SetLightPower(id string, on bool) error {
	light, err := m.GetLight(id)
	if err != nil {
		return err
	}
	light.On = on
	return nil
}

func (m *mockLightManager) SetLightState(id string, property string, value any) error {
	light, err := m.GetLight(id)
	if err != nil {
		return err
	}

	switch property {
	case "on":
		on, ok := value.(bool)
		if !ok {
			return fmt.Errorf("invalid value type for on: %T", value)
		}
		light.On = on
	case "brightness":
		brightness, ok := value.(int)
		if !ok {
			return fmt.Errorf("invalid value type for brightness: %T", value)
		}
		light.Brightness = brightness
	case "temperature":
		temp, ok := value.(int)
		if !ok {
			return fmt.Errorf("invalid value type for temperature: %T", value)
		}
		light.Temperature = temp
	default:
		return fmt.Errorf("unknown property: %s", property)
	}

	return nil
}

func (m *mockLightManager) StartCleanupWorker(ctx context.Context, cleanupInterval time.Duration, timeout time.Duration) {
	// No-op for mock implementation
}

func setupTestConfig(t *testing.T) *config.Config {
	// Create config
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetDefault("config.server.unix_socket", "/tmp/keylightd.sock")
	v.SetDefault("config.discovery.interval", 30)
	v.SetDefault("config.logging.level", "info")
	v.SetDefault("config.logging.format", "text")
	v.SetDefault("config.discovery.cleanup_interval", 60)
	v.SetDefault("config.discovery.cleanup_timeout", 180)
	v.SetDefault("config.api.listen_address", ":9123")
	v.SetDefault("state.api_keys", []config.APIKey{})

	cfg := config.New(v)
	err := v.Unmarshal(cfg)
	require.NoError(t, err)

	// Set Unix socket path to a temporary file
	tmpDir, err := os.MkdirTemp("", "keylightd-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	socketPath := filepath.Join(tmpDir, "keylightd.sock")
	cfg.Config.Server.UnixSocket = socketPath

	return cfg
}

func TestNewServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{lights: make(map[string]*keylight.Light)}
	cfg := setupTestConfig(t)
	server := New(logger, cfg, lights)
	assert.NotNil(t, server)
	assert.Equal(t, lights, server.lights)
	assert.Equal(t, cfg, server.cfg)
}

func TestServerStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	lights := &mockLightManager{lights: make(map[string]*keylight.Light)}
	cfg := setupTestConfig(t)
	server := New(logger, cfg, lights)

	// Start server
	err := server.Start()
	require.NoError(t, err)

	// Test connection
	conn, err := net.Dial("unix", cfg.Config.Server.UnixSocket)
	require.NoError(t, err)
	conn.Close()

	// Stop server
	server.Stop()

	// Verify socket is removed
	_, err = os.Stat(cfg.Config.Server.UnixSocket)
	assert.True(t, os.IsNotExist(err))
}
