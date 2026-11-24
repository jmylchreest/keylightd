package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

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

func (h *LightHandler) StartSetup(_ context.Context, input *GetLightInput) (*SetupOutput, error) {
	manager, ok := h.Lights.(deviceSetupManager)
	if !ok || h.PairerForLight == nil || h.SetupContext == nil {
		return nil, huma.Error503ServiceUnavailable("Bluetooth pairing is unavailable on this daemon")
	}
	pairer, err := h.PairerForLight(input.ID)
	if err != nil || pairer == nil {
		return nil, huma.Error503ServiceUnavailable("Bluetooth pairing is unavailable for this light")
	}
	//nolint:contextcheck // Setup survives request completion and stops with the daemon.
	status, err := manager.StartAirMK2Setup(h.SetupContext, input.ID, pairer)
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
