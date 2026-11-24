package keylight

import (
	"context"
	"log/slog"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/errors"
)

// deviceClient retrieves the light and the client registered with it.
func (m *Manager) deviceClient(id string) (DeviceClient, *Light, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	light, exists := m.lights[id]
	if !exists {
		return nil, nil, errors.NotFoundf("light %s not found", id)
	}
	client, exists := m.clients[id]
	if !exists {
		return nil, nil, errors.DeviceUnavailablef("light %s has no registered client", id)
	}
	return client, &light, nil
}

// updateLightState updates a light's state in the manager's internal maps.
// Requires a write lock to be held by the caller.
func (m *Manager) updateLightState(id string, state *LightState) (*Light, error) {
	light, exists := m.lights[id]
	if !exists {
		return nil, errors.NotFoundf("light %s not found", id)
	}

	// Update state fields
	light.State = state
	if state != nil && len(state.Lights) > 0 {
		light.Temperature = state.Lights[0].Temperature
		light.Brightness = state.Lights[0].Brightness
		light.On = state.Lights[0].On == 1
	}

	// Update last seen timestamp
	light.LastSeen = time.Now()

	// Store updated light back into the map
	m.lights[id] = light

	return &light, nil
}

// updateLightInfo updates a light's information fields in the manager's internal maps.
// Requires a write lock to be held by the caller.
func (m *Manager) updateLightInfo(id string, info *AccessoryInfo) (*Light, error) {
	light, exists := m.lights[id]
	if !exists {
		return nil, errors.NotFoundf("light %s not found", id)
	}

	// Update info fields
	if info != nil {
		light.ProductName = info.ProductName
		light.HardwareBoardType = info.HardwareBoardType
		light.FirmwareVersion = info.FirmwareVersion
		light.FirmwareBuild = info.FirmwareBuildNumber
		light.SerialNumber = info.SerialNumber
		light.Name = info.DisplayName
	}

	light.resolveModel()

	// Store updated light back into the map
	m.lights[id] = light

	return &light, nil
}

// fetchLightState retrieves the current state of a light from the device.
func (m *Manager) fetchLightState(ctx context.Context, client DeviceClient, id string) (*LightState, error) {

	state, err := client.GetLightState(ctx)
	if err != nil {
		return nil, errors.LogErrorAndReturn(
			m.logger,
			errors.DeviceUnavailablef("failed to get current state: %w", err),
			"failed to get current state",
			"id", id,
		)
	}
	return state, nil
}

// fetchAccessoryInfo retrieves accessory information for a light from the device.
func (m *Manager) fetchAccessoryInfo(ctx context.Context, client DeviceClient, id string) (*AccessoryInfo, error) {

	info, err := client.GetAccessoryInfo(ctx)
	if err != nil {
		return nil, errors.LogErrorAndReturn(
			m.logger,
			err,
			"failed to get accessory info",
			"id", id,
		)
	}
	return info, nil
}

// validateAndPrepareStateUpdate validates a property value before sending it to the device.
// Although we have type validation through the LightPropertyValue interface,
// this method is needed to update the device state object.
func (m *Manager) validateAndPrepareStateUpdate(property string, value any, currentState *LightState) error {
	if currentState == nil || len(currentState.Lights) == 0 {
		return errors.InvalidInputf("invalid current state")
	}

	switch PropertyName(property) {
	case PropertyOn:
		on, ok := value.(bool)
		if !ok {
			return errors.InvalidInputf("invalid value type for on: %T", value)
		}
		currentState.Lights[0].On = boolToInt(on)

	case PropertyBrightness:
		brightness, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for brightness: %T", value)
		}
		// Validation is already done in LightPropertyValue.Validate()
		// This is just a safety check
		if brightness < config.MinBrightness {
			brightness = config.MinBrightness
		} else if brightness > config.MaxBrightness {
			brightness = config.MaxBrightness
		}
		currentState.Lights[0].Brightness = brightness

	case PropertyTemperature:
		temp, ok := value.(int)
		if !ok {
			return errors.InvalidInputf("invalid value type for temperature: %T", value)
		}
		// Auto-detect format: mireds (143-344) vs Kelvin (2900-7000)
		// If temp is in mireds range, use as-is; otherwise convert from Kelvin
		if temp >= 143 && temp <= 344 {
			// Already in mireds format (from device)
			currentState.Lights[0].Temperature = temp
		} else {
			// Kelvin format (from API/user), convert to mireds
			currentState.Lights[0].Temperature = convertTemperatureToDevice(temp)
		}

	default:
		return errors.InvalidInputf("unknown property: %s", property)
	}

	return nil
}

// logLightInfo logs detailed information about a light.
func (m *Manager) logLightInfo(ctx context.Context, level slog.Level, message string, light *Light) {
	if light == nil {
		return
	}

	m.logger.Log(ctx, level, message,
		slog.String("id", light.ID),
		slog.String("ip", light.IP.String()),
		slog.Int("port", light.Port),
		slog.Int("brightness", light.Brightness),
		slog.Bool("on", light.On),
		slog.Int("temperature", light.Temperature),
		slog.String("name", light.Name),
		slog.String("serial", light.SerialNumber),
	)
}
