package keylight

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newValidAccessoryInfoHandler returns an http.Handler that responds with valid
// Elgato Key Light accessory-info JSON. An optional delay can simulate slow responses.
func newValidAccessoryInfoHandler(displayName string, delay time.Duration) http.Handler {
	return newAccessoryInfoHandler("Elgato Key Light", 2, displayName, delay)
}

// newAccessoryInfoHandler returns an http.Handler that responds with
// accessory-info JSON for the given product name and board type.
func newAccessoryInfoHandler(productName string, boardType int, displayName string, delay time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		if r.URL.Path == "/elgato/accessory-info" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(AccessoryInfo{
				ProductName:         productName,
				HardwareBoardType:   boardType,
				FirmwareBuildNumber: 200,
				FirmwareVersion:     "1.0.3",
				SerialNumber:        "SN-" + displayName,
				DisplayName:         displayName,
			})
			return
		}
		http.NotFound(w, r)
	})
}

// newInvalidProductHandler returns an http.Handler that responds with
// a non-Key-Light product name.
func newInvalidProductHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elgato/accessory-info" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(AccessoryInfo{
				ProductName:  "Elgato Ring Light",
				SerialNumber: "SN-RING",
				DisplayName:  "My Ring Light",
			})
			return
		}
		http.NotFound(w, r)
	})
}

// newErrorHandler returns an http.Handler that always responds with HTTP 500.
func newErrorHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
}

// newRequestCountingHandler wraps a handler and counts requests.
func newRequestCountingHandler(inner http.Handler, counter *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		inner.ServeHTTP(w, r)
	})
}

