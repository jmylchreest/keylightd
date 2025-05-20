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
	// Get client and light information
	client, light, err := m.getOrCreateClient(id)
	if err != nil {
		return nil, err
	}

	// Fetch accessory info if needed
	ctx := context.Background()
	needsInfo := light.ProductName == "" || light.SerialNumber == ""
	var info *AccessoryInfo
	
	if needsInfo {
		info, err = m.fetchAccessoryInfo(ctx, client, id)
		// Errors are already logged in fetchAccessoryInfo, continue without info
	}
	
	// Fetch current state
	state, err := m.fetchLightState(ctx, client, id)
	if err != nil {
		// If we have accessory info but state fetch failed, return partial information
		if needsInfo && info != nil {
			// Update light info with what we have
			m.mu.Lock()
			updatedLight, _ := m.updateLightInfo(id, info)
			m.mu.Unlock()
			return updatedLight, err
		}
		return light, err
	}

	// Update both state and info if needed
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// First update state
	updatedLight, err := m.updateLightState(id, state)
	if err != nil {
		return light, err
	}
	
	// Then update info if we fetched it
	if needsInfo && info != nil {
		updatedLight, err = m.updateLightInfo(id, info)
		if err != nil {
			return updatedLight, err
		}
	}

	// Log light information
	m.logger.Debug("GetLight returning light", slog.String("id", id), slog.Any("light", *updatedLight))
	if updatedLight.ProductName == "" || updatedLight.SerialNumber == "" || updatedLight.FirmwareVersion == "" {
		m.logger.Warn("GetLight: missing key fields in returned light", 
			slog.String("id", id), 
			slog.String("productname", updatedLight.ProductName), 
			slog.String("serialnumber", updatedLight.SerialNumber), 
			slog.String("firmwareversion", updatedLight.FirmwareVersion))
	}
	
	return updatedLight, nil
}

// SetLightState sets the state of a light using type-safe property values
// It fetches the current state, updates the specified property, and sends the new state to the device.
func (m *Manager) SetLightState(id string, propertyValue LightPropertyValue) error {
	// Validate the property value first
	if err := propertyValue.Validate(); err != nil {
		return errors.InvalidInputf("invalid property value: %w", err)
	}

	// Get client for this light
	client, _, err := m.getOrCreateClient(id)
	if err != nil {
		return err
	}
	
	ctx := context.Background()
	
	// Get current state from the device
	state, err := m.fetchLightState(ctx, client, id)
	if err != nil {
		return err
	}
	
	// Validate and prepare state update
	propertyName := propertyValue.PropertyName()
	if err := m.validateAndPrepareStateUpdate(string(propertyName), propertyValue.Value(), state); err != nil {
		return err
	}
	
	// Send updated state to device
	if err := client.SetLightState(
		ctx,
		state.Lights[0].On == 1,
		state.Lights[0].Brightness,
		state.Lights[0].Temperature,
	); err != nil {
		return errors.LogErrorAndReturn(
			m.logger,
			errors.DeviceUnavailablef("failed to send updated state: %w", err),
			"failed to set light state",
			"id", id,
			"property", string(propertyName),
		)
	}

	// Update local state in the manager with a write lock
	m.mu.Lock()
	_, err = m.updateLightState(id, state)
	m.mu.Unlock()
	
	if err != nil {
		return errors.NotFoundf("light %s removed during state update", id)
	}

	return nil
}

// SetLightStateOld is the legacy version of SetLightState (deprecated)
// Use the type-safe version with LightPropertyValue parameter instead.
// This method is kept for backward compatibility with existing code.
// Deprecated: Use SetLightState with LightPropertyValue instead.
func (m *Manager) SetLightStateOld(id string, property string, value any) error {
	// Convert the string property and interface value to our type-safe version
	switch property {
	case string(PropertyOn):
		on, ok := value.(bool)
		if !ok {
			return errors.InvalidInputf("invalid value type for on: %T", value)
		}
		return m.SetLightState(id, OnValue(on))
		
	case string(PropertyBrightness):
		brightness, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for brightness: %T", value)
		}
		return m.SetLightState(id, BrightnessValue(brightness))
		
	case string(PropertyTemperature):
		temp, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for temperature: %T", value)
		}
		return m.SetLightState(id, TemperatureValue(temp))
		
	default:
		return errors.InvalidInputf("unknown property: %s", property)
	}
}

