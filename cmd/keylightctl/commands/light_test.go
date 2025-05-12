package commands

import (
	"context"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/stretchr/testify/require"
)

// Use the same clientContextKey as in light.go
// var clientContextKey = &struct{}{} // already defined in light.go

// mockClient implements client.ClientInterface for CLI tests
// and returns static data for testing.
type mockClient struct{}

var _ client.ClientInterface = (*mockClient)(nil)

func (m *mockClient) GetLight(id string) (map[string]interface{}, error) {
	// Use a fixed time for predictable test output
	lastSeenTime := time.Date(2023, time.October, 26, 10, 0, 0, 0, time.UTC)
	return map[string]interface{}{
		"id":              id,
		"productname":     "Test Light",
		"serialnumber":    "123456",
		"firmwareversion": "1.0.0",
		"firmwarebuild":   1,
		"on":              true,
		"brightness":      50,
		"temperature":     5000,
		"ip":              "192.168.1.1",
		"port":            9123,
		"lastseen":        lastSeenTime,
	}, nil
}

func (m *mockClient) GetLights() (map[string]interface{}, error) {
	// Use a fixed time for predictable test output
	lastSeenTime1 := time.Date(2023, time.October, 26, 10, 0, 0, 0, time.UTC)
	lastSeenTime2 := time.Date(2023, time.October, 26, 10, 5, 0, 0, time.UTC)

	return map[string]interface{}{
		"light1": map[string]interface{}{
			"id":              "light1",
			"productname":     "Light 1",
			"serialnumber":    "SN1",
			"firmwareversion": "1.0.0",
			"firmwarebuild":   1,
			"on":              true,
			"brightness":      50,
			"temperature":     5000,
			"ip":              "192.168.1.1",
			"port":            9123,
			"lastseen":        lastSeenTime1,
		},
		"light2": map[string]interface{}{
			"id":              "light2",
			"productname":     "Light 2",
			"serialnumber":    "SN2",
			"firmwareversion": "1.0.0",
			"firmwarebuild":   1,
			"on":              false,
			"brightness":      0,
			"temperature":     3000,
			"ip":              "192.168.1.2",
			"port":            9123,
			"lastseen":        lastSeenTime2,
		},
	}, nil
}

func (m *mockClient) SetLightState(id string, property string, value interface{}) error {
	return nil
}

func (m *mockClient) CreateGroup(name string) error {
	return nil
}

func (m *mockClient) GetGroup(name string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *mockClient) GetGroups() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (m *mockClient) SetGroupState(name string, property string, value interface{}) error {
	return nil
}

func (m *mockClient) DeleteGroup(name string) error {
	return nil
}

func (m *mockClient) SetGroupLights(groupID string, lightIDs []string) error {
	return nil
}

// API Key Management Mocks (satisfy client.ClientInterface)
func (m *mockClient) AddAPIKey(name string, expiresInSeconds float64) (map[string]interface{}, error) {
	// Simple mock: doesn't actually store/return a real key structure for light tests
	return map[string]interface{}{"key": "mockapikey", "name": name}, nil
}

func (m *mockClient) ListAPIKeys() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil // Return empty list for light tests
}

func (m *mockClient) DeleteAPIKey(key string) error {
	return nil
}

func (m *mockClient) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (map[string]interface{}, error) {
	return map[string]interface{}{"key": keyOrName, "disabled": disabled}, nil
}

func TestLightGetCommandParseable(t *testing.T) {
	mock := &mockClient{}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)

	cmd := newLightGetCommand()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"test-light"})
	err := cmd.Execute()
	require.NoError(t, err)

	cmd = newLightGetCommand()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"test-light", "on"})
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestLightListCommandParseable(t *testing.T) {
	mock := &mockClient{}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)

	cmd := newLightListCommand()
	cmd.SetContext(ctx)
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestLightGetCommand(t *testing.T) {
	mock := &mockClient{}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)

	// Test table output
	outTable := captureStdout(func() {
		cmd := newLightGetCommand()
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"test-light"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
	require.Contains(t, outTable, "ID") // Check for plain text ID
	require.Contains(t, outTable, "Test Light")
	require.Contains(t, outTable, "1.0.0 (build 1)")
	require.Contains(t, outTable, "true")
	require.Contains(t, outTable, "50")
	require.Contains(t, outTable, "5000")
	require.Contains(t, outTable, "192.168.1.1")
	require.Contains(t, outTable, "9123")
	// Check for formatted LastSeen time (e.g., Thu, 26 Oct 2023 10:00:00 +0000)
	require.Contains(t, outTable, "Thu, 26 Oct 2023 10:00:00 +0000")

	// Test parseable output
	outParseable := captureStdout(func() {
		cmd := newLightGetCommand()
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"test-light", "--parseable"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
	require.Contains(t, outParseable, "id=\"test-light\"")
	require.Contains(t, outParseable, "productname=\"Test Light\"")
	require.Contains(t, outParseable, "serialnumber=\"123456\"")
	require.Contains(t, outParseable, "firmwareversion=\"1.0.0\"")
	require.Contains(t, outParseable, "firmwarebuild=1")
	require.Contains(t, outParseable, "on=true")
	require.Contains(t, outParseable, "brightness=50")
	require.Contains(t, outParseable, "temperature=5000")
	require.Contains(t, outParseable, "ip=\"192.168.1.1\"")
	require.Contains(t, outParseable, "port=9123")
	// Check for Unix timestamp of fixed time
	require.Contains(t, outParseable, "lastseen=1698314400") // Unix timestamp for 2023-10-26 10:00:00 UTC
}

func TestLightListCommand(t *testing.T) {
	mock := &mockClient{}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)

	// Test table output
	outTable := captureStdout(func() {
		cmd := newLightListCommand()
		cmd.Context() // Ensure context is set
		cmd.SetContext(ctx)
		err := cmd.Execute()
		require.NoError(t, err)
	})
	require.Contains(t, outTable, "ID") // Check for plain text ID
	require.Contains(t, outTable, "Light 1")
	require.Contains(t, outTable, "SN1")
	require.Contains(t, outTable, "192.168.1.1")
	// Check for formatted LastSeen times
	require.Contains(t, outTable, "Thu, 26 Oct 2023 10:00:00 +0000")
	require.Contains(t, outTable, "Thu, 26 Oct 2023 10:05:00 +0000")

	// Test parseable output
	outParseable := captureStdout(func() {
		cmd := newLightListCommand()
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"--parseable"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
	require.Contains(t, outParseable, "id=\"light1\"")
	require.Contains(t, outParseable, "serialnumber=\"SN1\"")
	require.Contains(t, outParseable, "lastseen=1698314400") // Unix timestamp for 2023-10-26 10:00:00 UTC
	require.Contains(t, outParseable, "id=\"light2\"")
	require.Contains(t, outParseable, "serialnumber=\"SN2\"")
	require.Contains(t, outParseable, "lastseen=1698314700") // Unix timestamp for 2023-10-26 10:05:00 UTC
}
