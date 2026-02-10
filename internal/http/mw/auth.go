package mw

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jmylchreest/keylightd/internal/apikey"
)

// HumaAuth returns a Huma middleware that handles API key authentication.
// It checks the operation's security requirements and validates API keys
// from either the Authorization: Bearer header or X-API-Key header.
func HumaAuth(api huma.API, apikeyManager *apikey.Manager) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		if op == nil {
			next(ctx)
			return
		}

		// Check if this operation requires auth
		if !operationRequiresAuth(op) {
			next(ctx)
			return
		}

		// Extract API key from headers
		apiKey := ctx.Header("Authorization")
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(apiKey, bearerPrefix) {
			apiKey = apiKey[len(bearerPrefix):]
		} else {
			apiKey = ctx.Header("X-API-Key")
		}

		if apiKey == "" {
			slog.Warn("API key missing")
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized: API key required")
			return
		}

		validKey, err := apikeyManager.ValidateAPIKey(apiKey)
		if err != nil {
			slog.Warn("Invalid API key used", "key_prefix", keyPrefix(apiKey), "error", err)
			huma.WriteErr(api, ctx, http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %s", err.Error()))
			return
		}

		slog.Debug("Authenticated API key", "name", validKey.Name, "key_prefix", keyPrefix(validKey.Key))
		next(ctx)
	}
}

// operationRequiresAuth checks if the operation has our security scheme in its security requirements.
func operationRequiresAuth(op *huma.Operation) bool {
	for _, secReq := range op.Security {
		if _, ok := secReq[SecurityScheme]; ok {
			return true
		}
	}
	return false
}

// keyPrefix returns the first 4 characters of a key for safe logging.
func keyPrefix(key string) string {
	if len(key) >= 4 {
		return key[:4]
	}
	return key
}
