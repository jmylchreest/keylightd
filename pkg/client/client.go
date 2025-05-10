package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

// Client represents a connection to keylightd
type Client struct {
	logger *slog.Logger
	socket string
}

// New creates a new client
func New(logger *slog.Logger, socket string) *Client {
	if socket == "" {
		// Use XDG runtime directory
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			socket = filepath.Join(dir, "keylightd.sock")
		} else {
			uid := os.Getuid()
			socket = filepath.Join("/run/user", fmt.Sprintf("%d", uid), "keylightd.sock")
		}
	}

	return &Client{
		logger: logger,
		socket: socket,
	}
}

// request sends a request to keylightd and returns the response
func (c *Client) request(req interface{}, resp interface{}) error {
	// Connect to socket
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		return fmt.Errorf("failed to connect to socket: %w", err)
	}
	defer conn.Close()

	// Encode request
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	// Decode response
	if err := json.NewDecoder(conn).Decode(resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetLights returns all discovered lights
func (c *Client) GetLights() (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.request(map[string]string{"action": "list_lights"}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetLight returns the state of a specific light
func (c *Client) GetLight(id string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.request(map[string]string{
		"action": "get_light",
		"id":     id,
	}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SetLightState sets the state of a specific light
func (c *Client) SetLightState(id string, property string, value interface{}) error {
	var resp map[string]interface{}
	if err := c.request(map[string]interface{}{
		"action":   "set_light",
		"id":       id,
		"property": property,
		"value":    value,
	}, &resp); err != nil {
		return err
	}
	return nil
}

// CreateGroup creates a new group of lights
func (c *Client) CreateGroup(name string) error {
	var resp map[string]interface{}
	if err := c.request(map[string]string{
		"action": "create_group",
		"name":   name,
	}, &resp); err != nil {
		return err
	}
	return nil
}

// GetGroup returns the state of all lights in a group
func (c *Client) GetGroup(name string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.request(map[string]string{
		"action": "get_group",
		"name":   name,
	}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SetGroupState sets the state of all lights in a group
func (c *Client) SetGroupState(name string, property string, value interface{}) error {
	var resp map[string]interface{}
	if err := c.request(map[string]interface{}{
		"action":   "set_group",
		"name":     name,
		"property": property,
		"value":    value,
	}, &resp); err != nil {
		return err
	}
	return nil
}
