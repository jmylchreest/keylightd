package main

import (
	"errors"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// LightSetup bridges the tray's setup dialog to the configured daemon.
func (a *App) LightSetup(action, id, confirmationID string, options keylight.AirMK2SetupOptions) (map[string]any, error) {
	c, ok := a.client.(interface {
		StartAirMK2Setup(string, keylight.AirMK2SetupOptions) (map[string]any, error)
		GetLightSetup(string) (map[string]any, error)
		BlinkLightSetup(string, string) error
		ConfirmLightSetup(string, string) error
		CancelLightSetup(string) (map[string]any, error)
		JoinLightSetup(string, string, keylight.AirMK2WiFiOptions) error
	})
	if !ok {
		return nil, errors.New("Bluetooth setup is unavailable on this connection")
	}
	switch action {
	case "start":
		return c.StartAirMK2Setup(id, options)
	case "wifi":
		return nil, c.JoinLightSetup(id, confirmationID, keylight.AirMK2WiFiOptions{SSID: options.SSID, Password: options.Password, Security: options.Security})
	case "status":
		return c.GetLightSetup(id)
	case "blink":
		return nil, c.BlinkLightSetup(id, confirmationID)
	case "confirm":
		return nil, c.ConfirmLightSetup(id, confirmationID)
	case "cancel":
		return c.CancelLightSetup(id)
	default:
		return nil, errors.New("unknown light setup action")
	}
}
