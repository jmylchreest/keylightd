package handlers

import (
	"log/slog"
	"testing"

	logfilter "github.com/jmylchreest/slog-logfilter"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLoggingUpdatesOnlyTheDaemonLogger(t *testing.T) {
	logger := logfilter.New(logfilter.WithLevel(slog.LevelInfo))
	other := logfilter.New(logfilter.WithLevel(slog.LevelInfo))
	handler := &LoggingHandler{Logger: logger}
	level := &SetLevelInput{}
	level.Body.Level = "debug"
	_, err := handler.SetLevel(t.Context(), level)
	require.NoError(t, err)

	input := &SetFiltersInput{}
	input.Body.Filters = []LogFilterResponse{{Type: "component", Pattern: "discovery", Level: "warn", Enabled: true}}
	_, err = handler.SetFilters(t.Context(), input)
	require.NoError(t, err)

	out, err := handler.ListFilters(t.Context(), &ListFiltersInput{})
	require.NoError(t, err)
	require.Equal(t, "debug", out.Body.Level)
	require.Equal(t, input.Body.Filters, out.Body.Filters)
	independent := other.Handler().(*logfilter.Handler)
	require.Equal(t, slog.LevelInfo, independent.GetLevel())
	require.Empty(t, independent.GetFilters())
}

func TestRuntimeLoggingUnavailable(t *testing.T) {
	handler := &LoggingHandler{Logger: slog.New(slog.DiscardHandler)}
	_, err := handler.ListFilters(t.Context(), &ListFiltersInput{})
	require.ErrorContains(t, err, "Runtime logging controls are unavailable")
	_, err = handler.SetFilters(t.Context(), &SetFiltersInput{})
	require.ErrorContains(t, err, "Runtime logging controls are unavailable")
	_, err = handler.SetLevel(t.Context(), &SetLevelInput{})
	require.ErrorContains(t, err, "Runtime logging controls are unavailable")
}
