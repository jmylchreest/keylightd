package keylight

import (
	"context"
	"errors"
	"net"
	"time"
)

// Common errors
var (
	ErrLightNotFound = errors.New("light not found")
)

// Light represents a Key Light device
type Light struct {
	ID                string
	Name              string
	IP                net.IP
	Port              int
	Temperature       int
	Brightness        int
	On                bool
	ProductName       string
	HardwareBoardType int
	FirmwareVersion   string
	FirmwareBuild     int
	SerialNumber      string
	State             *LightState
	LastSeen          time.Time // Timestamp of the last successful communication
}

// LightManager defines the interface for managing Keylight devices
type LightManager interface {
	GetDiscoveredLights() []*Light
	GetLight(id string) (*Light, error)
	SetLightState(id string, property string, value interface{}) error
	SetLightBrightness(id string, brightness int) error
	SetLightTemperature(id string, temperature int) error
	SetLightPower(id string, on bool) error
	GetLights() map[string]*Light
	AddLight(light Light)
	StartCleanupWorker(ctx context.Context, cleanupInterval time.Duration, timeout time.Duration)
}

// DiscoveryEvent represents an event from the mDNS discovery process
type DiscoveryEvent struct {
	Type  string // "add", "remove", "update"
	Light *Light
}
