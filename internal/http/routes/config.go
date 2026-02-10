// Package routes provides shared route registration for the keylightd HTTP API.
// Both the main server and the OpenAPI generator use the same route definitions,
// ensuring the spec is always in sync with the implementation.
package routes

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/http/mw"
)

// NewHumaConfig creates the shared Huma configuration for the API.
func NewHumaConfig(version, baseURL string) huma.Config {
	cfg := huma.DefaultConfig("keylightd API", version)
	cfg.Info.Description = "REST API for controlling Elgato Key Light devices via keylightd daemon."

	// Disable $schema field in responses
	cfg.CreateHooks = nil

	if baseURL != "" {
		cfg.Servers = []*huma.Server{
			{URL: baseURL, Description: "API Server"},
		}
	}

	// Add security scheme for API key auth (both Bearer and X-API-Key header)
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		mw.SecurityScheme: {
			Type:        "http",
			Scheme:      "bearer",
			Description: "API key authentication. Include your API key as `Authorization: Bearer <key>` or `X-API-Key: <key>`.",
		},
	}

	// Define OpenAPI tags
	cfg.Tags = []*huma.Tag{
		{Name: "Lights", Description: "Light discovery and control"},
		{Name: "Groups", Description: "Light group management"},
		{Name: "API Keys", Description: "API key management"},
		{Name: "Logging", Description: "Runtime log level and filter management"},
	}

	return cfg
}
