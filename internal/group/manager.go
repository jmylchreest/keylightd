package group

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Manager handles light group management
// Concurrency contract:
//   - All access to m.groups is protected by mu (RWMutex).
//   - Read methods (GetGroup, GetGroups, GetGroupsByName) acquire RLock.
//   - Mutating methods (CreateGroup, DeleteGroup, SetGroupLights, SetGroupState, SetGroupBrightness, SetGroupTemperature)
//     hold Lock only for in-memory modifications and release it before persistence.
//   - Persistence (saveGroups) snapshots groups under a read lock, then updates config & saves outside the write path.
//   - Returned *Group pointers must be treated as read-only by callers; mutating them directly risks data races.
//
// Future considerations:
// - Return defensive copies (DTOs) to avoid accidental external mutation.
// - Add batch operations with structured result reporting for partial failures.
type Manager struct {
	logger   *slog.Logger
	lights   keylight.LightManager
	groups   map[string]*Group
	mu       sync.RWMutex
	cfg      *config.Config
	eventBus *events.Bus
}

// Group represents a group of lights that can be controlled together
type Group struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Lights []string `json:"lights"` // Store light IDs instead of pointers
}

// MarshalJSON ensures that Lights is always marshaled as [] instead of null
func (g *Group) MarshalJSON() ([]byte, error) {
	type Alias Group
	tmp := &struct {
		*Alias
	}{
		Alias: (*Alias)(g),
	}
	if tmp.Lights == nil {
		tmp.Lights = []string{}
	}
	return json.Marshal(tmp)
}

// SetEventBus sets the event bus for publishing group change events.
func (m *Manager) SetEventBus(bus *events.Bus) {
	m.eventBus = bus
}

// emit publishes an event if an event bus is configured.
func (m *Manager) emit(t events.EventType, data any) {
	if m.eventBus != nil {
		m.eventBus.Publish(events.NewEvent(t, data))
	}
}

// NewManager creates a new group manager
func NewManager(logger *slog.Logger, lights keylight.LightManager, cfg *config.Config) *Manager {
	manager := &Manager{
		logger: logger,
		lights: lights,
		groups: make(map[string]*Group),
		cfg:    cfg,
	}

	// Load existing groups
	if err := manager.loadGroups(); err != nil {
		logger.Error("failed to load groups", "error", err)
	}

	return manager
}

// loadGroups loads groups from the configuration file
func (m *Manager) loadGroups() error {
	// Get groups from config
	groupsMap := m.cfg.State.Groups
	if groupsMap == nil {
		m.logger.Debug("No groups found in config")
		return nil // No groups yet
	}

	groups := make(map[string]*Group)
	for id, groupData := range groupsMap {
		groupMap, ok := groupData.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid group data for %s", id)
		}

		name, ok := groupMap["name"].(string)
		if !ok {
			return fmt.Errorf("invalid group name for %s", id)
		}

		group := &Group{
			ID:   id,
			Name: name,
		}

		// Convert lights array
		lightsArray, ok := groupMap["lights"].([]any)
		if !ok {
			return fmt.Errorf("invalid lights data for group %s", id)
		}
		group.Lights = make([]string, len(lightsArray))
		for i, light := range lightsArray {
			s, ok := light.(string)
			if !ok {
				return fmt.Errorf("invalid light ID in group %s at index %d", id, i)
			}
			group.Lights[i] = s
		}

		groups[id] = group
	}

	m.mu.Lock()
	m.groups = groups
	m.mu.Unlock()

	m.logger.Info("Loaded groups from config", "count", len(groups))
	return nil
}

// saveGroups saves groups to the configuration file
func (m *Manager) saveGroups() error {
	m.mu.RLock()
	// Deep copy the groups map to avoid data races
	groupsMap := make(map[string]any)
	for id, group := range m.groups {
		groupsMap[id] = map[string]any{
			"name":   group.Name,
			"lights": append([]string{}, group.Lights...),
		}
	}
	m.mu.RUnlock()

	m.logger.Debug("Updating config with groups", "count", len(groupsMap), "groups", groupsMap)
	// Update config
	m.cfg.State.Groups = groupsMap

	m.logger.Debug("Saving config to file")
	// Save config
	if err := m.cfg.Save(); err != nil {
		m.logger.Error("Failed to save groups to config", "error", err)
		return fmt.Errorf("failed to save groups to config: %w", err)
	}

	m.logger.Debug("Groups saved successfully", "groups", groupsMap)
	return nil
}

// CreateGroup creates a new group of lights
func (m *Manager) CreateGroup(ctx context.Context, name string, lightIDs []string) (*Group, error) {
	m.logger.Debug("Creating group", "name", name, "lights", lightIDs)

	// Verify all lights exist OUTSIDE the lock (network I/O)
	for _, id := range lightIDs {
		if _, err := m.lights.GetLight(ctx, id); err != nil {
			m.logger.Error("Light not found", "id", id, "error", err)
			return nil, fmt.Errorf("light not found: %s", err)
		}
	}

	m.mu.Lock()
	group := &Group{
		ID:     "group-" + uuid.New().String(),
		Name:   name,
		Lights: lightIDs,
	}

	// Add group to map
	m.groups[group.ID] = group
	m.mu.Unlock()

	// Save to config
	if err := m.saveGroups(); err != nil {
		m.logger.Error("Failed to save groups", "error", err)
		return nil, fmt.Errorf("failed to save groups: %w", err)
	}

	m.logger.Debug("Created group successfully", "id", group.ID, "name", group.Name, "lights", group.Lights)
	m.emit(events.GroupCreated, group)
	return group, nil
}

