package keylight

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeDiscovery func(func(Light, DeviceClient))

func (f fakeDiscovery) Discover(_ context.Context, _ time.Duration, found func(Light, DeviceClient)) error {
	f(found)
	return nil
}

type fakeDevice struct {
	state LightState
}

func (d *fakeDevice) GetAccessoryInfo(context.Context) (*AccessoryInfo, error) {
	return &AccessoryInfo{ProductName: "Test light", SerialNumber: "test"}, nil
}

func (d *fakeDevice) GetLightState(context.Context) (*LightState, error) {
	return &d.state, nil
}

func (d *fakeDevice) SetLightState(_ context.Context, on bool, brightness, temperature int) error {
	return nil
}

func TestDiscoverWithUsesProviderClient(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(discardLogger())
	device := &fakeDevice{}
	device.state.NumberOfLights = 1
	require.NoError(t, manager.DiscoverWith(ctx, time.Second, fakeDiscovery(func(found func(Light, DeviceClient)) {
		found(Light{ID: "test", Manufacturer: "other"}, device)
	})))
	light, err := manager.GetLight(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, "other", light.Manufacturer)
	require.Equal(t, "test", light.SerialNumber)
	client, _, err := manager.deviceClient("test")
	require.NoError(t, err)
	require.Same(t, device, client)
}
