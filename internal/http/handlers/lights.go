package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// --- List Lights ---

// ListLightsInput is the input for listing all lights.
type ListLightsInput struct{}

// ListLightsOutput is the output for listing all lights.
// Returns lights as a map keyed by ID for backward compatibility with the GNOME extension.
type ListLightsOutput struct {
	Body map[string]LightResponse
}

// --- Get Light ---

// GetLightInput is the input for getting a single light.
type GetLightInput struct {
	ID string `path:"id" doc:"Light identifier"`
}

// GetLightOutput is the output for getting a single light.
type GetLightOutput struct {
	Body LightResponse
}

// --- Set Light State ---

// SetLightStateInput is the input for setting a light's state.
type SetLightStateInput struct {
	ID   string `path:"id" doc:"Light identifier"`
	Body struct {
		On          *bool `json:"on,omitempty" doc:"Power state"`
		Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100)"`
		Temperature *int  `json:"temperature,omitempty" doc:"Color temperature in Kelvin"`
	}
}

// SetLightStateOutput is the output for setting a light's state.
type SetLightStateOutput struct {
	Body StatusResponse
}

// LightHandler implements light-related HTTP handlers.
type LightHandler struct {
	Lights keylight.LightManager
}

// ListLights returns all discovered lights as a map keyed by ID.
func (h *LightHandler) ListLights(_ context.Context, _ *ListLightsInput) (*ListLightsOutput, error) {
	lights := h.Lights.GetLights()
	return &ListLightsOutput{
		Body: LightsMapFromKeylight(lights),
	}, nil
}

// GetLight returns a single light by ID.
func (h *LightHandler) GetLight(ctx context.Context, input *GetLightInput) (*GetLightOutput, error) {
	light, err := h.Lights.GetLight(ctx, input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("Light not found: %s", err))
	}
	resp := LightFromKeylight(light)
	return &GetLightOutput{Body: resp}, nil
}

// SetLightState sets one or more properties on a light.
func (h *LightHandler) SetLightState(ctx context.Context, input *SetLightStateInput) (*SetLightStateOutput, error) {
	var errs []string

	if input.Body.On != nil {
		if err := h.Lights.SetLightState(ctx, input.ID, keylight.OnValue(*input.Body.On)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if input.Body.Brightness != nil {
		if err := h.Lights.SetLightState(ctx, input.ID, keylight.BrightnessValue(*input.Body.Brightness)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if input.Body.Temperature != nil {
		if err := h.Lights.SetLightState(ctx, input.ID, keylight.TemperatureValue(*input.Body.Temperature)); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return nil, huma.Error500InternalServerError(
			fmt.Sprintf("Error(s) setting light state: %s", joinStrings(errs, "; ")),
		)
	}

	return &SetLightStateOutput{
		Body: StatusResponse{Status: "ok"},
	}, nil
}

// joinStrings joins strings with a separator (avoids importing strings just for this).
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

// Ensure LightHandler implements the interface at compile time.
var _ LightHandlers = (*LightHandler)(nil)

// LightHandlers defines the interface for light operations.
type LightHandlers interface {
	ListLights(ctx context.Context, input *ListLightsInput) (*ListLightsOutput, error)
	GetLight(ctx context.Context, input *GetLightInput) (*GetLightOutput, error)
	SetLightState(ctx context.Context, input *SetLightStateInput) (*SetLightStateOutput, error)
}

// Ensure SetLightStateOutput is valid for non-error responses.
// The handler uses huma.Error for error cases, not the 207 pattern.
// For the single-light case, errors are returned as 500.
// The 207 pattern is only used for group state (multi-target).
var _ = http.StatusOK // reference to avoid unused import if needed
