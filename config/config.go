package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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

func getConfigPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "keylightd", "keylightd.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keylightd", "keylightd.yaml")
}

func getConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "keylightd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "keylightd")
}

// Config represents the application configuration
type Config struct {
	// Server configuration
	Server struct {
		UnixSocket string `mapstructure:"unix_socket"`
	} `mapstructure:"server"`

	// Discovery configuration
	Discovery struct {
		Interval int `mapstructure:"interval"`
	} `mapstructure:"discovery"`

	// Logging configuration
	Logging struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"logging"`
}

// Load loads the configuration from file and environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set default values using XDG helpers
	v.SetDefault("server.unix_socket", getRuntimeSocketPath())
	v.SetDefault("discovery.interval", 30) // Default to 30 seconds (minimum 5 seconds)
	v.SetDefault("logging.level", "info")

	// Set config file locations (XDG first)
	v.SetConfigFile(getConfigPath())
	v.AddConfigPath(getConfigDir())
	v.AddConfigPath("/etc/keylightd")
	v.AddConfigPath(".")

	// Read config file
	_ = v.ReadInConfig() // ignore not found, use defaults

	// Bind environment variables (override config file)
	v.SetEnvPrefix("KEYLIGHTD")
	v.AutomaticEnv()

	// Explicitly bind env vars for known fields
	_ = v.BindEnv("server.unix_socket", "KEYLIGHTD_UNIX_SOCKET")
	_ = v.BindEnv("discovery.interval", "KEYLIGHTD_DISCOVERY_INTERVAL")
	_ = v.BindEnv("logging.level", "KEYLIGHTD_LOGGING_LEVEL")

	// Create config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(configDir, "keylightd.yaml"))
	v.Set("server", c.Server)
	v.Set("discovery", c.Discovery)
	v.Set("logging", c.Logging)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
