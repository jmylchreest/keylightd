// Package events provides a lightweight in-process event bus for broadcasting
// state changes to subscribers (e.g., WebSocket hub, future SSE endpoint).
package events

import (
	"encoding/json"
	"sync"
	"time"
)

// EventType identifies the kind of event.
type EventType string

const (
	// Light events
	LightStateChanged EventType = "light.state_changed"
	LightDiscovered   EventType = "light.discovered"
	LightRemoved      EventType = "light.removed"

	// Group events
	GroupCreated EventType = "group.created"
	GroupDeleted EventType = "group.deleted"
	GroupUpdated EventType = "group.updated"
)

// Event is a single event emitted by a producer.
type Event struct {
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// NewEvent creates an Event, marshaling data to JSON.
// If marshaling fails the Data field is set to null.
func NewEvent(t EventType, data any) Event {
	raw, err := json.Marshal(data)
	if err != nil {
		raw = []byte("null")
	}
	return Event{
		Type:      t,
		Timestamp: time.Now(),
		Data:      raw,
	}
}

// SubscriberFunc is a callback invoked for each event.
// Implementations must not block; slow subscribers should buffer internally.
type SubscriberFunc func(Event)

// Bus is a simple synchronous fan-out event bus.
// Publishing blocks until all subscribers have been called, so subscribers
// should be fast (e.g., write to a channel).
type Bus struct {
	mu          sync.RWMutex
	subscribers map[int]SubscriberFunc
	nextID      int
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[int]SubscriberFunc),
	}
}

// Subscribe registers a callback and returns an unsubscribe function.
func (b *Bus) Subscribe(fn SubscriberFunc) func() {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = fn
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
	}
}

// Publish sends an event to all current subscribers.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	// Snapshot subscriber list under read lock so we don't hold it during callbacks.
	subs := make([]SubscriberFunc, 0, len(b.subscribers))
	for _, fn := range b.subscribers {
		subs = append(subs, fn)
	}
	b.mu.RUnlock()

	for _, fn := range subs {
		fn(e)
	}
}
