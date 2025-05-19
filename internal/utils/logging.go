package utils

import (
	"log/slog"
	"os"
)

// LogLevel defines log level types
type LogLevel string

// Log level constants
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat defines log format types
type LogFormat string

// Log format constants
const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// GetLogLevel converts a string log level to slog.Level
func GetLogLevel(level string) slog.Level {
	switch level {
	case string(LogLevelDebug):
		return slog.LevelDebug
	case string(LogLevelInfo):
		return slog.LevelInfo
	case string(LogLevelWarn):
		return slog.LevelWarn
	case string(LogLevelError):
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SetupLogger creates and returns a new logger with the specified configuration
func SetupLogger(level string, format string) *slog.Logger {
	logLevel := GetLogLevel(level)
	var handler slog.Handler

	switch format {
	case string(LogFormatJSON):
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	default:
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	}

	return slog.New(handler)
}

// SetupErrorLogger creates a simple text logger for reporting errors during startup
func SetupErrorLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	return slog.New(handler)
}

// SetAsDefaultLogger sets a logger as the default logger
func SetAsDefaultLogger(logger *slog.Logger) {
	slog.SetDefault(logger)
}