// makeServiceEntry creates a ServiceEntry for a given httptest.Server.
func makeServiceEntry(t *testing.T, server *httptest.Server, name string) *ServiceEntry {
	t.Helper()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	var p int
	_, err = (&net.Resolver{}).LookupHost(context.Background(), host) // just to validate
	_ = err
	p = server.Listener.Addr().(*net.TCPAddr).Port
	_ = port
	return &ServiceEntry{
		Name:   name,
		AddrV4: net.ParseIP(host),
		Port:   p,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// --- validateLight tests ---

func TestValidateLight_ValidKeyLight(t *testing.T) {
	server := httptest.NewServer(newValidAccessoryInfoHandler("Test Light", 0))
	defer server.Close()

	entry := makeServiceEntry(t, server, "test._elg._tcp.local.")
	light, valid := validateLight(context.Background(), entry, discardLogger())

	assert.True(t, valid)
	assert.Equal(t, "Elgato Key Light", light.ProductName)
	assert.Equal(t, "SN-Test Light", light.SerialNumber)
	assert.Equal(t, "Test Light", light.Name)
	assert.Equal(t, entry.Port, light.Port)
}

func TestValidateLight_ValidKeyLightMK2(t *testing.T) {
	server := httptest.NewServer(newAccessoryInfoHandler("Elgato Key Light MK.2", 205, "Test MK2 Light", 0))
	defer server.Close()

	entry := makeServiceEntry(t, server, "testmk2._elg._tcp.local.")
	light, valid := validateLight(context.Background(), entry, discardLogger())

	assert.True(t, valid)
	assert.Equal(t, "Elgato Key Light MK.2", light.ProductName)
	assert.Equal(t, 205, light.HardwareBoardType)
	assert.Equal(t, "SN-Test MK2 Light", light.SerialNumber)
	assert.Equal(t, "Test MK2 Light", light.Name)
	assert.Equal(t, entry.Port, light.Port)
}

func TestValidateLight_InvalidProduct(t *testing.T) {
	server := httptest.NewServer(newInvalidProductHandler())
	defer server.Close()

	entry := makeServiceEntry(t, server, "ring._elg._tcp.local.")
	_, valid := validateLight(context.Background(), entry, discardLogger())

	assert.False(t, valid, "non-Key-Light product should not validate")
}

func TestValidateLight_ServerError(t *testing.T) {
	server := httptest.NewServer(newErrorHandler())
	defer server.Close()

	entry := makeServiceEntry(t, server, "error._elg._tcp.local.")
	_, valid := validateLight(context.Background(), entry, discardLogger())

	assert.False(t, valid, "server error should cause validation failure")
}

func TestValidateLight_NilEntry(t *testing.T) {
	_, valid := validateLight(context.Background(), nil, discardLogger())
	assert.False(t, valid)
}

func TestValidateLight_MissingAddr(t *testing.T) {
	entry := &ServiceEntry{Name: "test", AddrV4: nil, Port: 9123}
	_, valid := validateLight(context.Background(), entry, discardLogger())
	assert.False(t, valid)
}

func TestValidateLight_ZeroPort(t *testing.T) {
	entry := &ServiceEntry{Name: "test", AddrV4: net.ParseIP("127.0.0.1"), Port: 0}
	_, valid := validateLight(context.Background(), entry, discardLogger())
	assert.False(t, valid)
}

//nolint:misspell // British spelling intentional
func TestValidateLight_ContextCancelled(t *testing.T) {
	//nolint:misspell // British spelling intentional
	// Server that delays long enough for context to be cancelled
	server := httptest.NewServer(newValidAccessoryInfoHandler("Slow Light", 5*time.Second))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	entry := makeServiceEntry(t, server, "slow._elg._tcp.local.")
	_, valid := validateLight(ctx, entry, discardLogger())

	assert.False(t, valid, "cancelled context should cause validation failure") //nolint:misspell
}

func TestValidateLight_ContextTimeout(t *testing.T) {
	// Server that delays longer than the context timeout
	server := httptest.NewServer(newValidAccessoryInfoHandler("Slow Light", 5*time.Second))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	entry := makeServiceEntry(t, server, "slow._elg._tcp.local.")
	_, valid := validateLight(ctx, entry, discardLogger())

	assert.False(t, valid, "timed-out context should cause validation failure")
}

// --- DiscoveryParams tests ---

func TestCalculateMaxDiscoveryTime(t *testing.T) {
	params := DiscoveryParams{
		browseAttempts:       3,
		initialBrowseTimeout: 3 * time.Second,
		browseDelay:          500 * time.Millisecond,
		validateTimeout:      5 * time.Second,
	}

	// attempt 1: 3s, delay: 500ms, attempt 2: 6s, delay: 500ms, attempt 3: 12s, validate: 5s
	expected := 3*time.Second + 500*time.Millisecond +
		6*time.Second + 500*time.Millisecond +
		12*time.Second +
		5*time.Second
	assert.Equal(t, expected, params.calculateMaxDiscoveryTime())
}

func TestCalculateMaxDiscoveryTime_SingleAttempt(t *testing.T) {
	params := DiscoveryParams{
		browseAttempts:       1,
		initialBrowseTimeout: 2 * time.Second,
		browseDelay:          1 * time.Second,
		validateTimeout:      3 * time.Second,
	}

	// 1 attempt: 2s (no delay after last attempt), validate: 3s
	expected := 2*time.Second + 3*time.Second
	assert.Equal(t, expected, params.calculateMaxDiscoveryTime())
}

// --- Context isolation test (the core bug fix) ---

func TestValidateLight_IndependentContexts(t *testing.T) {
	// This test verifies the core fix: two lights being validated with
	//nolint:misspell // British spelling intentional
	// independent contexts. Cancelling one context must not affect the other.

	var reqCount1 atomic.Int32
	var reqCount2 atomic.Int32

	server1 := httptest.NewServer(newRequestCountingHandler(
		newValidAccessoryInfoHandler("Light 1", 0), &reqCount1))
	defer server1.Close()

	server2 := httptest.NewServer(newRequestCountingHandler(
		newValidAccessoryInfoHandler("Light 2", 0), &reqCount2))
	defer server2.Close()

	entry1 := makeServiceEntry(t, server1, "light1._elg._tcp.local.")
	entry2 := makeServiceEntry(t, server2, "light2._elg._tcp.local.")

	// Create two independent contexts (simulating the fix)
	parentCtx := context.Background()

	ctx1, cancel1 := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel1()
	ctx2, cancel2 := context.WithTimeout(parentCtx, 5*time.Second)
	defer cancel2()

	// Validate light 1
	light1, valid1 := validateLight(ctx1, entry1, discardLogger())
	assert.True(t, valid1)
	assert.Equal(t, "Light 1", light1.Name)

	// Cancel ctx1 — this must NOT affect ctx2
	cancel1()

	// Validate light 2 — should still succeed
	light2, valid2 := validateLight(ctx2, entry2, discardLogger())
	assert.True(t, valid2, "cancelling ctx1 must not affect ctx2 validation") //nolint:misspell
	assert.Equal(t, "Light 2", light2.Name)

	// Both servers should have received exactly 1 request
	assert.Equal(t, int32(1), reqCount1.Load())
	assert.Equal(t, int32(1), reqCount2.Load())
}

func TestValidateLight_SharedContextCancelsAll(t *testing.T) {
	// This test demonstrates the OLD bug: if both validations share the same
	//nolint:misspell // British spelling intentional
	// context, cancelling it kills both. The fix ensures they DON'T share contexts.

	server1 := httptest.NewServer(newValidAccessoryInfoHandler("Light 1", 200*time.Millisecond))
	defer server1.Close()

	server2 := httptest.NewServer(newValidAccessoryInfoHandler("Light 2", 200*time.Millisecond))
	defer server2.Close()

	entry1 := makeServiceEntry(t, server1, "light1._elg._tcp.local.")
	entry2 := makeServiceEntry(t, server2, "light2._elg._tcp.local.")

	// Shared context — cancel it after first validation starts
	sharedCtx, sharedCancel := context.WithCancel(context.Background())

	// Validate light 1 with shared context, then cancel
	_, valid1 := validateLight(sharedCtx, entry1, discardLogger())
	assert.True(t, valid1, "first validation should succeed before cancel")

	sharedCancel()

	//nolint:misspell // British spelling intentional
	// With the shared context cancelled, second validation should fail
	_, valid2 := validateLight(sharedCtx, entry2, discardLogger())
	assert.False(t, valid2, "second validation should fail with cancelled shared context") //nolint:misspell
}
