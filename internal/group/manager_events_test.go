package group

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectEvents subscribes to a bus and returns a function to get collected events.
func collectEvents(bus *events.Bus) func() []events.Event {
	var mu sync.Mutex
	var collected []events.Event
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		collected = append(collected, e)
		mu.Unlock()
	})
	return func() []events.Event {
		mu.Lock()
		defer mu.Unlock()
		out := make([]events.Event, len(collected))
		copy(out, collected)
		return out
	}
}

func TestSetEventBus_EnablesGroupEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{lights: make(map[string]*keylight.Light)}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	bus := events.NewBus()

	// Before SetEventBus — emit should not panic
	manager.emit(events.GroupCreated, "test")

	// After SetEventBus — events should flow
	manager.SetEventBus(bus)
	getEvents := collectEvents(bus)

	manager.emit(events.GroupCreated, map[string]string{"name": "test"})

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.GroupCreated, evts[0].Type)
}

func TestCreateGroup_EmitsGroupCreatedEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	bus := events.NewBus()
	manager.SetEventBus(bus)
	getEvents := collectEvents(bus)

	group, err := manager.CreateGroup(context.Background(), "event-group", []string{"light1"})
	require.NoError(t, err)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.GroupCreated, evts[0].Type)

	// Verify event data
	var groupData Group
	require.NoError(t, json.Unmarshal(evts[0].Data, &groupData))
	assert.Equal(t, group.ID, groupData.ID)
	assert.Equal(t, "event-group", groupData.Name)
	assert.Equal(t, []string{"light1"}, groupData.Lights)
}

func TestDeleteGroup_EmitsGroupDeletedEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	bus := events.NewBus()
	manager.SetEventBus(bus)

	// Create group first
	group, err := manager.CreateGroup(context.Background(), "to-delete", []string{"light1"})
	require.NoError(t, err)

	// Start collecting events after creation (to ignore the creation event)
	getEvents := collectEvents(bus)

	err = manager.DeleteGroup(group.ID)
	require.NoError(t, err)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.GroupDeleted, evts[0].Type)

	var groupData Group
	require.NoError(t, json.Unmarshal(evts[0].Data, &groupData))
	assert.Equal(t, group.ID, groupData.ID)
	assert.Equal(t, "to-delete", groupData.Name)
}

func TestSetGroupLights_EmitsGroupUpdatedEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
			"light2": {ID: "light2", Name: "Light 2"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	bus := events.NewBus()
	manager.SetEventBus(bus)

	// Create group
	group, err := manager.CreateGroup(context.Background(), "update-lights", []string{"light1"})
	require.NoError(t, err)

	// Start collecting after creation
	getEvents := collectEvents(bus)

	// Update group lights
	err = manager.SetGroupLights(context.Background(), group.ID, []string{"light1", "light2"})
	require.NoError(t, err)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.GroupUpdated, evts[0].Type)

	var groupData Group
	require.NoError(t, json.Unmarshal(evts[0].Data, &groupData))
	assert.Equal(t, group.ID, groupData.ID)
	assert.Equal(t, []string{"light1", "light2"}, groupData.Lights)
}

func TestCreateGroup_NoEventWithoutBus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{
		lights: map[string]*keylight.Light{
			"light1": {ID: "light1", Name: "Light 1"},
		},
	}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	// No SetEventBus — should not panic
	_, err := manager.CreateGroup(context.Background(), "no-bus-group", []string{"light1"})
	require.NoError(t, err)
}

func TestDeleteGroup_NoEventForNonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	lights := &mockLightManager{lights: make(map[string]*keylight.Light)}
	cfg := setupTestConfig(t)
	manager := NewManager(logger, lights, cfg)

	bus := events.NewBus()
	manager.SetEventBus(bus)
	getEvents := collectEvents(bus)

	err := manager.DeleteGroup("non-existent")
	assert.Error(t, err)

	evts := getEvents()
	assert.Len(t, evts, 0, "no event should be emitted for non-existent group deletion")
}
