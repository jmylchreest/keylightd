package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/airble"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

type StartSetupInput struct {
	ID   string `path:"id" doc:"Light identifier"`
	Body *keylight.AirMK2SetupOptions
}

type SetupOutput struct {
	Body keylight.SetupStatus
}

type deviceSetupManager interface {
	StartAirMK2Setup(context.Context, string, keylight.AirMK2Pairer) (keylight.SetupStatus, error)
	DeviceSetupStatus(string) (keylight.SetupStatus, bool)
	CancelDeviceSetup(string) bool
	ForgetAirMK2(string) error
}

func (h *LightHandler) SetupStatus(_ context.Context, input *GetLightInput) (*SetupOutput, error) {
	manager, ok := h.Lights.(deviceSetupManager)
	if !ok {
		return nil, huma.Error503ServiceUnavailable("Device setup is unavailable")
	}
	status, ok := manager.DeviceSetupStatus(input.ID)
	if !ok {
		return nil, huma.Error404NotFound("No setup session for this light")
	}
	return &SetupOutput{Body: status}, nil
}

func (h *LightHandler) StartSetup(_ context.Context, input *StartSetupInput) (*SetupOutput, error) {
	manager, ok := h.Lights.(deviceSetupManager)
	if !ok || h.SetupContext == nil {
		return nil, huma.Error503ServiceUnavailable("Bluetooth pairing is unavailable on this daemon")
	}
	var pairer keylight.AirMK2Pairer
	var err error
	if input.Body != nil {
		pairer, err = airble.NewPairer(*input.Body)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
	} else if h.PairerForLight != nil {
		pairer, err = h.PairerForLight(input.ID)
	} else {
		return nil, huma.Error400BadRequest("Bluetooth peripheral and Wi-Fi settings are required")
	}
	if err != nil || pairer == nil {
		return nil, huma.Error503ServiceUnavailable("Bluetooth pairing is unavailable for this light")
	}
	var status keylight.SetupStatus
	if input.Body != nil && input.ID != keylight.NearbyAirMK2SetupID {
		named, ok := h.Lights.(interface {
			StartNamedAirMK2Setup(context.Context, string, keylight.AirMK2Pairer) (keylight.SetupStatus, error)
		})
		if !ok {
			return nil, huma.Error503ServiceUnavailable("Named Bluetooth setup is unavailable")
		}
		//nolint:contextcheck // Setup survives request completion and stops with the daemon.
		status, err = named.StartNamedAirMK2Setup(h.SetupContext, input.ID, pairer)
	} else if input.Body != nil {
		nearby, ok := h.Lights.(interface {
			StartNearbyAirMK2Setup(context.Context, keylight.AirMK2Pairer) (keylight.SetupStatus, error)
		})
		if !ok {
			return nil, huma.Error503ServiceUnavailable("Nearby setup is unavailable")
		}
		//nolint:contextcheck // Setup survives request completion and stops with the daemon.
		status, err = nearby.StartNearbyAirMK2Setup(h.SetupContext, pairer)
	} else {
		//nolint:contextcheck // Setup survives request completion and stops with the daemon.
		status, err = manager.StartAirMK2Setup(h.SetupContext, input.ID, pairer)
	}
	if err != nil {
		return nil, huma.Error409Conflict(err.Error())
	}
	return &SetupOutput{Body: status}, nil
}

func (h *LightHandler) CancelSetup(_ context.Context, input *GetLightInput) (*SetupOutput, error) {
	manager, ok := h.Lights.(deviceSetupManager)
	if !ok {
		return nil, huma.Error503ServiceUnavailable("Device setup is unavailable")
	}
	if !manager.CancelDeviceSetup(input.ID) {
		return nil, huma.Error404NotFound("No active setup session for this light")
	}
	return &SetupOutput{Body: keylight.SetupStatus{DeviceID: input.ID, Stage: "canceling"}}, nil
}

func (h *LightHandler) ForgetSetup(_ context.Context, input *GetLightInput) (*struct{}, error) {
	manager, ok := h.Lights.(deviceSetupManager)
	if !ok {
		return nil, huma.Error503ServiceUnavailable("Device setup is unavailable")
	}
	if err := manager.ForgetAirMK2(input.ID); err != nil {
		if errors.Is(err, keylight.ErrLightNotFound) {
			return nil, huma.Error404NotFound("No daemon-managed credentials for this light")
		}
		return nil, huma.Error409Conflict("Could not remove stored device credentials")
	}
	return nil, nil
}

type ConfirmSetupInput struct {
	ID   string `path:"id"`
	Body struct {
		ConfirmationID string `json:"confirmationId" minLength:"1"`
	}
}

func (h *LightHandler) ConfirmSetup(_ context.Context, input *ConfirmSetupInput) (*struct{}, error) {
	manager, ok := h.Lights.(interface{ ConfirmDeviceSetup(string, string) bool })
	if !ok || !manager.ConfirmDeviceSetup(input.ID, input.Body.ConfirmationID) {
		return nil, huma.Error409Conflict("No setup session awaiting confirmation")
	}
	return nil, nil
}

func (h *LightHandler) BlinkSetup(_ context.Context, input *ConfirmSetupInput) (*struct{}, error) {
	manager, ok := h.Lights.(interface{ BlinkDeviceSetup(string, string) bool })
	if !ok || !manager.BlinkDeviceSetup(input.ID, input.Body.ConfirmationID) {
		return nil, huma.Error409Conflict("No setup session awaiting confirmation")
	}
	return nil, nil
}

type JoinSetupInput struct {
	ID   string `path:"id"`
	Body struct {
		WiFiID  string                     `json:"wifiId" minLength:"1"`
		Network keylight.AirMK2WiFiOptions `json:"network"`
	}
}

func (h *LightHandler) JoinSetup(_ context.Context, input *JoinSetupInput) (*struct{}, error) {
	options := input.Body.Network
	if len(options.SSID) == 0 || len(options.SSID) > 32 || len(options.Password) > 64 ||
		(options.Security == 0 && options.Password != "") || (options.Security != 0 && options.Password == "") {
		return nil, huma.Error400BadRequest("Invalid Wi-Fi settings")
	}
	manager, ok := h.Lights.(interface {
		JoinDeviceSetupWiFi(string, string, keylight.AirMK2WiFiOptions) bool
	})
	if !ok || !manager.JoinDeviceSetupWiFi(input.ID, input.Body.WiFiID, options) {
		return nil, huma.Error409Conflict("No setup session awaiting Wi-Fi settings")
	}
	return nil, nil
}
