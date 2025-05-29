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
// It checks the user's runtime directory first, then falls back to system socket
func GetRuntimeSocketPath() string {
	userSocket := filepath.Join(GetRuntimeDir(), SocketFilename)
	
	// If user socket exists, use it
	if _, err := os.Stat(userSocket); err == nil {
		return userSocket
	}
	
	// Fall back to system socket path for systemd service
	systemSocket := filepath.Join("/run/keylightd", SocketFilename)
	if _, err := os.Stat(systemSocket); err == nil {
		return systemSocket
	}
	
	// Default to user socket path (original behavior)
	return userSocket
}

// GetConfigBaseDir returns the base directory for configuration files
func GetConfigBaseDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		// For system service, XDG_CONFIG_HOME is set to /etc/keylightd
		// so we return it directly without appending ConfigDirName
		if dir == "/etc/keylightd" {
			return dir
		}
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