// DeleteGroup removes a light group
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	group, exists := m.groups[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("group not found: %s", id)
	}

	// Copy for event emission after unlock
	groupCopy := *group
	delete(m.groups, id)
	m.logger.Info("deleted light group", "id", id)
	m.mu.Unlock()

	// Save groups to config
	if err := m.saveGroups(); err != nil {
		m.logger.Error("failed to save groups", "error", err)
	}

	m.emit(events.GroupDeleted, &groupCopy)
	return nil
}

// GetGroup returns a group by ID
func (m *Manager) GetGroup(id string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	if group.Lights == nil {
		group.Lights = []string{}
	}
	return group, nil
}

// GetGroups returns all groups
func (m *Manager) GetGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, group := range m.groups {
		if group.Lights == nil {
			group.Lights = []string{}
		}
		groups = append(groups, group)
	}
	return groups
}

// SetGroupLights sets the lights in a group
func (m *Manager) SetGroupLights(ctx context.Context, id string, lightIDs []string) error {
	// Verify all lights exist OUTSIDE the lock (network I/O)
	for _, lightID := range lightIDs {
		if _, err := m.lights.GetLight(ctx, lightID); err != nil {
			return fmt.Errorf("light not found: %s", err)
		}
	}

	m.mu.Lock()
	group, exists := m.groups[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("group not found: %s", id)
	}

	group.Lights = lightIDs
	// Copy for event emission after unlock
	groupCopy := *group
	m.logger.Info("updated group lights", "id", id, "lights", lightIDs)
	m.mu.Unlock()

	// Save groups to config
	if err := m.saveGroups(); err != nil {
		m.logger.Error("failed to save groups", "error", err)
	}

	m.emit(events.GroupUpdated, &groupCopy)
	return nil
}

// SetGroupState sets the power state for all lights in a group
func (m *Manager) SetGroupState(ctx context.Context, groupID string, on bool) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	errCh := make(chan error, len(group.Lights))
	var wg sync.WaitGroup
	for _, id := range group.Lights {
		wg.Add(1)
		go func(lightID string) {
			defer wg.Done()
			if err := m.lights.SetLightState(ctx, lightID, keylight.OnValue(on)); err != nil {
				errCh <- fmt.Errorf("light %s: %w", lightID, err)
			}
		}(id)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}
	return nil
}

// SetGroupBrightness sets the brightness for all lights in a group
func (m *Manager) SetGroupBrightness(ctx context.Context, groupID string, brightness int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	errCh := make(chan error, len(group.Lights))
	var wg sync.WaitGroup
	for _, id := range group.Lights {
		wg.Add(1)
		go func(lightID string) {
			defer wg.Done()
			if err := m.lights.SetLightBrightness(ctx, lightID, brightness); err != nil {
				errCh <- fmt.Errorf("light %s: %w", lightID, err)
			}
		}(id)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}
	return nil
}

// SetGroupTemperature sets the color temperature for all lights in a group
func (m *Manager) SetGroupTemperature(ctx context.Context, groupID string, temperature int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	errCh := make(chan error, len(group.Lights))
	var wg sync.WaitGroup
	for _, id := range group.Lights {
		wg.Add(1)
		go func(lightID string) {
			defer wg.Done()
			if err := m.lights.SetLightTemperature(ctx, lightID, temperature); err != nil {
				errCh <- fmt.Errorf("light %s: %w", lightID, err)
			}
		}(id)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}
	return nil
}

// GetGroupsByName returns all groups with the given name
func (m *Manager) GetGroupsByName(name string) []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Group
	for _, group := range m.groups {
		if group.Name == name {
			result = append(result, group)
		}
	}
	return result
}

// GetGroupsByKeys returns all groups matching the given comma-separated list of IDs or names.
// It matches by ID first, then by name (allowing multiple matches for names), and deduplicates results.
func (m *Manager) GetGroupsByKeys(keys string) ([]*Group, []string) {
	keyList := strings.Split(keys, ",")
	var matchedGroups []*Group
	var notFound []string
	groupSeen := make(map[string]bool)
	for _, key := range keyList {
		key = strings.TrimSpace(key)
		// Try by ID
		grp, err := m.GetGroup(key)
		if err == nil {
			if !groupSeen[grp.ID] {
				matchedGroups = append(matchedGroups, grp)
				groupSeen[grp.ID] = true
			}
			continue
		}
		// Try by name (could be multiple)
		byName := m.GetGroupsByName(key)
		if len(byName) > 0 {
			for _, g := range byName {
				if !groupSeen[g.ID] {
					matchedGroups = append(matchedGroups, g)
					groupSeen[g.ID] = true
				}
			}
		} else {
			notFound = append(notFound, key)
		}
	}
	return matchedGroups, notFound
}
