package config

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

const (
	// DefaultKeyLength is the default length for generated API keys.
	DefaultKeyLength = 32
	// DefaultKeyCharset is the characters used for API key generation.
	DefaultKeyCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// APIKey holds the information for an API authentication key.
type APIKey struct {
	Key         string    `json:"key" yaml:"key"`                   // The API key string (secret)
	Name        string    `json:"name" yaml:"name"`                 // A user-friendly name for the key
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`     // Timestamp of when the key was created
	ExpiresAt   time.Time `json:"expires_at" yaml:"expires_at"`     // Timestamp of when the key expires (zero value means never)
	LastUsedAt  time.Time `json:"last_used_at" yaml:"last_used_at"` // Timestamp of when the key was last used (zero value means never)
	disabled    bool      `json:"disabled" yaml:"disabled"`         // If true, the key is disabled
	Permissions []string  `json:"permissions" yaml:"permissions"`   // Future use: list of permissions
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
	for i := 0; i < length; i++ {
		b[i] = DefaultKeyCharset[int(b[i])%len(DefaultKeyCharset)]
	}
	return string(b), nil
}

// XDG helpers
func GetRuntimeSocketPath() string {
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
	API       APIConfig

	// Groups configuration
	Groups map[string]interface{} `mapstructure:"groups" yaml:"groups"`

	// Internal viper instance
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
	v.SetDefault("server.unix_socket", GetRuntimeSocketPath())
	v.SetDefault("discovery.interval", 30)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
	v.SetDefault("discovery.cleanup_interval", 60)
	v.SetDefault("discovery.cleanup_timeout", 180)
	v.SetDefault("api.listen_address", ":9123")
	v.SetDefault("api.api_keys", []APIKey{})

	// Add config paths
	if configFile != "" {
		v.SetConfigFile(configFile)
		slog.Info("Using config file from command line", "path", configFile)
	} else {
		configPath := getConfigPath(configName)
		v.SetConfigFile(configPath)
	}

	// Set up environment variable binding
	v.SetEnvPrefix("KEYLIGHT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file - ignore if it doesn't exist
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			// Config file not found is not an error, just use defaults
			slog.Debug("No config file found, using defaults")
		} else {
			return nil, err // Do not wrap, return as-is
		}
	}

	// Create config struct
	cfg := New(v)

	// Create a custom decoder config
	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:  cfg,
		TagName: "yaml",
	}

	// Create a new decoder
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating decoder: %w", err)
	}

	// Decode the config
	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to file
func (c *Config) Save() error {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()

	logger := slog.Default()
	logger.Info("Saving configuration", "path", c.v.ConfigFileUsed())

	// Update viper with current values
	c.v.Set("server", c.Server)
	c.v.Set("discovery", c.Discovery)
	c.v.Set("logging", c.Logging)
	c.v.Set("api", c.API)
	if c.Groups != nil {
		logger.Debug("Setting groups in config", "groups", c.Groups)
		c.v.Set("groups", c.Groups)
	}

	// Write config - Viper will create the file if it doesn't exist
	if err := c.v.WriteConfig(); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	logger.Info("Configuration saved successfully", "path", c.v.ConfigFileUsed())
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

// GetAPIKeys returns a copy of the API keys
func (c *Config) GetAPIKeys() []APIKey {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	keys := make([]APIKey, len(c.API.APIKeys))
	copy(keys, c.API.APIKeys)
	return keys
}

// SetAPIKeys sets the API keys in the configuration.
func (c *Config) SetAPIKeys(keys []APIKey) {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	c.API.APIKeys = keys
}

// AddAPIKey adds a new API key to the configuration.
func (c *Config) AddAPIKey(newKey APIKey) error {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	for _, existingKey := range c.API.APIKeys {
		if existingKey.Key == newKey.Key || existingKey.Name == newKey.Name {
			return fmt.Errorf("API key with key '%s' or name '%s' already exists", newKey.Key, newKey.Name)
		}
	}
	c.API.APIKeys = append(c.API.APIKeys, newKey)
	return nil
}

// DeleteAPIKey removes an API key from the configuration by its key string.
func (c *Config) DeleteAPIKey(keyString string) bool {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	originalLen := len(c.API.APIKeys)
	filteredKeys := []APIKey{}
	for _, k := range c.API.APIKeys {
		if k.Key != keyString {
			filteredKeys = append(filteredKeys, k)
		}
	}
	c.API.APIKeys = filteredKeys
	return len(c.API.APIKeys) < originalLen
}

// FindAPIKey retrieves an API key by its key string.
func (c *Config) FindAPIKey(keyString string) (*APIKey, bool) {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()
	for i, k := range c.API.APIKeys {
		if k.Key == keyString {
			return &c.API.APIKeys[i], true
		}
	}
	return nil, false
}

// UpdateAPIKeyLastUsed updates the LastUsedAt field for a given API key.
func (c *Config) UpdateAPIKeyLastUsed(keyString string, lastUsedTime time.Time) error {
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()

	found := false
	for i, apiKey := range c.API.APIKeys {
		if apiKey.Key == keyString {
			c.API.APIKeys[i].LastUsedAt = lastUsedTime
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
	c.saveMutex.Lock()
	defer c.saveMutex.Unlock()

	var targetKey *APIKey
	targetIndex := -1

	for i, apiKey := range c.API.APIKeys {
		if apiKey.Key == keyOrName || apiKey.Name == keyOrName {
			targetKey = &c.API.APIKeys[i]
			targetIndex = i
			break
		}
	}

	if targetKey == nil {
		return nil, fmt.Errorf("API key '%s' not found", keyOrName)
	}

	c.API.APIKeys[targetIndex].disabled = disabled

	return targetKey, nil
}
