package commands

import (
	"context"
	"testing"

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
	}, nil
}

func (m *mockClient) GetLights() (map[string]interface{}, error) {
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
