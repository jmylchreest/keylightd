package routes

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/http/mw"
)

// Register registers all API routes with the given Huma API instance.
// Pass real handler implementations for the main server, or stub implementations
// for OpenAPI generation.
func Register(api huma.API, h *Handlers) {
	// --- Health ---
	mw.PublicGet(api, "/api/v1/health", h.HealthCheck,
		mw.WithTags("Health"),
		mw.WithSummary("Health check"),
		mw.WithDescription("Returns service health status. This endpoint does not require authentication."),
		mw.WithOperationID("healthCheck"))

	mw.HiddenGet(api, "/healthz", h.HealthCheck)

	// --- Version ---
	mw.PublicGet(api, "/api/v1/version", h.VersionCheck,
		mw.WithTags("Version"),
		mw.WithSummary("Daemon version"),
		mw.WithDescription("Returns the running daemon's version, commit, and build date. This endpoint does not require authentication."),
		mw.WithOperationID("getVersion"))

	// --- Lights ---
	mw.ProtectedGet(api, "/api/v1/lights", h.Light.ListLights,
		mw.WithTags("Lights"),
		mw.WithSummary("List all lights"),
		mw.WithDescription("Returns all discovered lights as a map keyed by light ID."),
		mw.WithOperationID("listLights"))

	mw.ProtectedGet(api, "/api/v1/lights/{id}", h.Light.GetLight,
		mw.WithTags("Lights"),
		mw.WithSummary("Get a light"),
		mw.WithOperationID("getLight"))

	mw.ProtectedPost(api, "/api/v1/lights/{id}/state", h.Light.SetLightState,
		mw.WithTags("Lights"),
		mw.WithSummary("Set light state"),
		mw.WithDescription("Set one or more properties (on, brightness, temperature) on a light."),
		mw.WithOperationID("setLightState"))

	// --- Groups ---
	mw.ProtectedGet(api, "/api/v1/groups", h.Group.ListGroups,
		mw.WithTags("Groups"),
		mw.WithSummary("List all groups"),
		mw.WithOperationID("listGroups"))

	mw.ProtectedPost(api, "/api/v1/groups", h.Group.CreateGroup,
		mw.WithTags("Groups"),
		mw.WithSummary("Create a group"),
		mw.WithOperationID("createGroup"),
		mw.WithDefaultStatus(201))

	mw.ProtectedGet(api, "/api/v1/groups/{id}", h.Group.GetGroup,
		mw.WithTags("Groups"),
		mw.WithSummary("Get a group"),
		mw.WithOperationID("getGroup"))

	mw.ProtectedDelete(api, "/api/v1/groups/{id}", h.Group.DeleteGroup,
		mw.WithTags("Groups"),
		mw.WithSummary("Delete a group"),
		mw.WithOperationID("deleteGroup"),
		mw.WithDefaultStatus(204))

	mw.ProtectedPut(api, "/api/v1/groups/{id}/lights", h.Group.SetGroupLights,
		mw.WithTags("Groups"),
		mw.WithSummary("Set group lights"),
		mw.WithDescription("Set which lights belong to a group."),
		mw.WithOperationID("setGroupLights"))

	// Note: SetGroupState is registered as a raw Chi route in server.go
	// because it needs to return HTTP 207 Multi-Status on partial failures,
	// which Huma doesn't natively support. We still register it here for
	// OpenAPI documentation purposes.
	mw.ProtectedPut(api, "/api/v1/groups/{id}/state", h.Group.SetGroupState,
		mw.WithTags("Groups"),
		mw.WithSummary("Set group state"),
		mw.WithDescription("Set state for one or more groups. The ID parameter supports comma-separated IDs or names for multi-group targeting. Returns 200 on success, 207 on partial failure."),
		mw.WithOperationID("setGroupState"))

	// --- API Keys ---
	mw.ProtectedPost(api, "/api/v1/apikeys", h.APIKey.CreateAPIKey,
		mw.WithTags("API Keys"),
		mw.WithSummary("Create an API key"),
		mw.WithOperationID("createApiKey"),
		mw.WithDefaultStatus(201))

	mw.ProtectedGet(api, "/api/v1/apikeys", h.APIKey.ListAPIKeys,
		mw.WithTags("API Keys"),
		mw.WithSummary("List API keys"),
		mw.WithOperationID("listApiKeys"))

	mw.ProtectedDelete(api, "/api/v1/apikeys/{key}", h.APIKey.DeleteAPIKey,
		mw.WithTags("API Keys"),
		mw.WithSummary("Delete an API key"),
		mw.WithOperationID("deleteApiKey"),
		mw.WithDefaultStatus(204))

	mw.ProtectedPut(api, "/api/v1/apikeys/{key}/disabled", h.APIKey.SetAPIKeyDisabled,
		mw.WithTags("API Keys"),
		mw.WithSummary("Enable or disable an API key"),
		mw.WithOperationID("setApiKeyDisabled"))

	// --- Logging ---
	mw.ProtectedGet(api, "/api/v1/logging/filters", h.Logging.ListFilters,
		mw.WithTags("Logging"),
		mw.WithSummary("List log filters and current level"),
		mw.WithDescription("Returns the current global log level and all active log filters."),
		mw.WithOperationID("listLogFilters"))

	mw.ProtectedPut(api, "/api/v1/logging/filters", h.Logging.SetFilters,
		mw.WithTags("Logging"),
		mw.WithSummary("Replace all log filters"),
		mw.WithDescription("Validates and replaces all active log filters. Invalid filters are rejected entirely."),
		mw.WithOperationID("setLogFilters"))

	mw.ProtectedPut(api, "/api/v1/logging/level", h.Logging.SetLevel,
		mw.WithTags("Logging"),
		mw.WithSummary("Set global log level"),
		mw.WithDescription("Changes the global log level at runtime. Valid values: debug, info, warn, error."),
		mw.WithOperationID("setLogLevel"))
}
