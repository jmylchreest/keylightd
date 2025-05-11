package keylight

import (
	"fmt"
	"log/slog"
	"sync"
)

// Manager manages Key Light devices
type Manager struct {
	lights  map[string]Light
	clients map[string]*KeyLightClient
	mu      sync.RWMutex
	logger  *slog.Logger
}

// NewManager creates a new manager
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		lights:  make(map[string]Light),
		clients: make(map[string]*KeyLightClient),
		logger:  logger,
	}
}

// GetDiscoveredLights returns all discovered lights
func (m *Manager) GetDiscoveredLights() []*Light {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lights := make([]*Light, 0, len(m.lights))
	for id := range m.lights {
		light := m.lights[id]
		lights = append(lights, &light)
	}
	return lights
}

// GetLight returns a light by ID
func (m *Manager) GetLight(id string) (*Light, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	light, exists := m.lights[id]
	if !exists {
		return nil, fmt.Errorf("light %s not found", id)
	}

	// Get or create client for this light
	client, exists := m.clients[id]
	if !exists {
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
	}

	// Get accessory info if not already set
	if light.ProductName == "" {
		info, err := client.GetAccessoryInfo()
		if err != nil {
			return nil, fmt.Errorf("failed to get accessory info: %w", err)
		}
		light.ProductName = info.ProductName
		light.HardwareBoardType = info.HardwareBoardType
		light.FirmwareVersion = info.FirmwareVersion
		light.FirmwareBuild = info.FirmwareBuildNumber
		light.SerialNumber = info.SerialNumber
	}

	// Get current state
	state, err := client.GetLightState()
	if err != nil {
		return nil, fmt.Errorf("failed to get current state: %w", err)
	}

	// Update local state
	light.State = state
	light.Temperature = state.Lights[0].Temperature
	light.Brightness = state.Lights[0].Brightness
	light.On = state.Lights[0].On == 1
	m.lights[id] = light

	return &light, nil
}

// SetLightState sets the state of a light
func (m *Manager) SetLightState(id string, property string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	light, exists := m.lights[id]
	if !exists {
		return fmt.Errorf("light %s not found", id)
	}

	// Get or create client for this light
	client, exists := m.clients[id]
	if !exists {
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
	}

	// Get current state
	state, err := client.GetLightState()
	if err != nil {
		return fmt.Errorf("failed to get current state: %w", err)
	}

	// Update state based on property
	switch property {
	case "on":
		on, ok := value.(bool)
		if !ok {
			return fmt.Errorf("invalid value type for on: %T", value)
		}
		state.Lights[0].On = boolToInt(on)
	case "brightness":
		brightness, ok := value.(int)
		if !ok {
			return fmt.Errorf("invalid value type for brightness: %T", value)
		}
		// Clamp to valid range (3-100)
		if brightness < 3 {
			brightness = 3
		} else if brightness > 100 {
			brightness = 100
		}
		state.Lights[0].Brightness = brightness
	case "temperature":
		temp, ok := value.(int)
		if !ok {
			return fmt.Errorf("invalid value type for temperature: %T", value)
		}
		// Convert from Kelvin to mireds
		state.Lights[0].Temperature = convertTemperatureToDevice(temp)
	default:
		return fmt.Errorf("unknown property: %s", property)
	}

	// Send updated state to device
	if err := client.SetLightState(
		state.Lights[0].On == 1,
		state.Lights[0].Brightness,
		state.Lights[0].Temperature,
	); err != nil {
		return fmt.Errorf("failed to set light state: %w", err)
	}

	// Update local state
	light.State = state
	m.lights[id] = light

	return nil
}

// SetLightBrightness sets the brightness of a light
func (m *Manager) SetLightBrightness(id string, brightness int) error {
	return m.SetLightState(id, "brightness", brightness)
}

// SetLightTemperature sets the temperature of a light
func (m *Manager) SetLightTemperature(id string, temperature int) error {
	return m.SetLightState(id, "temperature", temperature)
}

// SetLightPower sets the power state of a light
func (m *Manager) SetLightPower(id string, on bool) error {
	return m.SetLightState(id, "on", on)
}

// GetLights returns all discovered lights
func (m *Manager) GetLights() map[string]*Light {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the map to avoid concurrent access issues
	lights := make(map[string]*Light)
	for id, light := range m.lights {
		lightCopy := light // Create a copy to avoid pointer issues
		lights[id] = &lightCopy
	}

	return lights
}

// AddLight adds a light to the manager
func (m *Manager) AddLight(light Light) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client := NewKeyLightClient(light.IP.String(), light.Port, m.logger)
	m.clients[light.ID] = client

	// Only fetch accessory info if required fields are missing
	if light.ProductName == "" || light.SerialNumber == "" {
		info, err := client.GetAccessoryInfo()
		if err != nil {
			m.logger.Error("failed to get accessory info", slog.String("id", light.ID), slog.Any("error", err))
		} else {
			light.ProductName = info.ProductName
			light.HardwareBoardType = info.HardwareBoardType
			light.FirmwareVersion = info.FirmwareVersion
			light.FirmwareBuild = info.FirmwareBuildNumber
			light.SerialNumber = info.SerialNumber
			light.Name = info.DisplayName
		}
	}

	// Get current state
	state, err := client.GetLightState()
	if err != nil {
		m.logger.Error("failed to get light state", slog.String("id", light.ID), slog.Any("error", err))
	} else {
		light.State = state
	}

	m.lights[light.ID] = light
	m.logger.Info("added light", slog.String("id", light.ID), slog.String("ip", light.IP.String()), slog.Int("port", light.Port))
}
