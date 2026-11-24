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

// PowerInfo describes the device's reported operating mode and brightness limit.
type PowerInfo struct {
	OperationMode     int `json:"operationMode"`
	MaximumBrightness int `json:"maximumBrightness"`
}

// Light represents a Key Light device
type Light struct {
	NeedsConfiguration []string     `json:"needsConfiguration,omitempty"`
	BatteryInfo        *BatteryInfo `json:"battery_info,omitempty"`
	Manufacturer       string       `json:"manufacturer,omitempty"`
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	IP                 net.IP       `json:"ip"`
	Port               int          `json:"port"`
	Temperature        int          `json:"temperature"`
	Brightness         int          `json:"brightness"`
	On                 bool         `json:"on"`
	Model              string       `json:"model,omitempty"`
	Experimental       bool         `json:"experimental"`
	ProductName        string       `json:"productname"`
	HardwareBoardType  int          `json:"hardwareboardtype"`
	FirmwareVersion    string       `json:"firmwareversion"`
	FirmwareBuild      int          `json:"firmwarebuild"`
	SerialNumber       string       `json:"serialnumber"`
	State              *LightState  `json:"state,omitempty"`
	LastSeen           time.Time    `json:"lastseen"`
	PowerInfo          *PowerInfo   `json:"power_info,omitempty"`
}

// LightManager defines the interface for managing Keylight devices
type LightManager interface {
	GetDiscoveredLights() []*Light
	GetLight(ctx context.Context, id string) (*Light, error)
	SetLightState(ctx context.Context, id string, propertyValue LightPropertyValue) error
	SetLightBrightness(ctx context.Context, id string, brightness int) error
	SetLightTemperature(ctx context.Context, id string, temperature int) error
	SetLightPower(ctx context.Context, id string, on bool) error
	GetLights() map[string]*Light
	AddLight(ctx context.Context, light Light)
	StartCleanupWorker(ctx context.Context, cleanupInterval time.Duration, timeout time.Duration)
}

// DiscoveryEvent represents an event from the mDNS discovery process
type DiscoveryEvent struct {
	Type  string // "add", "remove", "update"
	Light *Light
}
