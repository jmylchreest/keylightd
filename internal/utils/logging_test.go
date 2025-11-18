package utils

import (
	"log/slog"
	"testing"
)

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"unknown defaults to info", "unknown", slog.LevelInfo},
		{"empty defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLogLevel(tt.level)
			if result != tt.expected {
				t.Errorf("GetLogLevel(%q) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{"valid debug", "debug", "debug"},
		{"valid info", "info", "info"},
		{"valid warn", "warn", "warn"},
		{"valid error", "error", "error"},
		{"invalid defaults to info", "invalid", "info"},
		{"empty defaults to info", "", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateLogLevel(tt.level)
			if result != tt.expected {
				t.Errorf("ValidateLogLevel(%q) = %q, want %q", tt.level, result, tt.expected)
			}
		})
	}
}

func TestValidateLogFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{"valid text", "text", "text"},
		{"valid json", "json", "json"},
		{"invalid defaults to text", "invalid", "text"},
		{"empty defaults to text", "", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateLogFormat(tt.format)
			if result != tt.expected {
				t.Errorf("ValidateLogFormat(%q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"text logger with info", "info", "text"},
		{"json logger with debug", "debug", "json"},
		{"invalid level and format", "invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := SetupLogger(tt.level, tt.format)
			if logger == nil {
				t.Error("SetupLogger returned nil")
			}
		})
	}
}

func TestSetupErrorLogger(t *testing.T) {
	logger := SetupErrorLogger()
	if logger == nil {
		t.Error("SetupErrorLogger returned nil")
	}
}

func TestSetAsDefaultLogger(t *testing.T) {
	logger := SetupLogger("info", "text")
	// This should not panic
	SetAsDefaultLogger(logger)
}

func TestLogLevelConstants(t *testing.T) {
	// Verify constants match expected values
	if LogLevelDebug != "debug" {
		t.Errorf("LogLevelDebug = %q, want %q", LogLevelDebug, "debug")
	}
	if LogLevelInfo != "info" {
		t.Errorf("LogLevelInfo = %q, want %q", LogLevelInfo, "info")
	}
	if LogLevelWarn != "warn" {
		t.Errorf("LogLevelWarn = %q, want %q", LogLevelWarn, "warn")
	}
	if LogLevelError != "error" {
		t.Errorf("LogLevelError = %q, want %q", LogLevelError, "error")
	}
}

func TestLogFormatConstants(t *testing.T) {
	if LogFormatText != "text" {
		t.Errorf("LogFormatText = %q, want %q", LogFormatText, "text")
	}
	if LogFormatJSON != "json" {
		t.Errorf("LogFormatJSON = %q, want %q", LogFormatJSON, "json")
	}
}