// SetLightBrightness sets the brightness of a light
func (m *Manager) SetLightBrightness(id string, brightness int) error {
	return m.SetLightState(id, BrightnessValue(brightness))
}

// SetLightTemperature sets the temperature of a light
func (m *Manager) SetLightTemperature(id string, temperature int) error {
	return m.SetLightState(id, TemperatureValue(temperature))
}

// SetLightPower sets the power state of a light
func (m *Manager) SetLightPower(id string, on bool) error {
	return m.SetLightState(id, OnValue(on))
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
	ctx := context.Background()

	// Get current state - happens OUTSIDE the lock
	state, err := m.fetchLightState(ctx, client, light.ID)
	if err != nil {
		// Proceed adding the light even with error, error already logged
	} else if state != nil {
		// Update light with state information
		light.State = state
		if len(state.Lights) > 0 {
			light.Temperature = state.Lights[0].Temperature
			light.Brightness = state.Lights[0].Brightness
			light.On = state.Lights[0].On == 1
		}
	}

	// Try to get accessory info if needed
	if light.ProductName == "" || light.SerialNumber == "" {
		info, infoErr := m.fetchAccessoryInfo(ctx, client, light.ID)
		if infoErr == nil && info != nil {
			light.ProductName = info.ProductName
			light.HardwareBoardType = info.HardwareBoardType
			light.FirmwareVersion = info.FirmwareVersion
			light.FirmwareBuild = info.FirmwareBuildNumber
			light.SerialNumber = info.SerialNumber
			light.Name = info.DisplayName
		}
	}

	// Set LastSeen timestamp
	light.LastSeen = time.Now()

	// Acquire write lock briefly to update the maps
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if light already exists
	if existingLight, exists := m.lights[light.ID]; exists {
		m.logger.Debug("light already exists, updating", slog.String("id", light.ID))
		
		// Preserve any fields that might be missing in the new light
		if light.ProductName == "" {
			light.ProductName = existingLight.ProductName
		}
		if light.SerialNumber == "" {
			light.SerialNumber = existingLight.SerialNumber
		}
		if light.Name == "" {
			light.Name = existingLight.Name
		}
	}

	m.clients[light.ID] = client
	m.lights[light.ID] = light // Add or update the light with fetched state

	// Log the light addition/update
	m.logLightInfo(slog.LevelInfo, "light: added/updated", &light)
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
	// Use default timeout if the provided one is invalid
	if timeout <= 0 {
		m.logger.Debug("Invalid cleanup timeout, using default", 
			"provided", timeout, 
			"default", config.DefaultStateTimeout)
		timeout = config.DefaultStateTimeout
	}

	now := time.Now()
	
	// First identify stale lights with read lock to minimize lock duration
	m.mu.RLock()
	staleLights := []string{}
	
	for id, light := range m.lights {
		if now.Sub(light.LastSeen) > timeout {
			staleLights = append(staleLights, id)
		}
	}
	m.mu.RUnlock()
	
	// If no stale lights, return quickly without acquiring write lock
	if len(staleLights) == 0 {
		return
	}

	// Now remove the stale lights with write lock
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Double-check the lights are still stale after acquiring write lock
	for _, id := range staleLights {
		if light, exists := m.lights[id]; exists {
			// Re-check timeout condition to handle race condition
			// where the light might have been updated while we were unlocked
			if now.Sub(light.LastSeen) > timeout {
				m.logger.Info("Removing stale light", "id", id)
				delete(m.lights, id)
				delete(m.clients, id)
			}
		}
	}
}
