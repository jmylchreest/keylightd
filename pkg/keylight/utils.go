package keylight

import (
	"log"
	"log/slog"
	"strings"
)

// boolToInt converts a bool to int (true=1, false=0)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// convertTemperatureToDevice converts Kelvin to device mireds
func convertTemperatureToDevice(kelvin int) int {
	if kelvin < 2900 {
		kelvin = 2900
	} else if kelvin > 7000 {
		kelvin = 7000
	}
	mireds := 1000000 / kelvin
	if mireds > 344 {
		mireds = 344
	} else if mireds < 143 {
		mireds = 143
	}
	return mireds
}

// convertDeviceToTemperature converts device mireds to Kelvin
func convertDeviceToTemperature(mireds int) int {
	if mireds < 143 {
		mireds = 143
	} else if mireds > 344 {
		mireds = 344
	}
	return 1000000 / mireds
}

// slogToStdLogger bridges slog.Logger to log.Logger
func slogToStdLogger(s *slog.Logger) *log.Logger {
	return log.New(slogWriter{s}, "", 0)
}

type slogWriter struct {
	logger *slog.Logger
}

func (w slogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	msg = strings.TrimSpace(msg) // Remove leading/trailing whitespace

	level := slog.LevelInfo

	// Parse [LEVEL] prefix
	if strings.HasPrefix(msg, "[INFO]") {
		msg = strings.TrimPrefix(msg, "[INFO]")
		level = slog.LevelInfo
	} else if strings.HasPrefix(msg, "[WARN]") {
		msg = strings.TrimPrefix(msg, "[WARN]")
		level = slog.LevelWarn
	} else if strings.HasPrefix(msg, "[ERROR]") {
		msg = strings.TrimPrefix(msg, "[ERROR]")
		level = slog.LevelError
	}
	msg = strings.TrimSpace(msg)

	// Log at the correct level
	switch level {
	case slog.LevelInfo:
		w.logger.Info(msg)
	case slog.LevelWarn:
		w.logger.Warn(msg)
	case slog.LevelError:
		w.logger.Error(msg)
	default:
		w.logger.Info(msg)
	}
	return len(p), nil
}
