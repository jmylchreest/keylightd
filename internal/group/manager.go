package group

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Manager handles light group management
type Manager struct {
	logger *slog.Logger
	lights keylight.LightManager
	groups map[string]*Group
	mu     sync.RWMutex
}

// Group represents a group of lights that can be controlled together
type Group struct {
	ID     string
	Name   string
	Lights []*keylight.Light
}

// NewManager creates a new group manager
func NewManager(logger *slog.Logger, lights keylight.LightManager) *Manager {
	return &Manager{
		logger: logger,
		lights: lights,
		groups: make(map[string]*Group),
	}
}

// CreateGroup creates a new group of lights
func (m *Manager) CreateGroup(name string, lightIDs []string) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group := &Group{
		ID:     fmt.Sprintf("group-%d", time.Now().UnixNano()),
		Name:   name,
		Lights: make([]*keylight.Light, 0, len(lightIDs)),
	}

	for _, id := range lightIDs {
		light, err := m.lights.GetLight(id)
		if err != nil {
			return nil, fmt.Errorf("light not found: %s", err)
		}
		group.Lights = append(group.Lights, light)
	}

	m.groups[group.ID] = group
	m.logger.Info("created light group", "id", group.ID, "name", name)
	return group, nil
}

// DeleteGroup removes a light group
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[id]; !exists {
		return fmt.Errorf("group not found: %s", id)
	}

	delete(m.groups, id)
	m.logger.Info("deleted light group", "id", id)
	return nil
}

// GetGroup returns a specific group by ID
func (m *Manager) GetGroup(id string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return group, nil
}

// GetGroups returns all light groups
func (m *Manager) GetGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, group := range m.groups {
		groups = append(groups, group)
	}
	return groups
}

// SetGroupState sets the power state for all lights in a group
func (m *Manager) SetGroupState(groupID string, on bool) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	for _, light := range group.Lights {
		if err := m.lights.SetLightState(light.ID, on); err != nil {
			return err
		}
	}
	return nil
}

// SetGroupBrightness sets the brightness for all lights in a group
func (m *Manager) SetGroupBrightness(groupID string, brightness int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	for _, light := range group.Lights {
		if err := m.lights.SetLightBrightness(light.ID, brightness); err != nil {
			return err
		}
	}
	return nil
}

// SetGroupTemperature sets the color temperature for all lights in a group
func (m *Manager) SetGroupTemperature(groupID string, temperature int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return err
	}

	for _, light := range group.Lights {
		if err := m.lights.SetLightTemperature(light.ID, temperature); err != nil {
			return err
		}
	}
	return nil
}
