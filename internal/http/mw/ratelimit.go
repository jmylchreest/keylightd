package mw

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	// RequestsPerMinute is the maximum number of requests per minute per IP.
	RequestsPerMinute int
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 120,
	}
}

// RateLimitByIP returns a Chi middleware that rate limits by IP address.
func RateLimitByIP(cfg RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.RequestsPerMinute <= 0 {
		// No rate limiting
		return func(next http.Handler) http.Handler { return next }
	}
	return httprate.LimitByIP(cfg.RequestsPerMinute, time.Minute)
}
