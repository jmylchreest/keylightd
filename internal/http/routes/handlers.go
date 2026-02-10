package routes

import (
	"context"

	"github.com/jmylchreest/keylightd/internal/http/handlers"
)

// HealthCheckFunc is the type for health check handler functions.
type HealthCheckFunc func(ctx context.Context, input *handlers.HealthInput) (*handlers.HealthOutput, error)

// VersionCheckFunc is the type for version handler functions.
type VersionCheckFunc func(ctx context.Context, input *handlers.VersionInput) (*handlers.VersionOutput, error)

// Handlers aggregates all handler interfaces for route registration.
// For the main server, pass real handler implementations.
// For OpenAPI generation, pass stub implementations.
type Handlers struct {
	HealthCheck  HealthCheckFunc
	VersionCheck VersionCheckFunc
	Light        handlers.LightHandlers
	Group        handlers.GroupHandlers
	APIKey       handlers.APIKeyHandlers
	Logging      handlers.LoggingHandlers
}
