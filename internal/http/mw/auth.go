package mw

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jmylchreest/keylightd/internal/apikey"
)

// APIKeyAuth returns a Chi middleware that validates API keys on every request.
// It checks the Authorization: Bearer header first, then falls back to the
// X-API-Key header. This runs at the Chi router level so it covers all routes
// uniformly — both Huma-managed and raw handlers.
//
// The Huma security annotations in routes/ remain for OpenAPI documentation only.
func APIKeyAuth(logger *slog.Logger, apikeyManager *apikey.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from headers
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
