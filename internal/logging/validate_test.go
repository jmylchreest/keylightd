package logging

import (
	"testing"
	"time"

	logfilter "github.com/jmylchreest/slog-logfilter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFilters_ValidFilters(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "internal/*", Level: "debug", Enabled: true},
		{Type: "source:function", Pattern: "(*Server).Start", Level: "debug", Enabled: true},
		{Type: "context:request_id", Pattern: "*", Level: "debug", Enabled: true},
		{Type: "component", Pattern: "discovery*", Level: "warn", Enabled: true},
		{Type: "module", Pattern: "*http*", Level: "info", OutputLevel: "debug", Enabled: true},
		{Type: "source:file", Pattern: "*.go", Level: "error", Enabled: true, ExpiresAt: &future},
	}

	errs := ValidateFilters(filters)
	assert.Empty(t, errs)
}

func TestValidateFilters_EmptySlice(t *testing.T) {
	errs := ValidateFilters(nil)
	assert.Empty(t, errs)

	errs = ValidateFilters([]logfilter.LogFilter{})
	assert.Empty(t, errs)
}

func TestValidateFilters_EmptyType(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "", Pattern: "*", Level: "info", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, 0, errs[0].Index)
	assert.Equal(t, "type", errs[0].Field)
	assert.Contains(t, errs[0].Message, "must not be empty")
}

func TestValidateFilters_UnknownSourceType(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "source:line", Pattern: "*", Level: "info", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "type", errs[0].Field)
	assert.Contains(t, errs[0].Message, "unknown type")
}

func TestValidateFilters_EmptyContextKey(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "context:", Pattern: "*", Level: "info", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "type", errs[0].Field)
	assert.Contains(t, errs[0].Message, "unknown type")
}

func TestValidateFilters_EmptyPattern(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "", Level: "info", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "pattern", errs[0].Field)
	assert.Contains(t, errs[0].Message, "must not be empty")
}

func TestValidateFilters_EmptyLevel(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "level", errs[0].Field)
	assert.Contains(t, errs[0].Message, "must not be empty")
}

func TestValidateFilters_InvalidLevel(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "trace", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "level", errs[0].Field)
	assert.Contains(t, errs[0].Message, "invalid level")
}

func TestValidateFilters_InvalidOutputLevel(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "info", OutputLevel: "verbose", Enabled: true},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "output_level", errs[0].Field)
	assert.Contains(t, errs[0].Message, "invalid output_level")
}

func TestValidateFilters_ExpiredFilter(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "info", Enabled: true, ExpiresAt: &past},
	}
	errs := ValidateFilters(filters)
	require.Len(t, errs, 1)
	assert.Equal(t, "expires_at", errs[0].Field)
	assert.Contains(t, errs[0].Message, "in the past")
}

func TestValidateFilters_MultipleErrors(t *testing.T) {
	filters := []logfilter.LogFilter{
		{Type: "", Pattern: "", Level: "", Enabled: true},
		{Type: "source:file", Pattern: "*", Level: "info", OutputLevel: "bad", Enabled: true},
	}
	errs := ValidateFilters(filters)
	// First filter: type, pattern, level = 3 errors
	// Second filter: output_level = 1 error
	assert.Len(t, errs, 4)
}

func TestValidateFilters_WarningLevel(t *testing.T) {
	// "warning" is accepted as an alias for "warn"
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "warning", Enabled: true},
	}
	errs := ValidateFilters(filters)
	assert.Empty(t, errs)
}

func TestValidateFilters_CaseInsensitiveLevel(t *testing.T) {
	// Levels should be validated case-insensitively
	filters := []logfilter.LogFilter{
		{Type: "source:file", Pattern: "*", Level: "DEBUG", Enabled: true},
		{Type: "source:file", Pattern: "*", Level: "Info", Enabled: true},
		{Type: "source:file", Pattern: "*", Level: "WARN", Enabled: true},
		{Type: "source:file", Pattern: "*", Level: "Error", Enabled: true},
	}
	errs := ValidateFilters(filters)
	assert.Empty(t, errs)
}

func TestFormatErrors_Empty(t *testing.T) {
	assert.Equal(t, "", FormatErrors(nil))
	assert.Equal(t, "", FormatErrors([]FilterError{}))
}

func TestFormatErrors_Single(t *testing.T) {
	errs := []FilterError{{Index: 0, Field: "type", Message: "must not be empty"}}
	result := FormatErrors(errs)
	assert.Equal(t, "filter[0].type: must not be empty", result)
}

func TestFormatErrors_Multiple(t *testing.T) {
	errs := []FilterError{
		{Index: 0, Field: "type", Message: "must not be empty"},
		{Index: 1, Field: "level", Message: "invalid level"},
	}
	result := FormatErrors(errs)
	assert.Contains(t, result, "filter[0].type: must not be empty")
	assert.Contains(t, result, "filter[1].level: invalid level")
	assert.Contains(t, result, "; ")
}

func TestFilterError_Error(t *testing.T) {
	e := &FilterError{Index: 2, Field: "pattern", Message: "must not be empty"}
	assert.Equal(t, "filter[2].pattern: must not be empty", e.Error())
}

func TestIsValidFilterType(t *testing.T) {
	tests := []struct {
		name  string
		typ   string
		valid bool
	}{
		{"source:file", "source:file", true},
		{"source:function", "source:function", true},
		{"context:key", "context:request_id", true},
		{"context:empty key", "context:", false},
		{"plain attribute", "component", true},
		{"plain attribute with dots", "http.method", true},
		{"unknown source prefix", "source:line", false},
		{"unknown source prefix 2", "source:package", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidFilterType(tt.typ))
		})
	}
}
