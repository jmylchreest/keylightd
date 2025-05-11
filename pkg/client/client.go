package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

var dial = net.Dial

// ClientInterface defines the methods for interacting with keylightd
// Used for testability and mocking in CLI

type ClientInterface interface {
	GetLights() (map[string]interface{}, error)
	GetLight(id string) (map[string]interface{}, error)
	SetLightState(id string, property string, value interface{}) error
	CreateGroup(name string) error
	GetGroup(name string) (map[string]interface{}, error)
	GetGroups() ([]map[string]interface{}, error)
	SetGroupState(name string, property string, value interface{}) error
	DeleteGroup(name string) error
	SetGroupLights(groupID string, lightIDs []string) error
}

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
			logger.Debug("Using XDG runtime directory for socket", "dir", dir, "socket", socket)
		} else {
			uid := os.Getuid()
			socket = filepath.Join("/run/user", fmt.Sprintf("%d", uid), "keylightd.sock")
			logger.Debug("Using /run/user for socket", "uid", uid, "socket", socket)
		}
	} else {
		logger.Debug("Using provided socket path", "socket", socket)
	}

	return &Client{
		logger: logger,
		socket: socket,
	}
}

// request sends a request to keylightd and returns the response
func (c *Client) request(req interface{}, resp interface{}) error {
	c.logger.Debug("Connecting to socket", "socket", c.socket)
	// Connect to socket
	conn, err := dial("unix", c.socket)
	if err != nil {
		c.logger.Error("Failed to connect to socket", "error", err, "socket", c.socket)
		return fmt.Errorf("failed to connect to socket: %w", err)
	}
	defer conn.Close()

	c.logger.Debug("Encoding request", "request", req)
	// Encode request
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		c.logger.Error("Failed to encode request", "error", err)
		return fmt.Errorf("failed to encode request: %w", err)
	}

	c.logger.Debug("Waiting for response")
	// Decode response
	if err := json.NewDecoder(conn).Decode(resp); err != nil {
		c.logger.Error("Failed to decode response", "error", err)
		return fmt.Errorf("failed to decode response: %w", err)
	}
	c.logger.Debug("Received response", "response", resp)

	// Check for error in response
	if respMap, ok := resp.(map[string]interface{}); ok {
		if err, ok := respMap["error"].(string); ok {
			c.logger.Error("Server returned error", "error", err)
			return fmt.Errorf("server error: %s", err)
		}
		c.logger.Debug("Response processed successfully")
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

	// Check for error in response
	if err, ok := resp["error"].(string); ok {
		return fmt.Errorf("server error: %s", err)
	}

	// Log success
	c.logger.Debug("Group created successfully", "name", name, "response", resp)
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

// GetGroups returns all groups
func (c *Client) GetGroups() ([]map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.request(map[string]string{
		"action": "list_groups",
	}, &resp); err != nil {
		return nil, err
	}

	// Handle nil response
	if resp == nil {
		return []map[string]interface{}{}, nil
	}

	// Convert map to slice of maps
	groups := make([]map[string]interface{}, 0, len(resp))
	for id, group := range resp {
		if groupMap, ok := group.(map[string]interface{}); ok {
			groupMap["id"] = id
			groups = append(groups, groupMap)
		}
	}
	return groups, nil
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

// DeleteGroup deletes a group of lights
func (c *Client) DeleteGroup(name string) error {
	var resp map[string]interface{}
	if err := c.request(map[string]string{
		"action": "delete_group",
		"name":   name,
	}, &resp); err != nil {
		return err
	}
	return nil
}

// SetGroupLights sets the lights in a group
func (c *Client) SetGroupLights(groupID string, lightIDs []string) error {
	var resp map[string]interface{}
	if err := c.request(map[string]interface{}{
		"action": "set_group_lights",
		"id":     groupID,
		"lights": lightIDs,
	}, &resp); err != nil {
		return err
	}

	// Check for error in response
	if err, ok := resp["error"].(string); ok {
		return fmt.Errorf("server error: %s", err)
	}

	return nil
}
