// Package handlers provides typed Huma request/response structs and handler
// implementations for the keylightd HTTP API.
package handlers

import (
	"time"

	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// --- Light types ---

// LightResponse is the API representation of a discovered light.
type LightResponse struct {
	ID                string    `json:"id" doc:"Unique light identifier"`
	Name              string    `json:"name" doc:"Display name of the light"`
	IP                string    `json:"ip" doc:"IP address of the light"`
	Port              int       `json:"port" doc:"Port number of the light"`
	Temperature       int       `json:"temperature" doc:"Color temperature in mireds"`
	Brightness        int       `json:"brightness" doc:"Brightness level (0-100)"`
	On                bool      `json:"on" doc:"Whether the light is currently on"`
	ProductName       string    `json:"productname" doc:"Product name"`
	HardwareBoardType int       `json:"hardwareboardtype" doc:"Hardware board type identifier"`
	FirmwareVersion   string    `json:"firmwareversion" doc:"Firmware version string"`
	FirmwareBuild     int       `json:"firmwarebuild" doc:"Firmware build number"`
	SerialNumber      string    `json:"serialnumber" doc:"Serial number"`
	LastSeen          time.Time `json:"lastseen" doc:"Last time the light was seen on the network"`
}

// LightFromKeylight converts a keylight.Light to a LightResponse.
func LightFromKeylight(l *keylight.Light) LightResponse {
	return LightResponse{
		ID:                l.ID,
		Name:              l.Name,
		IP:                l.IP.String(),
		Port:              l.Port,
		Temperature:       l.Temperature,
		Brightness:        l.Brightness,
		On:                l.On,
		ProductName:       l.ProductName,
		HardwareBoardType: l.HardwareBoardType,
		FirmwareVersion:   l.FirmwareVersion,
		FirmwareBuild:     l.FirmwareBuild,
		SerialNumber:      l.SerialNumber,
		LastSeen:          l.LastSeen,
	}
}

// LightsMapFromKeylight converts the keylight manager's map to our API map.
// The GNOME extension expects lights as map[string]*Light (object keyed by ID).
func LightsMapFromKeylight(lights map[string]*keylight.Light) map[string]LightResponse {
	result := make(map[string]LightResponse, len(lights))
	for id, l := range lights {
		result[id] = LightFromKeylight(l)
	}
	return result
}

// --- Group types ---

// GroupResponse is the API representation of a light group.
type GroupResponse struct {
	ID     string   `json:"id" doc:"Unique group identifier (UUID)"`
	Name   string   `json:"name" doc:"Display name of the group"`
	Lights []string `json:"lights" doc:"List of light IDs in this group"`
}

// GroupFromInternal converts a group.Group to a GroupResponse.
func GroupFromInternal(g *group.Group) GroupResponse {
	lights := g.Lights
	if lights == nil {
		lights = []string{}
	}
	return GroupResponse{
		ID:     g.ID,
		Name:   g.Name,
		Lights: lights,
	}
}

// GroupsFromInternal converts a slice of group.Group to GroupResponses.
func GroupsFromInternal(groups []*group.Group) []GroupResponse {
	result := make([]GroupResponse, len(groups))
	for i, g := range groups {
		result[i] = GroupFromInternal(g)
	}
	return result
}

// --- API Key types ---

// APIKeyResponse is the API representation of an API key.
type APIKeyResponse struct {
	ID        string    `json:"id" doc:"Key identifier"`
	Name      string    `json:"name" doc:"Display name of the key"`
	Key       string    `json:"key,omitempty" doc:"Full key string (only present on creation)"`
	CreatedAt time.Time `json:"created_at" doc:"When the key was created"`
	ExpiresAt time.Time `json:"expires_at" doc:"When the key expires"`
}

// --- Common response types ---

// StatusResponse is a simple status response.
type StatusResponse struct {
	Status string `json:"status" doc:"Operation status"`
}

// PartialStatusResponse is returned when some operations in a batch succeed and others fail.
type PartialStatusResponse struct {
	Status string   `json:"status" doc:"Operation status (partial)"`
	Errors []string `json:"errors" doc:"List of errors for failed operations"`
}
