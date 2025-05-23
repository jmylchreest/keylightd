package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// GetRuntimeDir returns the XDG runtime directory
func GetRuntimeDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	uid := os.Getuid()
	return filepath.Join("/run/user", strconv.Itoa(uid))
}

// GetRuntimeSocketPath returns the full path to the Unix socket
func GetRuntimeSocketPath() string {
	return filepath.Join(GetRuntimeDir(), SocketFilename)
}

// GetConfigBaseDir returns the base directory for configuration files
func GetConfigBaseDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, ConfigDirName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", ConfigDirName)
}

// GetConfigPath returns the full path to a configuration file
func GetConfigPath(filename string) string {
	return filepath.Join(GetConfigBaseDir(), filename)
}

// GetDaemonConfigPath returns the full path to the daemon configuration file
func GetDaemonConfigPath() string {
	return GetConfigPath(DaemonConfigFilename)
}

// GetClientConfigPath returns the full path to the client configuration file
func GetClientConfigPath() string {
	return GetConfigPath(ClientConfigFilename)
}

// ValidateDiscoveryInterval validates and converts the discovery interval
// Returns the interval in seconds, clamped to the minimum allowed value
func ValidateDiscoveryInterval(intervalSeconds int) int {
	minSeconds := int(MinDiscoveryInterval.Seconds())
	if intervalSeconds < minSeconds {
		return minSeconds
	}
	return intervalSeconds
}