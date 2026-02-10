// Package logging provides filter validation and setup helpers for the
// keylightd structured logging system backed by slog-logfilter.
package logging

import (
	"fmt"
	"strings"
	"time"

	logfilter "github.com/jmylchreest/slog-logfilter"
)

// Known filter types that map to slog-logfilter's special prefixes.
var knownSpecialTypes = map[string]bool{
	"source:file":     true,
	"source:function": true,
}

// validLevels is the set of accepted log level strings.
var validLevels = map[string]bool{
	"debug":   true,
	"info":    true,
	"warn":    true,
	"warning": true,
	"error":   true,
}

// FilterError describes a single validation failure for a log filter.
type FilterError struct {
	Index   int    // Position in the filter slice
	Field   string // Which field failed validation
	Message string // Human-readable description
}

func (e *FilterError) Error() string {
	return fmt.Sprintf("filter[%d].%s: %s", e.Index, e.Field, e.Message)
}

// ValidateFilters checks a slice of LogFilter values and returns all
// validation errors found.  An empty error slice means all filters are valid.
func ValidateFilters(filters []logfilter.LogFilter) []FilterError {
	var errs []FilterError

	for i, f := range filters {
		// Type must be non-empty
		if f.Type == "" {
			errs = append(errs, FilterError{Index: i, Field: "type", Message: "must not be empty"})
		} else if !isValidFilterType(f.Type) {
			errs = append(errs, FilterError{Index: i, Field: "type",
				Message: fmt.Sprintf("unknown type %q; use source:file, source:function, context:<key>, or a plain attribute key", f.Type)})
		}

		// Pattern must be non-empty (empty pattern in slog-logfilter always returns false)
		if f.Pattern == "" {
			errs = append(errs, FilterError{Index: i, Field: "pattern", Message: "must not be empty"})
		}

		// Level must be a recognized level string
		if f.Level == "" {
			errs = append(errs, FilterError{Index: i, Field: "level", Message: "must not be empty"})
		} else if !validLevels[strings.ToLower(f.Level)] {
			errs = append(errs, FilterError{Index: i, Field: "level",
				Message: fmt.Sprintf("invalid level %q; must be debug, info, warn, or error", f.Level)})
		}

		// OutputLevel, if set, must also be valid
		if f.OutputLevel != "" && !validLevels[strings.ToLower(f.OutputLevel)] {
			errs = append(errs, FilterError{Index: i, Field: "output_level",
				Message: fmt.Sprintf("invalid output_level %q; must be debug, info, warn, or error", f.OutputLevel)})
		}

		// ExpiresAt, if set, must be in the future
		if f.ExpiresAt != nil && !f.ExpiresAt.IsZero() && f.ExpiresAt.Before(time.Now()) {
			errs = append(errs, FilterError{Index: i, Field: "expires_at",
				Message: "expiration time is in the past"})
		}
	}

	return errs
}

// isValidFilterType checks whether a filter type is acceptable.
// Accepted forms:
//   - "source:file", "source:function"        (known special types)
//   - "context:<key>"  where key is non-empty  (context extractors)
//   - any other non-empty string without ":"   (plain slog attribute key)
func isValidFilterType(t string) bool {
	if knownSpecialTypes[t] {
		return true
	}
	if strings.HasPrefix(t, "context:") {
		return len(t) > len("context:") // key part must be non-empty
	}
	// Reject unknown "prefix:" patterns to avoid silent misconfiguration.
	// source: and context: are the only supported prefixed types.
	if strings.HasPrefix(t, "source:") {
		return false // only source:file and source:function are valid
	}
	// Plain attribute key — allow anything non-empty (already checked by caller)
	return t != ""
}

// FormatErrors returns a human-readable summary of filter validation errors.
func FormatErrors(errs []FilterError) string {
	if len(errs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range errs {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e.Error())
	}
	return b.String()
}
