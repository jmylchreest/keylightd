package utils

import (
	"github.com/jmylchreest/keylightd/internal/config"
	"log/slog"
	"os"
)

// LogLevel defines log level types
type LogLevel string

// Log level constants - using values from config package
const (
	LogLevelDebug LogLevel = LogLevel(config.LogLevelDebug)
	LogLevelInfo  LogLevel = LogLevel(config.LogLevelInfo)
	LogLevelWarn  LogLevel = LogLevel(config.LogLevelWarn)
	LogLevelError LogLevel = LogLevel(config.LogLevelError)
)

// LogFormat defines log format types
type LogFormat string

// Log format constants - using values from config package
const (
	LogFormatText LogFormat = LogFormat(config.LogFormatText)
	LogFormatJSON LogFormat = LogFormat(config.LogFormatJSON)
)

// GetLogLevel converts a string log level to slog.Level
func GetLogLevel(level string) slog.Level {
	switch level {
	case string(LogLevelDebug):
		return slog.LevelDebug
	case string(LogLevelWarn):
		return slog.LevelWarn
	case string(LogLevelError):
		return slog.LevelError
	case string(LogLevelInfo):
		fallthrough
	default:
		return slog.LevelInfo
	}
}

// ValidateLogLevel ensures the provided level is valid, returning a default if not
func ValidateLogLevel(level string) string {
	switch level {
	case string(LogLevelDebug), string(LogLevelInfo), string(LogLevelWarn), string(LogLevelError):
		return level
	default:
		return string(LogLevelInfo)
	}
}

// ValidateLogFormat ensures the provided format is valid, returning a default if not
func ValidateLogFormat(format string) string {
	switch format {
	case string(LogFormatText), string(LogFormatJSON):
		return format
	default:
		return string(LogFormatText)
	}
}

// SetupLogger creates and returns a new logger with the specified configuration
func SetupLogger(level string, format string) *slog.Logger {
	// Validate inputs internally
	validLevel := ValidateLogLevel(level)
	validFormat := ValidateLogFormat(format)
	
	logLevel := GetLogLevel(validLevel)
	var handler slog.Handler

	switch validFormat {
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