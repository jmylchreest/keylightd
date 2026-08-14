package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClient records how often the tray refresh loop fetched status.
type fakeClient struct {
	mu     sync.Mutex
	lights map[string]any
	calls  atomic.Int64
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		lights: map[string]any{
			"light-1": map[string]any{"name": "Key Light", "on": true},
		},
	}
}

func (f *fakeClient) setOn(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lights["light-1"] = map[string]any{"name": "Key Light", "on": on}
}

func (f *fakeClient) GetLights() (map[string]any, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]any, len(f.lights))
	for k, v := range f.lights {
		out[k] = v
	}
	return out, nil
}

func (f *fakeClient) GetGroups() ([]map[string]any, error) { return nil, nil }

func (f *fakeClient) GetVersion() (map[string]any, error)     { return nil, nil }
func (f *fakeClient) GetLight(string) (map[string]any, error) { return nil, nil }
func (f *fakeClient) SetLightState(string, string, any) error { return nil }
func (f *fakeClient) CreateGroup(string) error                { return nil }
func (f *fakeClient) GetGroup(string) (map[string]any, error) { return nil, nil }
func (f *fakeClient) SetGroupState(string, string, any) error { return nil }
func (f *fakeClient) DeleteGroup(string) error                { return nil }
func (f *fakeClient) SetGroupLights(string, []string) error   { return nil }
func (f *fakeClient) ListAPIKeys() ([]map[string]any, error)  { return nil, nil }
func (f *fakeClient) DeleteAPIKey(string) error               { return nil }
func (f *fakeClient) AddAPIKey(string, float64) (map[string]any, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeClient) SetAPIKeyDisabledStatus(string, bool) (map[string]any, error) {
	return nil, errors.New("not implemented")
}

func newTestApp(c *fakeClient) *App {
	app := NewApp("test", "test", "test")
	app.client = c
	app.logger = slog.New(slog.DiscardHandler)
	return app
}

func waitForCalls(t *testing.T, c *fakeClient, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.calls.Load() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected at least %d status fetches, got %d", want, c.calls.Load())
}

// The tray must refresh from Go, not from the frontend: the window is hidden
// most of the time and the frontend stops polling while hidden.
func TestRunTrayRefreshOnSignal(t *testing.T) {
	c := newFakeClient()
	app := newTestApp(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.runTrayRefresh(ctx)

	app.SignalTrayRefresh()
	waitForCalls(t, c, 1, 2*time.Second)
}

// A group toggle emits one daemon event per light; those must collapse into
// a single status fetch rather than one fetch each.
func TestRunTrayRefreshCoalescesBurst(t *testing.T) {
	c := newFakeClient()
	app := newTestApp(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.runTrayRefresh(ctx)

	for range 20 {
		app.SignalTrayRefresh()
	}
	waitForCalls(t, c, 1, 2*time.Second)

	time.Sleep(2 * trayRefreshDebounce)
	if got := c.calls.Load(); got > 2 {
		t.Errorf("burst of 20 signals produced %d status fetches, want <= 2", got)
	}
}

func TestSignalTrayRefreshNeverBlocks(t *testing.T) {
	app := NewApp("test", "test", "test") // no refresh loop draining the channel

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			app.SignalTrayRefresh()
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SignalTrayRefresh blocked when a refresh was already pending")
	}
}

func TestRunTrayRefreshStopsOnContextCancel(t *testing.T) {
	c := newFakeClient()
	app := newTestApp(c)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		app.runTrayRefresh(ctx)
		close(stopped)
	}()

	app.SignalTrayRefresh()
	waitForCalls(t, c, 1, 2*time.Second)
	cancel()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("runTrayRefresh did not exit after context cancellation")
	}
}

// The status the tray renders must be the post-toggle state, not the state
// read before the set call.
func TestGetStatusReflectsLatestState(t *testing.T) {
	c := newFakeClient()
	app := newTestApp(c)

	status, err := app.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.OnCount != 1 {
		t.Fatalf("OnCount = %d, want 1", status.OnCount)
	}

	c.setOn(false)

	status, err = app.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.OnCount != 0 || status.OffCount != 1 {
		t.Errorf("after toggle: OnCount = %d, OffCount = %d, want 0 and 1", status.OnCount, status.OffCount)
	}
}
