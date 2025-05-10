package commands

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct {
	lights map[string]interface{}
}

func (m *mockClient) GetLights() (map[string]interface{}, error) {
	return m.lights, nil
}

func (m *mockClient) GetLight(id string) (map[string]interface{}, error) {
	if light, ok := m.lights[id]; ok {
		return light.(map[string]interface{}), nil
	}
	return nil, nil
}

func (m *mockClient) SetLightState(id string, property string, value interface{}) error {
	return nil
}

func TestLightGetCommandParseable(t *testing.T) {
	// Setup mock client
	mock := &mockClient{
		lights: map[string]interface{}{
			"test-light": map[string]interface{}{
				"productname":     "Test Light",
				"serialnumber":    "TEST123",
				"firmwareversion": "1.0.0",
				"firmwarebuild":   123,
				"on":              true,
				"brightness":      50,
				"temperature":     344,
				"ip":              "192.168.1.100",
				"port":            9123,
			},
		},
	}

	// Create command
	cmd := newLightGetCommand()
	cmd.Flags().Set("parseable", "true")

	// Create context with mock client
	ctx := context.WithValue(context.Background(), "client", mock)
	cmd.SetContext(ctx)

	// Test getting all properties
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, []string{"test-light"})
	require.NoError(t, err)

	expected := `id="test-light" productname="Test Light" serialnumber="TEST123" firmwareversion="1.0.0" firmwarebuild=123 on=true brightness=50 temperature=344 ip="192.168.1.100" port=9123`
	assert.Equal(t, expected+"\n", buf.String())

	// Test getting single property
	buf.Reset()
	err = cmd.RunE(cmd, []string{"test-light", "brightness"})
	require.NoError(t, err)
	assert.Equal(t, "brightness=50\n", buf.String())
}

func TestLightListCommandParseable(t *testing.T) {
	// Setup mock client
	mock := &mockClient{
		lights: map[string]interface{}{
			"light1": map[string]interface{}{
				"productname":     "Light 1",
				"serialnumber":    "TEST123",
				"firmwareversion": "1.0.0",
				"firmwarebuild":   123,
				"on":              true,
				"brightness":      50,
				"temperature":     344,
				"ip":              "192.168.1.100",
				"port":            9123,
			},
			"light2": map[string]interface{}{
				"productname":     "Light 2",
				"serialnumber":    "TEST456",
				"firmwareversion": "1.0.0",
				"firmwarebuild":   123,
				"on":              false,
				"brightness":      0,
				"temperature":     143,
				"ip":              "192.168.1.101",
				"port":            9123,
			},
		},
	}

	// Create command
	cmd := newLightListCommand()
	cmd.Flags().Set("parseable", "true")

	// Create context with mock client
	ctx := context.WithValue(context.Background(), "client", mock)
	cmd.SetContext(ctx)

	// Test listing all lights
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)

	expected := `id="light1" productname="Light 1" serialnumber="TEST123" firmwareversion="1.0.0" firmwarebuild=123 on=true brightness=50 temperature=344 ip="192.168.1.100" port=9123
id="light2" productname="Light 2" serialnumber="TEST456" firmwareversion="1.0.0" firmwarebuild=123 on=false brightness=0 temperature=143 ip="192.168.1.101" port=9123
`
	assert.Equal(t, expected, buf.String())
}
