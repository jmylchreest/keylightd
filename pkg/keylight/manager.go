/*
Concurrency Notes (audit):
- Read operations (GetDiscoveredLights, GetLights) use RLock and return snapshots (map/slice copies or new structs) to avoid external mutation of internal state.
- Mutations (AddLight, SetLightState*, cleanupStaleLights) perform network/device I/O outside the write lock; only the minimal in-memory updates are under Lock to reduce contention.
- cleanupStaleLights snapshots stale devices, probes outside the lock, then rechecks each snapshot before updating or removing it.
- deviceClient & state fetch patterns avoid holding locks during remote calls.
- Returned *Light pointers from GetLight should be treated as read-only by callers; direct mutation risks data races unless routed through manager methods.
Future considerations:
- Enforce deep-copy semantics for GetLight to eliminate accidental external mutation risks.
- Replace context.Background() usages with caller-provided contexts for cancellation propagation.
- Consider a lightweight RW lock partitioning (e.g., sharded maps) only if contention is observed under profiling.
*/
package keylight

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/errors"
	"github.com/jmylchreest/keylightd/internal/events"
)

// Manager manages Key Light devices
type Manager struct {
	httpProbeOnDisconnect map[string]bool
	lights                map[string]Light
	clients               map[string]DeviceClient
	mu                    sync.RWMutex
	logger                *slog.Logger
	eventBus              *events.Bus
}

// NewManager creates a new manager
func NewManager(logger *slog.Logger, lightConfig ...config.LightConfig) *Manager {
	probes := make(map[string]bool)
	for _, option := range lightConfig {
		if option.SerialNumber != "" && option.HTTPProbeOnDisconnect != nil {
			probes[option.SerialNumber] = *option.HTTPProbeOnDisconnect
		}
	}
	return &Manager{
		httpProbeOnDisconnect: probes,
		lights:                make(map[string]Light),
		clients:               make(map[string]DeviceClient),
		logger:                logger,
	}
}

// SetEventBus sets the event bus for publishing state change events.
// If not set, no events are emitted (fire-and-forget mode).
func (m *Manager) SetEventBus(bus *events.Bus) {
	m.eventBus = bus
}

// emit publishes an event if an event bus is configured.
func (m *Manager) emit(t events.EventType, data any) {
	if m.eventBus != nil {
		m.eventBus.Publish(events.NewEvent(t, data))
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
func (m *Manager) GetLight(ctx context.Context, id string) (*Light, error) {
	// Get client and light information
	client, light, err := m.deviceClient(id)
	if err != nil {
		return nil, err
	}

	// Fetch accessory info if needed
	// Using passed-in ctx (propagated from caller) instead of context.Background()
	needsInfo := light.ProductName == "" || light.SerialNumber == ""
	var info *AccessoryInfo

	if needsInfo {
		info, _ = m.fetchAccessoryInfo(ctx, client, id)
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
func (m *Manager) SetLightState(ctx context.Context, id string, propertyValue LightPropertyValue) error {
	// Validate the property value first
	if err := propertyValue.Validate(); err != nil {
		return errors.InvalidInputf("invalid property value: %w", err)
	}

	// Get client for this light
	client, _, err := m.deviceClient(id)
	if err != nil {
		return err
	}

	// Using passed-in ctx

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
	updatedLight, err := m.updateLightState(id, state)
	m.mu.Unlock()

	if err != nil {
		return errors.NotFoundf("light %s removed during state update", id)
	}

	// Emit state change event
	if updatedLight != nil {
		m.emit(events.LightStateChanged, updatedLight)
	}

	return nil
}

// SetLightBrightness sets the brightness of a light
func (m *Manager) SetLightBrightness(ctx context.Context, id string, brightness int) error {
	return m.SetLightState(ctx, id, BrightnessValue(brightness))
}

// SetLightTemperature sets the temperature of a light
func (m *Manager) SetLightTemperature(ctx context.Context, id string, temperature int) error {
	return m.SetLightState(ctx, id, TemperatureValue(temperature))
}

// SetLightPower sets the power state of a light
func (m *Manager) SetLightPower(ctx context.Context, id string, on bool) error {
	return m.SetLightState(ctx, id, OnValue(on))
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
func (m *Manager) AddLight(ctx context.Context, light Light) {
	// Create client for this light - not blocking, can be done before lock
	m.addDevice(ctx, light, NewKeyLightClient(light.IP.String(), light.Port, m.logger))
}

func (m *Manager) addDevice(ctx context.Context, light Light, client DeviceClient) {
	// Using caller-provided ctx

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

	light.resolveModel()
	m.clients[light.ID] = client
	m.lights[light.ID] = light // Add or update the light with fetched state

	// Log the light addition/update
	m.logLightInfo(ctx, slog.LevelInfo, "light: added/updated", &light)

	// Emit discovered event (covers both new discoveries and re-discoveries with updated state)
	m.emit(events.LightDiscovered, &light)
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
				m.cleanupStaleLights(ctx, timeout)
			}
		}
	}()
}

// cleanupStaleLights checks stale devices directly before treating missed mDNS as a disconnect.
func (m *Manager) cleanupStaleLights(ctx context.Context, timeout time.Duration) {
	if timeout <= 0 {
		timeout = config.DefaultStateTimeout
	}
	now := time.Now()
	m.mu.RLock()
	var stale []Light
	for _, light := range m.lights {
		if now.Sub(light.LastSeen) > timeout {
			stale = append(stale, light)
		}
	}
	m.mu.RUnlock()

	for _, candidate := range stale {
		if ctx.Err() != nil {
			return
		}
		var state *LightState
		enabled, configured := m.httpProbeOnDisconnect[candidate.SerialNumber]
		if !configured || enabled {
			client, _, err := m.deviceClient(candidate.ID)
			if err != nil {
				continue
			}
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			state, err = client.GetLightState(probeCtx)
			cancel()
			if err != nil || state == nil || len(state.Lights) != 1 || state.NumberOfLights != 1 {
				state = nil
			}
		}
		// Shutdown cancellation is not evidence that the device is offline.
		if ctx.Err() != nil {
			return
		}
		m.mu.Lock()
		current, exists := m.lights[candidate.ID]
		// Discovery or a control request may have refreshed or replaced this device
		// while the probe was in flight. Never overwrite that newer information.
		if !exists || !current.LastSeen.Equal(candidate.LastSeen) ||
			!current.IP.Equal(candidate.IP) || current.Port != candidate.Port {
			m.mu.Unlock()
			continue
		}
		if state != nil {
			updated, _ := m.updateLightState(candidate.ID, state)
			m.mu.Unlock()
			m.logger.Debug("Retaining stale light after successful HTTP probe", "id", candidate.ID)
			m.emit(events.LightStateChanged, updated)
			continue
		}
		delete(m.lights, candidate.ID)
		delete(m.clients, candidate.ID)
		m.mu.Unlock()
		m.logger.Info("Removing stale light", "id", candidate.ID)
		m.emit(events.LightRemoved, &current)
	}
}
