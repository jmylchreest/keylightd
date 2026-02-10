package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/danielgtaylor/huma/v2"
	logfilter "github.com/jmylchreest/slog-logfilter"

	"github.com/jmylchreest/keylightd/internal/logging"
	"github.com/jmylchreest/keylightd/internal/utils"
)

// --- Log Filter types ---

// LogFilterResponse is the API representation of a log filter.
type LogFilterResponse struct {
	Type        string     `json:"type" doc:"Filter type: source:file, source:function, context:<key>, or a plain attribute key"`
	Pattern     string     `json:"pattern" doc:"Glob pattern for matching (exact, prefix*, *suffix, *contains*)"`
	Level       string     `json:"level" doc:"Minimum log level threshold (debug, info, warn, error)"`
	OutputLevel string     `json:"output_level,omitempty" doc:"Optional output level transformation"`
	Enabled     bool       `json:"enabled" doc:"Whether the filter is active"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" doc:"Optional expiration time (nil = never)"`
}

// --- List Filters ---

// ListFiltersInput is the input for listing log filters.
type ListFiltersInput struct{}

// ListFiltersOutput is the output for listing log filters.
type ListFiltersOutput struct {
	Body struct {
		Level   string              `json:"level" doc:"Current global log level"`
		Filters []LogFilterResponse `json:"filters" doc:"Active log filters"`
	}
}

// --- Set Filters ---

// SetFiltersInput is the input for replacing all log filters.
type SetFiltersInput struct {
	Body struct {
		Filters []LogFilterResponse `json:"filters" doc:"New filter list to apply" required:"true"`
	}
}

// SetFiltersOutput is the output after replacing filters.
type SetFiltersOutput struct {
	Body struct {
		Level   string              `json:"level" doc:"Current global log level"`
		Filters []LogFilterResponse `json:"filters" doc:"Applied log filters"`
	}
}

// --- Set Level ---

// SetLevelInput is the input for changing the global log level.
type SetLevelInput struct {
	Body struct {
		Level string `json:"level" doc:"New log level (debug, info, warn, error)" minLength:"1"`
	}
}

// SetLevelOutput is the output after changing the log level.
type SetLevelOutput struct {
	Body struct {
		Level string `json:"level" doc:"Updated global log level"`
	}
}

// LoggingHandler implements logging management HTTP handlers.
type LoggingHandler struct {
	Logger *slog.Logger
}

// ListFilters returns the current log level and active filters.
func (h *LoggingHandler) ListFilters(_ context.Context, _ *ListFiltersInput) (*ListFiltersOutput, error) {
	filters := logfilter.GetFilters()
	level := logfilter.GetLevel()

	out := &ListFiltersOutput{}
	out.Body.Level = LevelToString(level)
	out.Body.Filters = filtersToResponse(filters)
	return out, nil
}

// SetFilters validates and replaces all active log filters.
func (h *LoggingHandler) SetFilters(_ context.Context, input *SetFiltersInput) (*SetFiltersOutput, error) {
	newFilters := responseToFilters(input.Body.Filters)

	// Validate before applying
	if errs := logging.ValidateFilters(newFilters); len(errs) > 0 {
		return nil, huma.Error400BadRequest(
			fmt.Sprintf("Invalid filters: %s", logging.FormatErrors(errs)))
	}

	logfilter.SetFilters(newFilters)
	h.Logger.Info("Log filters updated via API", "count", len(newFilters))

	out := &SetFiltersOutput{}
	out.Body.Level = LevelToString(logfilter.GetLevel())
	out.Body.Filters = filtersToResponse(logfilter.GetFilters())
	return out, nil
}

// SetLevel validates and changes the global log level at runtime.
func (h *LoggingHandler) SetLevel(_ context.Context, input *SetLevelInput) (*SetLevelOutput, error) {
	validated := utils.ValidateLogLevel(input.Body.Level)
	if validated != input.Body.Level {
		return nil, huma.Error400BadRequest(
			fmt.Sprintf("Invalid log level %q; must be debug, info, warn, or error", input.Body.Level))
	}

	newLevel := utils.GetLogLevel(validated)
	logfilter.SetLevel(newLevel)
	h.Logger.Info("Log level changed via API", "level", validated)

	out := &SetLevelOutput{}
	out.Body.Level = validated
	return out, nil
}

// Ensure LoggingHandler implements the interface at compile time.
var _ LoggingHandlers = (*LoggingHandler)(nil)

// LoggingHandlers defines the interface for logging management operations.
type LoggingHandlers interface {
	ListFilters(ctx context.Context, input *ListFiltersInput) (*ListFiltersOutput, error)
	SetFilters(ctx context.Context, input *SetFiltersInput) (*SetFiltersOutput, error)
	SetLevel(ctx context.Context, input *SetLevelInput) (*SetLevelOutput, error)
}

// --- Conversion helpers ---

func filtersToResponse(filters []logfilter.LogFilter) []LogFilterResponse {
	result := make([]LogFilterResponse, len(filters))
	for i, f := range filters {
		result[i] = LogFilterResponse{
			Type:        f.Type,
			Pattern:     f.Pattern,
			Level:       f.Level,
			OutputLevel: f.OutputLevel,
			Enabled:     f.Enabled,
			ExpiresAt:   f.ExpiresAt,
		}
	}
	return result
}

func responseToFilters(resp []LogFilterResponse) []logfilter.LogFilter {
	result := make([]logfilter.LogFilter, len(resp))
	for i, r := range resp {
		result[i] = logfilter.LogFilter{
			Type:        r.Type,
			Pattern:     r.Pattern,
			Level:       r.Level,
			OutputLevel: r.OutputLevel,
			Enabled:     r.Enabled,
			ExpiresAt:   r.ExpiresAt,
		}
	}
	return result
}

// LevelToString converts a slog.Level to its string representation.
func LevelToString(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "debug"
	case level <= slog.LevelInfo:
		return "info"
	case level <= slog.LevelWarn:
		return "warn"
	default:
		return "error"
	}
}
