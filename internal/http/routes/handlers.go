package routes

import (
	"github.com/jmylchreest/keylightd/internal/http/handlers"
)

// Handlers aggregates all handler interfaces for route registration.
// For the main server, pass real handler implementations.
// For OpenAPI generation, pass stub implementations.
type Handlers struct {
	Light   handlers.LightHandlers
	Group   handlers.GroupHandlers
	APIKey  handlers.APIKeyHandlers
	Logging handlers.LoggingHandlers
}
