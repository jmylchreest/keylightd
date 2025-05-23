package config

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Using constants defined in constants.go

// APIKey holds the information for an API authentication key.
type APIKey struct {
	Key        string    `json:"key" yaml:"key"`                   // The API key string (secret)
	Name       string    `json:"name" yaml:"name"`                 // A user-friendly name for the key
	CreatedAt  time.Time `json:"created_at" yaml:"created_at"`     // Timestamp of when the key was created
	ExpiresAt  time.Time `json:"expires_at" yaml:"expires_at"`     // Timestamp of when the key expires (zero value means never)
	LastUsedAt time.Time `json:"last_used_at" yaml:"last_used_at"` // Timestamp of when the key was last used (zero value means never)
	disabled   bool      // If true, the key is disabled
}

// IsExpired checks if the API key has expired.
func (ak *APIKey) IsExpired() bool {
	if ak.ExpiresAt.IsZero() {
		return false // Never expires
	}
	return time.Now().After(ak.ExpiresAt)
}

// IsDisabled checks if the API key is disabled.
func (ak *APIKey) IsDisabled() bool {
	return ak.disabled
}

// GenerateKey creates a new random API key string.
func GenerateKey(length int) (string, error) {
	if length <= 0 {
		length = DefaultKeyLength
	}
	b := make([]byte, length)
	// crypto/rand.Read is preferred for cryptographic security
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	for i, v := range b {
		b[i] = DefaultKeyCharset[int(v)%len(DefaultKeyCharset)]
	}
	return string(b), nil
}

// XDG helpers
// Using path utility functions from utils.go

// State holds persistent data like API keys and groups
type State struct {
	APIKeys []APIKey          `yaml:"api_keys"`
	Groups  map[string]any `yaml:"groups"`
}

