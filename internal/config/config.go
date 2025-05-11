package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// XDG helpers
func getRuntimeSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "keylightd.sock")
	}
	uid := os.Getuid()
	return filepath.Join("/run/user", strconv.Itoa(uid), "keylightd.sock")
}

func getConfigBaseDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "keylight")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keylight")
}

func getConfigPath(filename string) string {
	return filepath.Join(getConfigBaseDir(), filename)
}

// Config represents the application configuration
type Config struct {
	Server    ServerConfig
	Discovery DiscoveryConfig
	Logging   LoggingConfig

	// Groups configuration
	Groups map[string]interface{} `mapstructure:"groups"`

	// Internal viper instance
	v *viper.Viper
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	UnixSocket string
}

// DiscoveryConfig represents the discovery configuration
type DiscoveryConfig struct {
	Interval        int
	CleanupInterval int `mapstructure:"cleanup_interval"` // Interval for running cleanup worker in seconds
	CleanupTimeout  int `mapstructure:"cleanup_timeout"`  // Timeout for considering a light stale in seconds
}

// LoggingConfig represents the logging configuration
type LoggingConfig struct {
	Level  string
	Format string
}

// Load loads configuration from a file and environment variables
func Load(configName, configFile string) (*Config, error) {
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType("yaml")

	// Set default values
	v.SetDefault("server.unix_socket", getRuntimeSocketPath())
	v.SetDefault("discovery.interval", 30)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
	v.SetDefault("discovery.cleanup_interval", 60) // Default cleanup interval: 60 seconds
	v.SetDefault("discovery.cleanup_timeout", 180) // Default cleanup timeout: 180 seconds (3 minutes)

	// Add config paths
	if configFile != "" {
		v.SetConfigFile(configFile)
		slog.Info("Using config file from command line", "path", configFile)
	} else {
		configPath := getConfigPath(configName)
		v.SetConfigFile(configPath)

		// Create config directory if it doesn't exist
		configDir := getConfigBaseDir()
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("error creating config directory: %w", err)
		}

		// Only log if config file exists
		if _, err := os.Stat(configPath); err == nil {
			slog.Info("Using default config file", "path", configPath)
		}
	}

	// Read config file - Viper will use defaults if file not found
	v.ReadInConfig()

	// Bind environment variables
	v.SetEnvPrefix("KEYLIGHT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Create config struct
	cfg := &Config{
		Server: ServerConfig{
			UnixSocket: v.GetString("server.unix_socket"),
		},
		Discovery: DiscoveryConfig{
			Interval:        v.GetInt("discovery.interval"),
			CleanupInterval: v.GetInt("discovery.cleanup_interval"),
			CleanupTimeout:  v.GetInt("discovery.cleanup_timeout"),
		},
		Logging: LoggingConfig{
			Level:  v.GetString("logging.level"),
			Format: v.GetString("logging.format"),
		},
		v: v,
	}

	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save(filename string) error {
	logger := slog.Default()
	configDir := getConfigBaseDir()
	configPath := getConfigPath(filename)

	logger.Info("Saving configuration", "path", configPath)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	// Set config file path
	c.v.SetConfigFile(configPath)

	// Update viper with current values
	c.v.Set("server", c.Server)
	c.v.Set("discovery", c.Discovery)
	c.v.Set("logging", c.Logging)
	if c.Groups != nil {
		logger.Debug("Setting groups in config", "groups", c.Groups)
		c.v.Set("groups", c.Groups)
	}

	// Write config - Viper will create the file if it doesn't exist
	if err := c.v.WriteConfig(); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	logger.Info("Configuration saved successfully", "path", configPath)
	return nil
}

// Get retrieves a value from the configuration
func (c *Config) Get(key string) interface{} {
	if c.v == nil {
		return nil
	}
	return c.v.Get(key)
}

// Set sets a value in the configuration
func (c *Config) Set(key string, value interface{}) {
	if c.v == nil {
		return
	}
	c.v.Set(key, value)
}
