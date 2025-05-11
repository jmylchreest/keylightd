package keylight

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
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

// GetLight returns a light by ID and updates its state
func (m *Manager) GetLight(id string) (*Light, error) {
	// Use RLock to safely read the light and client initially
	m.mu.RLock()
	light, exists := m.lights[id]
	client, clientExists := m.clients[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("light %s not found", id)
	}

	// If client doesn't exist (shouldn't happen after AddLight, but as a safeguard)
	// create one. This might indicate a logic error elsewhere if it occurs often.
	if !clientExists {
		// Although creating the client is not blocking, acquiring a lock here briefly is fine.
		m.mu.Lock()
		// Double check exists after re-locking
		light, exists = m.lights[id] // Re-read light in case it was removed while unlocked
		if !exists {
			m.mu.Unlock()
			return nil, fmt.Errorf("light %s not found", id)
		}
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
		m.mu.Unlock()
	}

	var updatedLight Light = light // Work on a copy

	// Get accessory info if not already set - happens OUTSIDE the lock
	if updatedLight.ProductName == "" || updatedLight.SerialNumber == "" {
		info, err := client.GetAccessoryInfo()
		if err != nil {
			m.logger.Error("failed to get accessory info during GetLight", slog.String("id", id), slog.Any("error", err))
			// Continue without accessory info if fetching fails
		} else {
			updatedLight.ProductName = info.ProductName
			updatedLight.HardwareBoardType = info.HardwareBoardType
			updatedLight.FirmwareVersion = info.FirmwareVersion
			updatedLight.FirmwareBuild = info.FirmwareBuildNumber
			updatedLight.SerialNumber = info.SerialNumber
			updatedLight.Name = info.DisplayName // Update name based on display name
		}
	}

	// Get current state - happens OUTSIDE the lock
	state, err := client.GetLightState()
	if err != nil {
		// Log the error but we might still return the potentially stale light info
		m.logger.Error("failed to get current state during GetLight", slog.String("id", id), slog.Any("error", err))
		// Decide if you want to return an error here or just return the potentially stale light data
		// For now, returning error if state cannot be fetched
		return &updatedLight, fmt.Errorf("failed to get current state: %w", err)
	}

	// Update local state - acquire write lock briefly
	m.mu.Lock()
	// Re-read light under lock just in case it was updated by another goroutine between RUnlock and Lock
	if l, ok := m.lights[id]; ok {
		// Update only the state fields
		l.State = state
		l.Temperature = state.Lights[0].Temperature
		l.Brightness = state.Lights[0].Brightness
		l.On = state.Lights[0].On == 1

		// Merge updated info fields from the copy back into the stored light
		l.ProductName = updatedLight.ProductName
		l.HardwareBoardType = updatedLight.HardwareBoardType
		l.FirmwareVersion = updatedLight.FirmwareVersion
		l.FirmwareBuild = updatedLight.FirmwareBuild
		l.SerialNumber = updatedLight.SerialNumber
		l.Name = updatedLight.Name

		// Update LastSeen on successful communication
		l.LastSeen = time.Now()

		m.lights[id] = l // Store the updated light back
		updatedLight = l // Ensure the returned light has the latest combined data
	} else {
		// Light was removed while we were fetching data. Return not found error.
		m.mu.Unlock()
		return nil, fmt.Errorf("light %s not found during update after fetch", id)
	}
	m.mu.Unlock()

	return &updatedLight, nil
}

// SetLightState sets the state of a light
// It fetches the current state, updates the specified property, and sends the new state to the device.
func (m *Manager) SetLightState(id string, property string, value interface{}) error {
	// Use RLock to safely read the light and client initially
	m.mu.RLock()
	light, exists := m.lights[id]
	client, clientExists := m.clients[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("light %s not found", id)
	}

	// If client doesn't exist (shouldn't happen after AddLight, but as a safeguard)
	// create one. This might indicate a logic error elsewhere if it occurs often.
	// Note: Creating a client is not blocking, so a brief lock here is acceptable.
	if !clientExists {
		m.mu.Lock()
		// Double check exists after re-locking
		light, exists = m.lights[id] // Re-read light in case it was removed while unlocked
		if !exists {
			m.mu.Unlock()
			return fmt.Errorf("light %s not found", id)
		}
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
		m.mu.Unlock()
	}

	// Get current state from the device - happens OUTSIDE the lock
	state, err := client.GetLightState()
	if err != nil {
		return fmt.Errorf("failed to get current state before setting: %w", err)
	}

	// Update state based on property - happens OUTSIDE the lock
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

	// Send updated state to device - happens OUTSIDE the lock
	if err := client.SetLightState(
		state.Lights[0].On == 1,
		state.Lights[0].Brightness,
		state.Lights[0].Temperature,
	); err != nil {
		return fmt.Errorf("failed to send updated state: %w", err)
	}

	// Update local state in the manager - acquire write lock briefly
	m.mu.Lock()
	// Re-read light under lock just in case it was updated/removed by another goroutine
	if l, ok := m.lights[id]; ok {
		// Update only the state fields and LastSeen
		l.State = state
		l.Temperature = state.Lights[0].Temperature
		l.Brightness = state.Lights[0].Brightness
		l.On = state.Lights[0].On == 1
		l.LastSeen = time.Now() // Update LastSeen on successful state set

		m.lights[id] = l // Store the updated light back
	} else {
		// Light was removed while we were setting state. Log and continue, or return error.
		m.mu.Unlock()
		return fmt.Errorf("light %s removed during state update", id)
	}
	m.mu.Unlock()

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

// AddLight adds a light to the manager and fetches its initial state.
func (m *Manager) AddLight(light Light) {
	// Create client for this light - not blocking, can be done before lock
	client := NewKeyLightClient(light.IP.String(), light.Port, m.logger)

	// Get current state - happens OUTSIDE the lock
	state, err := client.GetLightState()
	if err != nil {
		m.logger.Error("failed to get initial light state during AddLight", slog.String("id", light.ID), slog.Any("error", err))
		// Decide how to handle failure to get initial state.
		// For now, proceed adding the light but log the error.
	} else {
		light.State = state
	}

	// Set LastSeen timestamp
	light.LastSeen = time.Now()

	// Acquire write lock briefly to update the maps
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if light already exists (e.g., from a previous discovery run)
	if _, exists := m.lights[light.ID]; exists {
		m.logger.Debug("light already exists, updating", slog.String("id", light.ID))
		// You might want to merge or update existing light info here if needed
	}

	m.clients[light.ID] = client
	m.lights[light.ID] = light // Add or update the light with fetched state

	m.logger.Info("added light", slog.String("id", light.ID), slog.String("ip", light.IP.String()), slog.Int("port", light.Port))
}

// StartCleanupWorker starts a background goroutine to remove stale lights.
func (m *Manager) StartCleanupWorker(ctx context.Context, cleanupInterval time.Duration, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		m.logger.Info("Starting cleanup worker", "interval", cleanupInterval, "timeout", timeout)

		for {
			select {
			case <-ctx.Done():
				m.logger.Info("Cleanup worker stopping")
				return
			case <-ticker.C:
				m.cleanupStaleLights(timeout)
			}
		}
	}()
}

func (m *Manager) cleanupStaleLights(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	staleLights := []string{}

	// Identify stale lights
	for id, light := range m.lights {
		if now.Sub(light.LastSeen) > timeout {
			staleLights = append(staleLights, id)
		}
	}

	// Remove stale lights
	for _, id := range staleLights {
		m.logger.Info("Removing stale light", "id", id)
		delete(m.lights, id)
		delete(m.clients, id)
	}
}
