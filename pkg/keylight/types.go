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
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	IP                net.IP      `json:"ip"`
	Port              int         `json:"port"`
	Temperature       int         `json:"temperature"`
	Brightness        int         `json:"brightness"`
	On                bool        `json:"on"`
	ProductName       string      `json:"productname"`
	HardwareBoardType int         `json:"hardwareboardtype"`
	FirmwareVersion   string      `json:"firmwareversion"`
	FirmwareBuild     int         `json:"firmwarebuild"`
	SerialNumber      string      `json:"serialnumber"`
	State             *LightState `json:"state,omitempty"`
	LastSeen          time.Time   `json:"lastseen"`
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
