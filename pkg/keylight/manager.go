package keylight

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/errors"
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
		return nil, errors.NotFoundf("light %s not found", id)
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
			return nil, errors.NotFoundf("light %s not found", id)
		}
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
		m.mu.Unlock()
	}

	var updatedLight Light = light // Work on a copy

	// Get accessory info if not already set - happens OUTSIDE the lock
	if updatedLight.ProductName == "" || updatedLight.SerialNumber == "" {
		info, err := client.GetAccessoryInfo(context.Background())
		if err != nil {
			// Log error but continue without accessory info if fetching fails
			errors.LogErrorAndReturn(m.logger, err, "failed to get accessory info during GetLight", "id", id)
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
	state, err := client.GetLightState(context.Background())
	if err != nil {
		// Return the error with proper wrapping and logging
		return &updatedLight, errors.LogErrorAndReturn(
			m.logger,
			errors.DeviceUnavailablef("failed to get current state: %w", err),
			"failed to get current state during GetLight",
			"id", id,
		)
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
		return nil, errors.NotFoundf("light %s not found during update after fetch", id)
	}
	m.mu.Unlock()

	// Log the final light struct before returning
	m.logger.Debug("GetLight returning light", slog.String("id", id), slog.Any("light", updatedLight))
	if updatedLight.ProductName == "" || updatedLight.SerialNumber == "" || updatedLight.FirmwareVersion == "" {
		m.logger.Warn("GetLight: missing key fields in returned light", slog.String("id", id), slog.String("productname", updatedLight.ProductName), slog.String("serialnumber", updatedLight.SerialNumber), slog.String("firmwareversion", updatedLight.FirmwareVersion))
	}
	return &updatedLight, nil
}

// SetLightState sets the state of a light
// It fetches the current state, updates the specified property, and sends the new state to the device.
func (m *Manager) SetLightState(id string, property string, value any) error {
	// Use RLock to safely read the light and client initially
	m.mu.RLock()
	light, exists := m.lights[id]
	client, clientExists := m.clients[id]
	m.mu.RUnlock()

	if !exists {
		return errors.NotFoundf("light %s not found", id)
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
			return errors.NotFoundf("light %s not found", id)
		}
		client = NewKeyLightClient(light.IP.String(), light.Port, m.logger)
		m.clients[id] = client
		m.mu.Unlock()
	}

	// Get current state from the device - happens OUTSIDE the lock
	state, err := client.GetLightState(context.Background())
	if err != nil {
		return errors.LogErrorAndReturn(
			m.logger,
			errors.DeviceUnavailablef("failed to get current state before setting: %w", err),
			"failed to get light state",
			"id", id,
		)
	}

	// Update state based on property - happens OUTSIDE the lock
	switch property {
	case "on":
		on, ok := value.(bool)
		if !ok {
			return errors.InvalidInputf("invalid value type for on: %T", value)
		}
		state.Lights[0].On = boolToInt(on)
	case "brightness":
		brightness, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for brightness: %T", value)
		}
		// Clamp to valid range using constants from config package
		if brightness < config.MinBrightness {
			brightness = config.MinBrightness
		} else if brightness > config.MaxBrightness {
			brightness = config.MaxBrightness
		}
		state.Lights[0].Brightness = brightness
	case "temperature":
		temp, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for temperature: %T", value)
		}
		// Convert from Kelvin to mireds
		state.Lights[0].Temperature = convertTemperatureToDevice(temp)
	default:
		return errors.InvalidInputf("unknown property: %s", property)
	}

	// Send updated state to device - happens OUTSIDE the lock
	if err := client.SetLightState(
		context.Background(),
		state.Lights[0].On == 1,
		state.Lights[0].Brightness,
		state.Lights[0].Temperature,
	); err != nil {
		return errors.LogErrorAndReturn(
			m.logger,
			errors.DeviceUnavailablef("failed to send updated state: %w", err),
			"failed to set light state",
			"id", id,
			"property", property,
		)
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
		// Light was removed while we were setting state.
		m.mu.Unlock()
		return errors.NotFoundf("light %s removed during state update", id)
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
	state, err := client.GetLightState(context.Background())
	if err != nil {
		errors.LogErrorAndReturn(
			m.logger,
			err,
			"failed to get initial light state during AddLight",
			"id", light.ID,
		)
		// Proceed adding the light even with error, but the error was logged
	} else {
		light.State = state
		if state != nil && len(state.Lights) > 0 {
			light.Temperature = state.Lights[0].Temperature
			light.Brightness = state.Lights[0].Brightness
			light.On = state.Lights[0].On == 1
		}
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

	m.logger.Info("light: added",
		slog.String("id", light.ID),
		slog.String("ip", light.IP.String()),
		slog.Int("port", light.Port),
		slog.Int("brightness", light.Brightness),
		slog.Bool("on", light.On),
		slog.Int("temperature", light.Temperature),
	)
}

// StartCleanupWorker starts a background goroutine to remove stale lights.
func (m *Manager) StartCleanupWorker(ctx context.Context, cleanupInterval time.Duration, timeout time.Duration) {
	m.logger.Debug("light: StartCleanupWorker called", "interval", cleanupInterval, "timeout", timeout)
	if cleanupInterval <= 0 {
		m.logger.Warn("Cleanup interval must be positive, using default instead", 
			"interval", cleanupInterval, 
			"default", config.DefaultCleanupInterval)
		cleanupInterval = config.DefaultCleanupInterval
	}
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		m.logger.Info("light: cleanup worker started", "interval", cleanupInterval, "timeout", timeout)
		for {
			select {
			case <-ctx.Done():
				m.logger.Info("light: cleanup worker stopped (context canceled)")
				return
			case <-ticker.C:
				m.cleanupStaleLights(timeout)
			}
		}
	}()
}

// cleanupStaleLights removes lights that haven't been seen for a while
func (m *Manager) cleanupStaleLights(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Use default timeout if the provided one is invalid
	if timeout <= 0 {
		m.logger.Debug("Invalid cleanup timeout, using default", 
			"provided", timeout, 
			"default", config.DefaultStateTimeout)
		timeout = config.DefaultStateTimeout
	}

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
