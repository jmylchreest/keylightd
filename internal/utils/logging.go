package utils

import (
	"log/slog"
	"os"

	"github.com/jmylchreest/keylightd/internal/config"
	logfilter "github.com/jmylchreest/slog-logfilter"
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

// SetupLogger creates and returns a new logger backed by slog-logfilter.
// The logger supports runtime level changes and log filter hot-reload via
// the logfilter package-level functions (SetLevel, SetFilters, etc.).
func SetupLogger(level string, format string) *slog.Logger {
	validLevel := ValidateLogLevel(level)
	validFormat := ValidateLogFormat(format)
	logLevel := GetLogLevel(validLevel)

	return logfilter.New(
		logfilter.WithLevel(logLevel),
		logfilter.WithFormat(validFormat),
		logfilter.WithSource(true),
		logfilter.WithOutput(os.Stderr),
	)
}

// SetupLoggerWithFilters creates a logger with initial filters applied.
func SetupLoggerWithFilters(level string, format string, filters []logfilter.LogFilter) *slog.Logger {
	validLevel := ValidateLogLevel(level)
	validFormat := ValidateLogFormat(format)
	logLevel := GetLogLevel(validLevel)

	opts := []logfilter.Option{
		logfilter.WithLevel(logLevel),
		logfilter.WithFormat(validFormat),
		logfilter.WithSource(true),
		logfilter.WithOutput(os.Stderr),
	}

	if len(filters) > 0 {
		opts = append(opts, logfilter.WithFilters(filters))
	}

	return logfilter.New(opts...)
}

// SetupErrorLogger creates a simple text logger for reporting errors during startup.
// Uses slog-logfilter for consistency, but with error-only level.
func SetupErrorLogger() *slog.Logger {
	return logfilter.New(
		logfilter.WithLevel(slog.LevelError),
		logfilter.WithFormat("text"),
		logfilter.WithOutput(os.Stderr),
	)
}

// SetAsDefaultLogger sets a logger as the default logger
func SetAsDefaultLogger(logger *slog.Logger) {
	slog.SetDefault(logger)
}
