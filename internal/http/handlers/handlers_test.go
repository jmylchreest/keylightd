package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock light manager ---

type mockLightManager struct {
	lights map[string]*keylight.Light
}

func (m *mockLightManager) GetLights() map[string]*keylight.Light        { return m.lights }
func (m *mockLightManager) GetDiscoveredLights() []*keylight.Light       { return nil }
func (m *mockLightManager) AddLight(_ context.Context, _ keylight.Light) {}
func (m *mockLightManager) StartCleanupWorker(_ context.Context, _ time.Duration, _ time.Duration) {
}

func (m *mockLightManager) GetLight(_ context.Context, id string) (*keylight.Light, error) {
	l, ok := m.lights[id]
	if !ok {
		return nil, fmt.Errorf("light %s not found", id)
	}
	return l, nil
}

func (m *mockLightManager) SetLightState(_ context.Context, id string, pv keylight.LightPropertyValue) error {
	l, ok := m.lights[id]
	if !ok {
		return fmt.Errorf("light %s not found", id)
	}
	if err := pv.Validate(); err != nil {
		return err
	}
	switch pv.PropertyName() {
	case keylight.PropertyOn:
		l.On = pv.Value().(bool)
	case keylight.PropertyBrightness:
		l.Brightness = pv.Value().(int)
	case keylight.PropertyTemperature:
		l.Temperature = pv.Value().(int)
	}
	return nil
}

func (m *mockLightManager) SetLightBrightness(ctx context.Context, id string, b int) error {
	return m.SetLightState(ctx, id, keylight.BrightnessValue(b))
}
func (m *mockLightManager) SetLightTemperature(ctx context.Context, id string, t int) error {
	return m.SetLightState(ctx, id, keylight.TemperatureValue(t))
}
func (m *mockLightManager) SetLightPower(ctx context.Context, id string, on bool) error {
	return m.SetLightState(ctx, id, keylight.OnValue(on))
}

var _ keylight.LightManager = (*mockLightManager)(nil)

func newMockLights() *mockLightManager {
	return &mockLightManager{
		lights: map[string]*keylight.Light{
			"light-1": {
				ID: "light-1", Name: "Test Light 1",
				IP: net.ParseIP("192.168.1.1"), Port: 9123,
				Brightness: 50, Temperature: 5000, On: true,
				LastSeen: time.Now(),
			},
			"light-2": {
				ID: "light-2", Name: "Test Light 2",
				IP: net.ParseIP("192.168.1.2"), Port: 9123,
				Brightness: 75, Temperature: 4000, On: false,
				LastSeen: time.Now(),
			},
		},
	}
}

// === Health Handler Tests ===

func TestHealthCheck(t *testing.T) {
	out, err := HealthCheck(context.Background(), &HealthInput{})
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Body.Status)
}

// === Light Handler Tests ===

func TestLightHandler_ListLights(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	out, err := handler.ListLights(context.Background(), &ListLightsInput{})
	require.NoError(t, err)
	assert.Len(t, out.Body, 2)
	assert.Contains(t, out.Body, "light-1")
	assert.Contains(t, out.Body, "light-2")
	assert.Equal(t, "Test Light 1", out.Body["light-1"].Name)
	assert.Equal(t, 50, out.Body["light-1"].Brightness)
}

func TestLightHandler_ListLights_Empty(t *testing.T) {
	handler := &LightHandler{Lights: &mockLightManager{lights: map[string]*keylight.Light{}}}

	out, err := handler.ListLights(context.Background(), &ListLightsInput{})
	require.NoError(t, err)
	assert.Empty(t, out.Body)
}

func TestLightHandler_GetLight(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	out, err := handler.GetLight(context.Background(), &GetLightInput{ID: "light-1"})
	require.NoError(t, err)
	assert.Equal(t, "light-1", out.Body.ID)
	assert.Equal(t, "Test Light 1", out.Body.Name)
	assert.Equal(t, 50, out.Body.Brightness)
	assert.True(t, out.Body.On)
}

