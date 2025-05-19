package config

import "time"

// Common constants shared between daemon and client
const (
	// ConfigDirName is the name of the config directory within XDG_CONFIG_HOME
	ConfigDirName = "keylight"

	// DaemonConfigFilename is the base filename for daemon config
	DaemonConfigFilename = "keylightd.yaml"

	// ClientConfigFilename is the base filename for client config
	ClientConfigFilename = "keylightctl.yaml"

	// SocketFilename is the base filename for the Unix socket
	SocketFilename = "keylightd.sock"

	// DefaultKeyLength is the default length for generated API keys
	DefaultKeyLength = 32

	// DefaultKeyCharset is the characters used for API key generation
	DefaultKeyCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// DefaultAPIListenAddress is the default HTTP API listen address
	DefaultAPIListenAddress = ":9123"
)

// Default timeouts and intervals
const (
	// DefaultDiscoveryInterval is the default interval for mDNS discovery
	DefaultDiscoveryInterval = 30 * time.Second

	// DefaultCleanupInterval is the default interval for cleaning up stale lights
	DefaultCleanupInterval = 180 * time.Second

	// DefaultStateTimeout is the default timeout for considering a light stale
	DefaultStateTimeout = 180 * time.Second

	// MinDiscoveryInterval is the minimum allowed discovery interval
	MinDiscoveryInterval = 5 * time.Second
)

// Light constraints
const (
	// MinBrightness is the minimum allowed brightness value
	MinBrightness = 0

	// MaxBrightness is the maximum allowed brightness value
	MaxBrightness = 100

	// MinTemperature is the minimum allowed temperature value (in Kelvin)
	MinTemperature = 2900

	// MaxTemperature is the maximum allowed temperature value (in Kelvin)
	MaxTemperature = 7000
)

// Logging constants
const (
	// LogLevelDebug represents debug log level
	LogLevelDebug = "debug"

	// LogLevelInfo represents info log level
	LogLevelInfo = "info"

	// LogLevelWarn represents warning log level
	LogLevelWarn = "warn"

	// LogLevelError represents error log level
	LogLevelError = "error"

	// LogFormatText represents text log format
	LogFormatText = "text"

	// LogFormatJSON represents JSON log format
	LogFormatJSON = "json"
)
