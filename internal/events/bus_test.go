package events

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	e := NewEvent(LightStateChanged, map[string]string{"id": "light-1"})

	assert.Equal(t, LightStateChanged, e.Type)
	assert.False(t, e.Timestamp.IsZero())

	var data map[string]string
	require.NoError(t, json.Unmarshal(e.Data, &data))
	assert.Equal(t, "light-1", data["id"])
}

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus()
	var received []Event
	var mu sync.Mutex

	unsub := bus.Subscribe(func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	bus.Publish(NewEvent(LightDiscovered, "hello"))
	bus.Publish(NewEvent(LightRemoved, "goodbye"))

	mu.Lock()
	assert.Len(t, received, 2)
	assert.Equal(t, LightDiscovered, received[0].Type)
	assert.Equal(t, LightRemoved, received[1].Type)
	mu.Unlock()

	// Unsubscribe and verify no more events
	unsub()
	bus.Publish(NewEvent(GroupCreated, nil))

	mu.Lock()
	assert.Len(t, received, 2)
	mu.Unlock()
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()
	var count1, count2 atomic.Int32

	unsub1 := bus.Subscribe(func(e Event) { count1.Add(1) })
	unsub2 := bus.Subscribe(func(e Event) { count2.Add(1) })

	bus.Publish(NewEvent(LightStateChanged, nil))

	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(1), count2.Load())

	unsub1()
	bus.Publish(NewEvent(LightStateChanged, nil))

	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(2), count2.Load())

	unsub2()
}

func TestBusNoSubscribers(t *testing.T) {
	bus := NewBus()
	// Should not panic
	bus.Publish(NewEvent(LightStateChanged, nil))
}