// ConfigBlock holds operational/configuration settings
type ConfigBlock struct {
	Server    ServerConfig    `yaml:"server"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	Logging   LoggingConfig   `yaml:"logging"`
	API       APIConfig       `yaml:"api"`
}

// Config represents the application configuration (top-level)
type Config struct {
	State  State       `yaml:"state"`
	Config ConfigBlock `yaml:"config"`

	v         *viper.Viper
	saveMutex sync.Mutex `mapstructure:"-" yaml:"-"`
}

// APIConfig represents the API specific configuration
type APIConfig struct {
	ListenAddress string   `mapstructure:"listen_address" yaml:"listen_address"`
	APIKeys       []APIKey `mapstructure:"api_keys" yaml:"api_keys"`
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	UnixSocket string `mapstructure:"unix_socket" yaml:"unix_socket"`
}

// DiscoveryConfig represents the discovery configuration
type DiscoveryConfig struct {
	Interval        int `mapstructure:"interval" yaml:"interval"`
	CleanupInterval int `mapstructure:"cleanup_interval" yaml:"cleanup_interval"`
	CleanupTimeout  int `mapstructure:"cleanup_timeout" yaml:"cleanup_timeout"`
}

// LoggingConfig represents the logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
}

// New creates a new Config with the given viper instance
func New(v *viper.Viper) *Config {
	return &Config{v: v}
}

// Load loads configuration from a file and environment variables
func Load(configName, configFile string) (*Config, error) {
	v := viper.New()
	v.SetConfigName(configName)
	v.SetConfigType("yaml")

	// Set default values
	v.SetDefault("config.server.unix_socket", GetRuntimeSocketPath())
	v.SetDefault("config.discovery.interval", int(DefaultDiscoveryInterval.Seconds()))
	v.SetDefault("config.logging.level", LogLevelInfo)
	v.SetDefault("config.logging.format", LogFormatText)
	v.SetDefault("config.discovery.cleanup_interval", int(DefaultCleanupInterval.Seconds()))
	v.SetDefault("config.discovery.cleanup_timeout", int(DefaultStateTimeout.Seconds()))
	v.SetDefault("config.api.listen_address", DefaultAPIListenAddress)
	v.SetDefault("state.api_keys", []APIKey{})

	// Add config paths
	if configFile != "" {
		v.SetConfigFile(configFile)
		slog.Info("Using config file from command line", "path", configFile)
	} else {
		configPath := GetConfigPath(configName)
		v.SetConfigFile(configPath)
	}

	v.SetEnvPrefix("KEYLIGHT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			slog.Debug("No config file found, using defaults")
		} else {
			return nil, err // Do not wrap, return as-is
		}
	}

	cfg := New(v)
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Viper seems to fill map[string]any fine, but doesnt seem reliable at all for []APIKey?
	configPath := v.ConfigFileUsed()
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			var raw map[string]any
			if err := yaml.Unmarshal(data, &raw); err == nil {
				if stateRaw, ok := raw["state"]; ok {
					stateBytes, _ := yaml.Marshal(stateRaw)
					yaml.Unmarshal(stateBytes, &cfg.State)
				}
			}
		}
	}

	// Enforce critical defaults after unmarshal
	if cfg.Config.Server.UnixSocket == "" {
		cfg.Config.Server.UnixSocket = GetRuntimeSocketPath()
	}
	if cfg.Config.Discovery.Interval == 0 {
		cfg.Config.Discovery.Interval = int(DefaultDiscoveryInterval.Seconds())
	} else {
		cfg.Config.Discovery.Interval = ValidateDiscoveryInterval(cfg.Config.Discovery.Interval)
	}
	if cfg.Config.Discovery.CleanupInterval == 0 {
		cfg.Config.Discovery.CleanupInterval = int(DefaultCleanupInterval.Seconds())
	}
	if cfg.Config.Discovery.CleanupTimeout == 0 {
		cfg.Config.Discovery.CleanupTimeout = int(DefaultStateTimeout.Seconds())
	}
	if cfg.Config.API.ListenAddress == "" {
		cfg.Config.API.ListenAddress = DefaultAPIListenAddress
	}
	// Use default values if logging configuration is invalid
	if cfg.Config.Logging.Level != LogLevelDebug && cfg.Config.Logging.Level != LogLevelInfo && 
	   cfg.Config.Logging.Level != LogLevelWarn && cfg.Config.Logging.Level != LogLevelError {
		cfg.Config.Logging.Level = LogLevelInfo
	}
	
	if cfg.Config.Logging.Format != LogFormatText && cfg.Config.Logging.Format != LogFormatJSON {
		cfg.Config.Logging.Format = LogFormatText
	}
	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()

	logger := slog.Default()
	logger.Debug("Saving configuration", "path", c.v.ConfigFileUsed())

	settings := map[string]any{}

	// Only write state if api_keys or groups are non-empty
	stateMap := map[string]any{}
	if len(c.State.APIKeys) > 0 {
		stateMap["api_keys"] = c.State.APIKeys
	}
	if len(c.State.Groups) > 0 {
		stateMap["groups"] = c.State.Groups
	}
	if len(stateMap) > 0 {
		settings["state"] = stateMap
	}

	// Only write config if any sub-block is non-default
	configMap := map[string]any{}
	if !isDefaultServer(c.Config.Server) {
		configMap["server"] = c.Config.Server
	}
	if !isDefaultDiscovery(c.Config.Discovery) {
		configMap["discovery"] = c.Config.Discovery
	}
	if !isDefaultLogging(c.Config.Logging) {
		configMap["logging"] = c.Config.Logging
	}
	if c.Config.API.ListenAddress != DefaultAPIListenAddress {
		configMap["api"] = c.Config.API
	}
	if len(configMap) > 0 {
		settings["config"] = configMap
	}

	data, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("error marshaling config to YAML: %w", err)
	}
	configPath := c.v.ConfigFileUsed()
	if configPath == "" {
		return fmt.Errorf("no config file path set for saving")
	}

	// Create the directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("error creating config directory %s: %w", configDir, err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	logger.Debug("Configuration saved successfully", "path", configPath)
	return nil
}

func isDefaultServer(s ServerConfig) bool {
	return s.UnixSocket == GetRuntimeSocketPath()
}

func isDefaultDiscovery(d DiscoveryConfig) bool {
	return d.Interval == 30 && d.CleanupInterval == 60 && d.CleanupTimeout == 180
}

func isDefaultLogging(l LoggingConfig) bool {
	return l.Level == LogLevelInfo && l.Format == LogFormatText
}

// Get retrieves a value from the configuration
func (c *Config) Get(key string) any {
	if c.v == nil {
		return nil
	}
	return c.v.Get(key)
}

// Set sets a value in the configuration
func (c *Config) Set(key string, value any) {
	if c.v == nil {
		return
	}
	c.v.Set(key, value)
}

// GetAPIKeys returns a copy of the API keys
func (c *Config) GetAPIKeys() []APIKey {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	keys := make([]APIKey, len(c.State.APIKeys))
	copy(keys, c.State.APIKeys)
	return keys
}

// SetAPIKeys sets the API keys in the configuration.
func (c *Config) SetAPIKeys(keys []APIKey) {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	c.State.APIKeys = keys
}

// AddAPIKey adds a new API key to the configuration.
func (c *Config) AddAPIKey(newKey APIKey) error {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	for _, existingKey := range c.State.APIKeys {
		if existingKey.Key == newKey.Key || existingKey.Name == newKey.Name {
			return fmt.Errorf("API key with key '%s' or name '%s' already exists", newKey.Key, newKey.Name)
		}
	}
	c.State.APIKeys = append(c.State.APIKeys, newKey)
	return nil
}

// DeleteAPIKey removes an API key from the configuration by its key string.
func (c *Config) DeleteAPIKey(keyString string) bool {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	originalLen := len(c.State.APIKeys)
	filteredKeys := []APIKey{}
	for _, k := range c.State.APIKeys {
		if k.Key != keyString {
			filteredKeys = append(filteredKeys, k)
		}
	}
	c.State.APIKeys = filteredKeys
	return len(c.State.APIKeys) < originalLen
}

// FindAPIKey retrieves an API key by its key string.
func (c *Config) FindAPIKey(keyString string) (*APIKey, bool) {
	for i, k := range c.State.APIKeys {
		if k.Key == keyString {
			return &c.State.APIKeys[i], true
		}
	}
	return nil, false
}

// UpdateAPIKeyLastUsed updates the LastUsedAt field for a given API key.
func (c *Config) UpdateAPIKeyLastUsed(keyString string, lastUsedTime time.Time) error {
	found := false
	for i, apiKey := range c.State.APIKeys {
		if apiKey.Key == keyString {
			c.State.APIKeys[i].LastUsedAt = lastUsedTime
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("API key '%s' not found for updating last used time", keyString)
	}
	return nil
}

// SetAPIKeyDisabledStatus updates the disabled status of an API key.
func (c *Config) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (*APIKey, error) {
	var targetKey *APIKey
	targetIndex := -1

	for i, apiKey := range c.State.APIKeys {
		if apiKey.Key == keyOrName || apiKey.Name == keyOrName {
			targetKey = &c.State.APIKeys[i]
			targetIndex = i
			break
		}
	}

	if targetKey == nil {
		return nil, fmt.Errorf("API key '%s' not found", keyOrName)
	}

	c.State.APIKeys[targetIndex].disabled = disabled

	return targetKey, nil
}
