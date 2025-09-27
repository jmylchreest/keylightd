package apikey

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
)

// Manager handles API key business logic
// Concurrency contract:
//   - All mutations & persistence go through config.Config which encapsulates its own mutex.
//   - This manager does not layer additional locking; config methods (AddAPIKey, SetAPIKeyDisabledStatus, Save, etc.)
//     are assumed to be safe for concurrent invocation.
//   - Returned *config.APIKey pointers must be treated as read-only; callers should not modify fields directly.
//   - ValidateAPIKey performs a mutation (LastUsedAt) via config which handles synchronization.
//
// Future considerations:
//   - If additional in-memory (non-config) state is added here, introduce a dedicated RWMutex.
//   - Consider returning defensive copies if external mutation risk is observed in reviews.
//   - Add metrics hooks (e.g., validations, creations) once a metrics subsystem is introduced.
type Manager struct {
	cfg *config.Config
	log *slog.Logger
}

// NewManager creates a new APIKeyManager
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	m := &Manager{
		cfg: cfg,
		log: logger,
	}
	logger.Info("Loaded API keys from config", "count", len(cfg.State.APIKeys))
	return m
}

// CreateAPIKey generates a new API key, stores it, and saves the config.
func (m *Manager) CreateAPIKey(name string, expiresIn time.Duration) (*config.APIKey, error) {
	existingKeys := m.cfg.GetAPIKeys() // Returns []APIKey
	for _, existingKey := range existingKeys {
		if existingKey.Name == name {
			return nil, fmt.Errorf("API key with name '%s' already exists", name)
		}
	}

	keyString, err := config.GenerateKey(config.DefaultKeyLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key string: %w", err)
	}

	newKey := config.APIKey{
		Key:       keyString,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	if expiresIn > 0 {
		newKey.ExpiresAt = time.Now().UTC().Add(expiresIn)
	}

	if err := m.cfg.AddAPIKey(newKey); err != nil {
		return nil, fmt.Errorf("failed to add API key to config: %w", err)
	}

	// Save the configuration to persist the new key
	if err := m.cfg.Save(); err != nil {
		m.log.Error("failed to save config after adding API key", "name", name, "error", err)
		// Decide if we should revert the AddAPIKey or return an error indicating partial success.
		// For now, return the error, the key is in memory but not saved.
		return nil, fmt.Errorf("API key added to memory but failed to save to disk: %w", err)
	}

	m.log.Info("created API key and saved to config", "name", name, "key_prefix", newKey.Key[:4])
	return &newKey, nil
}

// ListAPIKeys returns all API keys.
func (m *Manager) ListAPIKeys() []config.APIKey { // No error returned by m.cfg.GetAPIKeys()
	return m.cfg.GetAPIKeys()
}

// DeleteAPIKey removes an API key and saves the config.
func (m *Manager) DeleteAPIKey(key string) error {
	if !m.cfg.DeleteAPIKey(key) { // DeleteAPIKey returns bool
		return fmt.Errorf("API key '%s' not found for deletion", key)
	}

	// Save the configuration to persist the deletion
	if err := m.cfg.Save(); err != nil {
		m.log.Error("failed to save config after deleting API key", "key_prefix", key[:4], "error", err)
		return fmt.Errorf("API key deleted from memory but failed to save to disk: %w", err)
	}
	m.log.Info("deleted API key and saved to config", "key_prefix", key[:4])
	return nil
}

// ValidateAPIKey checks if an API key is valid (exists, not disabled, not expired).
// Side effects: updates LastUsedAt on successful validation and persists the change (best-effort).
// Concurrency: underlying config access is internally locked; the returned pointer must be treated as read-only by callers.
func (m *Manager) ValidateAPIKey(key string) (*config.APIKey, error) {
	apiKey, found := m.cfg.FindAPIKey(key) // FindAPIKey returns (*APIKey, bool)
	if !found {
		return nil, fmt.Errorf("API key not found")
	}

	if apiKey.IsDisabled() {
		return nil, fmt.Errorf("API key is disabled")
	}

	if apiKey.IsExpired() {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update LastUsedAt timestamp
	if err := m.cfg.UpdateAPIKeyLastUsed(key, time.Now().UTC()); err != nil {
		m.log.Error("failed to update last used timestamp for API key in memory", "key", key, "error", err)
		// Do not save if in-memory update failed, but the key is still valid for this request.
		return apiKey, nil // Return original apiKey data before failed update
	}

	// Save the configuration to persist the LastUsedAt update
	if err := m.cfg.Save(); err != nil {
		m.log.Error("failed to save config after updating API key LastUsedAt", "key", key, "error", err)
		// Even if save fails, the key was validated and LastUsedAt is updated in memory for this session.
		// The next validation might hit the old LastUsedAt if the daemon restarts before a successful save.
	}

	// FindAPIKey returns a pointer to the key in the slice. To avoid the caller modifying it directly
	// after validation (if they hold onto the pointer), it might be safer to return a copy.
	// However, for now, we return the direct pointer as the config methods are managing persistence.
	// If LastUsedAt was updated, apiKey pointer already reflects this for the current request.
	return apiKey, nil
}

// SetAPIKeyDisabledStatus updates the disabled status of an API key and saves the config.
func (m *Manager) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (*config.APIKey, error) {
	updatedKey, err := m.cfg.SetAPIKeyDisabledStatus(keyOrName, disabled) // This modifies in-memory
	if err != nil {
		m.log.Error("failed to set API key disabled status in config memory", "key_or_name", keyOrName, "disabled", disabled, "error", err)
		return nil, err
	}

	// Save the configuration to persist the status change
	if err := m.cfg.Save(); err != nil {
		m.log.Error("failed to save config after setting API key disabled status", "key_or_name", keyOrName, "error", err)
		return nil, fmt.Errorf("API key status updated in memory but failed to save to disk: %w", err)
	}
	m.log.Info("set API key disabled status and saved to config", "key_or_name", keyOrName, "disabled", disabled)
	return updatedKey, nil
}
