package keylight

import (
	"context"
	"time"
)

// DeviceClient reads and updates one light. Implementations translate device
// protocols into the state and units used by the manager.
type DeviceClient interface {
	GetAccessoryInfo(context.Context) (*AccessoryInfo, error)
	GetLightState(context.Context) (*LightState, error)
	SetLightState(ctx context.Context, on bool, brightness, temperature int) error
}

// DiscoveryProvider reports validated lights and their bound clients. Discover
// runs until cancellation or failure and must stop callbacks before returning.
type DiscoveryProvider interface {
	Discover(ctx context.Context, interval time.Duration, found func(Light, DeviceClient)) error
}
