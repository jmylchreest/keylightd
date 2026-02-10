package events

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusConcurrentPublish(t *testing.T) {
	bus := NewBus()
	var count atomic.Int64

	bus.Subscribe(func(e Event) {
		count.Add(1)
	})

	const goroutines = 50
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				bus.Publish(NewEvent(LightStateChanged, nil))
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(goroutines*eventsPerGoroutine), count.Load())
}

func TestBusConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus()
	var count atomic.Int64

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			unsub := bus.Subscribe(func(e Event) {
				count.Add(1)
			})
			// Publish an event while subscribed
			bus.Publish(NewEvent(LightDiscovered, nil))
			// Unsubscribe
			unsub()
			// Publish after unsubscribe — this subscriber should not receive
			bus.Publish(NewEvent(LightRemoved, nil))
		}()
	}
	wg.Wait()

	// Each goroutine's subscriber should receive at least 1 event (the one it publishes).
	// The exact count depends on timing — other goroutines' subscriptions may also receive events.
	// The key invariant: no panic, no race.
	assert.True(t, count.Load() >= int64(goroutines),
		"each subscriber should receive at least its own event, got %d", count.Load())
}

func TestBusConcurrentPublishAndSubscribe(t *testing.T) {
	bus := NewBus()
	var received atomic.Int64

	const publishers = 20
	const subscribers = 20
	const eventsPerPublisher = 50

	// Start subscribers
	var subWg sync.WaitGroup
	subWg.Add(subscribers)
	unsubs := make([]func(), subscribers)
	for i := 0; i < subscribers; i++ {
		idx := i
		unsubs[idx] = bus.Subscribe(func(e Event) {
			received.Add(1)
		})
		subWg.Done()
	}
	subWg.Wait()

	// Start publishers concurrently
	var pubWg sync.WaitGroup
	pubWg.Add(publishers)
	for i := 0; i < publishers; i++ {
		go func() {
			defer pubWg.Done()
			for j := 0; j < eventsPerPublisher; j++ {
				bus.Publish(NewEvent(LightStateChanged, map[string]int{"v": j}))
			}
		}()
	}
	pubWg.Wait()

	// Each subscriber should have received all events
	expected := int64(publishers * eventsPerPublisher * subscribers)
	assert.Equal(t, expected, received.Load())

	// Cleanup
	for _, unsub := range unsubs {
		unsub()
	}
}

func TestDoubleUnsubscribe(t *testing.T) {
	bus := NewBus()
	var count atomic.Int32

	unsub := bus.Subscribe(func(e Event) {
		count.Add(1)
	})

	bus.Publish(NewEvent(LightStateChanged, nil))
	assert.Equal(t, int32(1), count.Load())

	// First unsubscribe
	unsub()
	bus.Publish(NewEvent(LightStateChanged, nil))
	assert.Equal(t, int32(1), count.Load())

	// Second unsubscribe — should not panic
	unsub()
	bus.Publish(NewEvent(LightStateChanged, nil))
	assert.Equal(t, int32(1), count.Load())
}

func TestNewEvent_MarshalFailure(t *testing.T) {
	// json.Marshal will fail for channels
	ch := make(chan int)
	e := NewEvent(LightStateChanged, ch)

	assert.Equal(t, LightStateChanged, e.Type)
	assert.False(t, e.Timestamp.IsZero())
	assert.Equal(t, json.RawMessage("null"), e.Data)
}

func TestNewEvent_NilData(t *testing.T) {
	e := NewEvent(GroupCreated, nil)

	assert.Equal(t, GroupCreated, e.Type)
	assert.False(t, e.Timestamp.IsZero())

	var data any
	require.NoError(t, json.Unmarshal(e.Data, &data))
	assert.Nil(t, data)
}

func TestBus_SubscribeReturnsUniqueIDs(t *testing.T) {
	bus := NewBus()
	var received1, received2 atomic.Int32

	unsub1 := bus.Subscribe(func(e Event) { received1.Add(1) })
	unsub2 := bus.Subscribe(func(e Event) { received2.Add(1) })

	bus.Publish(NewEvent(LightStateChanged, nil))
	assert.Equal(t, int32(1), received1.Load())
	assert.Equal(t, int32(1), received2.Load())

	// Unsubscribe first, second should still work
	unsub1()
	bus.Publish(NewEvent(LightStateChanged, nil))
	assert.Equal(t, int32(1), received1.Load())
	assert.Equal(t, int32(2), received2.Load())

	unsub2()
}

func TestBus_PublishOrder(t *testing.T) {
	bus := NewBus()
	var mu sync.Mutex
	var received []EventType

	bus.Subscribe(func(e Event) {
		mu.Lock()
		received = append(received, e.Type)
		mu.Unlock()
	})

	types := []EventType{LightDiscovered, LightStateChanged, LightRemoved, GroupCreated, GroupDeleted, GroupUpdated}
	for _, et := range types {
		bus.Publish(NewEvent(et, nil))
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, types, received)
}
