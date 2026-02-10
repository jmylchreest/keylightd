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
// It checks the operation's Security requirements to determine if auth is needed.
// Operations registered via PublicGet/HiddenGet have no Security set and pass through.
// Operations registered via ProtectedGet/ProtectedPost/etc. have the SecurityScheme
// set and require a valid API key.
//
// This approach naturally exempts Huma's auto-registered routes (/openapi.json,
// /docs, /schemas/) since they have no Security set on their operations.
func HumaAuth(api huma.API, logger *slog.Logger, apikeyManager *apikey.Manager) func(ctx huma.Context, next func(huma.Context)) {
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
		key := ctx.Header("Authorization")
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(key, bearerPrefix) {
			key = key[len(bearerPrefix):]
		} else {
			key = ctx.Header("X-API-Key")
		}

		if key == "" {
			logger.Warn("API key missing",
				"method", ctx.Method(),
				"path", ctx.URL().Path,
				"remote_addr", ctx.RemoteAddr(),
			)
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized: API key required")
			return
		}

		validKey, err := apikeyManager.ValidateAPIKey(key)
		if err != nil {
			logger.Warn("Invalid API key used",
				"key_prefix", keyPrefix(key),
				"error", err,
				"method", ctx.Method(),
				"path", ctx.URL().Path,
				"remote_addr", ctx.RemoteAddr(),
			)
			huma.WriteErr(api, ctx, http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %s", err.Error()))
			return
		}

		logger.Debug("Authenticated API key",
			"name", validKey.Name,
			"key_prefix", keyPrefix(validKey.Key),
		)
		next(ctx)
	}
}

// operationRequiresAuth checks if the operation has our security scheme
// in its security requirements.
func operationRequiresAuth(op *huma.Operation) bool {
	for _, secReq := range op.Security {
		if _, ok := secReq[SecurityScheme]; ok {
			return true
		}
	}
	return false
}

// RawAPIKeyAuth returns a Chi middleware for raw (non-Huma) handlers that need
// API key authentication. Used for endpoints like the 207 Multi-Status group
// state handler that bypass Huma's routing.
func RawAPIKeyAuth(logger *slog.Logger, apikeyManager *apikey.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Authorization")
			const bearerPrefix = "Bearer "
			if strings.HasPrefix(key, bearerPrefix) {
				key = key[len(bearerPrefix):]
			} else {
				key = r.Header.Get("X-API-Key")
			}

			if key == "" {
				logger.Warn("API key missing",
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
				return
			}

			validKey, err := apikeyManager.ValidateAPIKey(key)
			if err != nil {
				logger.Warn("Invalid API key used",
					"key_prefix", keyPrefix(key),
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, fmt.Sprintf("Unauthorized: %s", err.Error()), http.StatusUnauthorized)
				return
			}

			logger.Debug("Authenticated API key",
				"name", validKey.Name,
				"key_prefix", keyPrefix(validKey.Key),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// keyPrefix returns the first 4 characters of a key for safe logging.
func keyPrefix(key string) string {
	if len(key) >= 4 {
		return key[:4]
	}
	return key
}
