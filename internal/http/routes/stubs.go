package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/http/handlers"
)

// StubHandlers returns a Handlers instance with stub implementations.
// All handlers return nil responses — these are only used for OpenAPI generation
// where Huma extracts type information from function signatures.
func StubHandlers() *Handlers {
	return &Handlers{
		HealthCheck: func(_ context.Context, _ *handlers.HealthInput) (*handlers.HealthOutput, error) {
			return nil, nil
		},
		Light:   &stubLightHandlers{},
		Group:   &stubGroupHandlers{},
		APIKey:  &stubAPIKeyHandlers{},
		Logging: &stubLoggingHandlers{},
	}
}

// --- Light stubs ---

type stubLightHandlers struct{}

func (s *stubLightHandlers) ListLights(_ context.Context, _ *handlers.ListLightsInput) (*handlers.ListLightsOutput, error) {
	return nil, nil
}

func (s *stubLightHandlers) GetLight(_ context.Context, _ *handlers.GetLightInput) (*handlers.GetLightOutput, error) {
	return nil, nil
}

func (s *stubLightHandlers) SetLightState(_ context.Context, _ *handlers.SetLightStateInput) (*handlers.SetLightStateOutput, error) {
	return nil, nil
}

// --- Group stubs ---

type stubGroupHandlers struct{}

func (s *stubGroupHandlers) ListGroups(_ context.Context, _ *handlers.ListGroupsInput) (*handlers.ListGroupsOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) CreateGroup(_ context.Context, _ *handlers.CreateGroupInput) (*handlers.CreateGroupOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) GetGroup(_ context.Context, _ *handlers.GetGroupInput) (*handlers.GetGroupOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) DeleteGroup(_ context.Context, _ *handlers.DeleteGroupInput) (*handlers.DeleteGroupOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) SetGroupLights(_ context.Context, _ *handlers.SetGroupLightsInput) (*handlers.SetGroupLightsOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) SetGroupState(_ context.Context, _ *handlers.SetGroupStateInput) (*handlers.SetGroupStateOutput, error) {
	return nil, nil
}

func (s *stubGroupHandlers) SetGroupStateRaw(_ huma.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Stub — never called during OpenAPI generation
	}
}

// --- API Key stubs ---

type stubAPIKeyHandlers struct{}

func (s *stubAPIKeyHandlers) CreateAPIKey(_ context.Context, _ *handlers.CreateAPIKeyInput) (*handlers.CreateAPIKeyOutput, error) {
	return nil, nil
}

func (s *stubAPIKeyHandlers) ListAPIKeys(_ context.Context, _ *handlers.ListAPIKeysInput) (*handlers.ListAPIKeysOutput, error) {
	return nil, nil
}

func (s *stubAPIKeyHandlers) DeleteAPIKey(_ context.Context, _ *handlers.DeleteAPIKeyInput) (*handlers.DeleteAPIKeyOutput, error) {
	return nil, nil
}

func (s *stubAPIKeyHandlers) SetAPIKeyDisabled(_ context.Context, _ *handlers.SetAPIKeyDisabledInput) (*handlers.SetAPIKeyDisabledOutput, error) {
	return nil, nil
}

// --- Logging stubs ---

type stubLoggingHandlers struct{}

func (s *stubLoggingHandlers) ListFilters(_ context.Context, _ *handlers.ListFiltersInput) (*handlers.ListFiltersOutput, error) {
	return nil, nil
}

func (s *stubLoggingHandlers) SetFilters(_ context.Context, _ *handlers.SetFiltersInput) (*handlers.SetFiltersOutput, error) {
	return nil, nil
}

func (s *stubLoggingHandlers) SetLevel(_ context.Context, _ *handlers.SetLevelInput) (*handlers.SetLevelOutput, error) {
	return nil, nil
}
