package utils

import "log/slog"

// LogLevel defines log level types
type LogLevel string

// Log level constants
const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
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