func TestLightHandler_GetLight_NotFound(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	_, err := handler.GetLight(context.Background(), &GetLightInput{ID: "no-such"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLightHandler_SetLightState_On(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	on := false
	out, err := handler.SetLightState(context.Background(), &SetLightStateInput{
		ID: "light-1",
		Body: struct {
			On          *bool `json:"on,omitempty" doc:"Power state"`
			Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100)"`
			Temperature *int  `json:"temperature,omitempty" doc:"Color temperature in Kelvin"`
		}{On: &on},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Body.Status)
	assert.False(t, lights.lights["light-1"].On)
}

func TestLightHandler_SetLightState_Brightness(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	brightness := 80
	out, err := handler.SetLightState(context.Background(), &SetLightStateInput{
		ID: "light-1",
		Body: struct {
			On          *bool `json:"on,omitempty" doc:"Power state"`
			Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100)"`
			Temperature *int  `json:"temperature,omitempty" doc:"Color temperature in Kelvin"`
		}{Brightness: &brightness},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Body.Status)
	assert.Equal(t, 80, lights.lights["light-1"].Brightness)
}

func TestLightHandler_SetLightState_MultipleProperties(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	on := true
	brightness := 90
	out, err := handler.SetLightState(context.Background(), &SetLightStateInput{
		ID: "light-2",
		Body: struct {
			On          *bool `json:"on,omitempty" doc:"Power state"`
			Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100)"`
			Temperature *int  `json:"temperature,omitempty" doc:"Color temperature in Kelvin"`
		}{On: &on, Brightness: &brightness},
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", out.Body.Status)
	assert.True(t, lights.lights["light-2"].On)
	assert.Equal(t, 90, lights.lights["light-2"].Brightness)
}

func TestLightHandler_SetLightState_NotFound(t *testing.T) {
	lights := newMockLights()
	handler := &LightHandler{Lights: lights}

	on := true
	_, err := handler.SetLightState(context.Background(), &SetLightStateInput{
		ID: "no-such",
		Body: struct {
			On          *bool `json:"on,omitempty" doc:"Power state"`
			Brightness  *int  `json:"brightness,omitempty" doc:"Brightness level (0-100)"`
			Temperature *int  `json:"temperature,omitempty" doc:"Color temperature in Kelvin"`
		}{On: &on},
	})
	assert.Error(t, err)
}

// === Type Conversion Tests ===

func TestLightFromKeylight(t *testing.T) {
	l := &keylight.Light{
		ID:              "l1",
		Name:            "Light 1",
		IP:              net.ParseIP("10.0.0.1"),
		Port:            9123,
		Brightness:      42,
		Temperature:     300,
		On:              true,
		ProductName:     "Key Light",
		SerialNumber:    "SN123",
		FirmwareVersion: "1.0.0",
	}

	resp := LightFromKeylight(l)
	assert.Equal(t, "l1", resp.ID)
	assert.Equal(t, "10.0.0.1", resp.IP)
	assert.Equal(t, 9123, resp.Port)
	assert.Equal(t, 42, resp.Brightness)
	assert.True(t, resp.On)
	assert.Equal(t, "Key Light", resp.ProductName)
	assert.Equal(t, "SN123", resp.SerialNumber)
}

func TestLightsMapFromKeylight(t *testing.T) {
	lights := map[string]*keylight.Light{
		"a": {ID: "a", Name: "A", IP: net.ParseIP("1.1.1.1")},
		"b": {ID: "b", Name: "B", IP: net.ParseIP("2.2.2.2")},
	}
	result := LightsMapFromKeylight(lights)
	assert.Len(t, result, 2)
	assert.Equal(t, "A", result["a"].Name)
	assert.Equal(t, "B", result["b"].Name)
}

func TestLightsMapFromKeylight_Empty(t *testing.T) {
	result := LightsMapFromKeylight(map[string]*keylight.Light{})
	assert.Empty(t, result)
}

func TestGroupFromInternal(t *testing.T) {
	g := &group.Group{ID: "g1", Name: "Office", Lights: []string{"l1", "l2"}}
	resp := GroupFromInternal(g)
	assert.Equal(t, "g1", resp.ID)
	assert.Equal(t, "Office", resp.Name)
	assert.Equal(t, []string{"l1", "l2"}, resp.Lights)
}

func TestGroupFromInternal_NilLights(t *testing.T) {
	g := &group.Group{ID: "g2", Name: "Empty", Lights: nil}
	resp := GroupFromInternal(g)
	assert.Equal(t, []string{}, resp.Lights, "nil lights should become empty slice")
}

func TestGroupsFromInternal(t *testing.T) {
	groups := []*group.Group{
		{ID: "g1", Name: "A", Lights: []string{"l1"}},
		{ID: "g2", Name: "B", Lights: nil},
	}
	result := GroupsFromInternal(groups)
	assert.Len(t, result, 2)
	assert.Equal(t, "A", result[0].Name)
	assert.Equal(t, []string{}, result[1].Lights)
}

// === Logging Handler Tests ===

func TestLevelToString(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warn"},
		{slog.LevelError, "error"},
		{slog.LevelDebug - 4, "debug"}, // below debug
		{slog.LevelError + 4, "error"}, // above error
	}
	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, LevelToString(tc.level))
		})
	}
}

// === joinStrings Tests ===

func TestJoinStrings(t *testing.T) {
	assert.Equal(t, "", joinStrings(nil, "; "))
	assert.Equal(t, "a", joinStrings([]string{"a"}, "; "))
	assert.Equal(t, "a; b; c", joinStrings([]string{"a", "b", "c"}, "; "))
}
