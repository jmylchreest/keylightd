package keylight

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectEvents subscribes to a bus and returns a function to get collected events.
func collectEvents(bus *events.Bus) (getEvents func() []events.Event) {
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

func TestSetEventBus_EnablesEventEmission(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, _ := newTestManager(logger)
	bus := events.NewBus()

	// Before SetEventBus — no panic, no events
	manager.emit(events.LightStateChanged, "test")

	// After SetEventBus — events should flow
	manager.SetEventBus(bus)
	getEvents := collectEvents(bus)

	manager.emit(events.LightStateChanged, map[string]string{"id": "test"})

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.LightStateChanged, evts[0].Type)
}

func TestAddLight_EmitsLightDiscoveredEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, _ := newTestManager(logger)
	bus := events.NewBus()
	manager.SetEventBus(bus)
	getEvents := collectEvents(bus)

	light := Light{
		ID:   "event-light-1",
		Name: "Event Light",
		IP:   net.ParseIP("192.168.1.100"),
		Port: 9123,
	}

	manager.AddLight(context.Background(), light)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.LightDiscovered, evts[0].Type)

	// Verify event data contains the light
	var lightData Light
	require.NoError(t, json.Unmarshal(evts[0].Data, &lightData))
	assert.Equal(t, "event-light-1", lightData.ID)
}

func TestSetLightState_EmitsStateChangedEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, mockHTTP := newTestManager(logger)
	bus := events.NewBus()
	manager.SetEventBus(bus)

	// Set up a light with a mock client
	light := Light{
		ID:   "state-event-light",
		Name: "State Event Light",
		IP:   net.ParseIP("192.168.1.50"),
		Port: 9123,
	}
	manager.lights[light.ID] = light
	manager.clients[light.ID] = NewKeyLightClient(light.IP.String(), light.Port, logger, mockHTTP)

	getEvents := collectEvents(bus)

	// SetLightState should emit LightStateChanged
	err := manager.SetLightState(context.Background(), "state-event-light", OnValue(true))
	require.NoError(t, err)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.LightStateChanged, evts[0].Type)

	// Verify event data
	var lightData Light
	require.NoError(t, json.Unmarshal(evts[0].Data, &lightData))
	assert.Equal(t, "state-event-light", lightData.ID)
}

func TestSetLightState_NoEventWithoutBus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, mockHTTP := newTestManager(logger)

	// No SetEventBus call — should not panic
	light := Light{
		ID:   "no-bus-light",
		Name: "No Bus Light",
		IP:   net.ParseIP("192.168.1.51"),
		Port: 9123,
	}
	manager.lights[light.ID] = light
	manager.clients[light.ID] = NewKeyLightClient(light.IP.String(), light.Port, logger, mockHTTP)

	err := manager.SetLightState(context.Background(), "no-bus-light", BrightnessValue(50))
	require.NoError(t, err)
	// No panic means pass
}

func TestCleanupStaleLights_EmitsLightRemovedEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, mockHTTP := newTestManager(logger)
	bus := events.NewBus()
	manager.SetEventBus(bus)

	// Add a stale light
	staleLight := Light{
		ID:       "stale-event-light",
		Name:     "Stale Event Light",
		IP:       net.ParseIP("192.168.1.200"),
		Port:     9123,
		LastSeen: time.Now().Add(-10 * time.Minute),
	}
	manager.lights[staleLight.ID] = staleLight
	manager.clients[staleLight.ID] = NewKeyLightClient(staleLight.IP.String(), staleLight.Port, logger, mockHTTP)

	getEvents := collectEvents(bus)

	// Run cleanup with 5-minute timeout
	manager.cleanupStaleLights(5 * time.Minute)

	evts := getEvents()
	require.Len(t, evts, 1)
	assert.Equal(t, events.LightRemoved, evts[0].Type)

	var lightData Light
	require.NoError(t, json.Unmarshal(evts[0].Data, &lightData))
	assert.Equal(t, "stale-event-light", lightData.ID)
}

func TestCleanupStaleLights_NoEventForFreshLights(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, mockHTTP := newTestManager(logger)
	bus := events.NewBus()
	manager.SetEventBus(bus)

	freshLight := Light{
		ID:       "fresh-event-light",
		Name:     "Fresh Event Light",
		IP:       net.ParseIP("192.168.1.201"),
		Port:     9123,
		LastSeen: time.Now(),
	}
	manager.lights[freshLight.ID] = freshLight
	manager.clients[freshLight.ID] = NewKeyLightClient(freshLight.IP.String(), freshLight.Port, logger, mockHTTP)

	getEvents := collectEvents(bus)

	manager.cleanupStaleLights(5 * time.Minute)

	evts := getEvents()
	assert.Len(t, evts, 0, "no events should be emitted for fresh lights")
}

func TestMultipleStaleCleanup_EmitsMultipleEvents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelInfo}))
	manager, mockHTTP := newTestManager(logger)
	bus := events.NewBus()
	manager.SetEventBus(bus)

	staleTime := time.Now().Add(-10 * time.Minute)
	for i, id := range []string{"stale-1", "stale-2", "stale-3"} {
		l := Light{
			ID:       id,
			Name:     id,
			IP:       net.ParseIP("192.168.1." + string(rune('1'+i))),
			Port:     9123,
			LastSeen: staleTime,
		}
		manager.lights[id] = l
		manager.clients[id] = NewKeyLightClient(l.IP.String(), l.Port, logger, mockHTTP)
	}

	getEvents := collectEvents(bus)
	manager.cleanupStaleLights(5 * time.Minute)

	evts := getEvents()
	assert.Len(t, evts, 3, "should emit one event per stale light removed")
	for _, evt := range evts {
		assert.Equal(t, events.LightRemoved, evt.Type)
	}
}
