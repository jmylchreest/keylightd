package client

import (
	"encoding/json"
	"fmt"
	"github.com/jmylchreest/keylightd/internal/config"
	"log/slog"
	"net"
	"strconv"
	"time"
)

var dial = func(network, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, 10*time.Second)
}

// ClientInterface defines the methods for interacting with keylightd
// Used for testability and mocking in CLI

type ClientInterface interface {
	GetVersion() (map[string]any, error)
	GetLights() (map[string]any, error)
	GetLight(id string) (map[string]any, error)
	SetLightState(id string, property string, value any) error
	CreateGroup(name string) error
	GetGroup(name string) (map[string]any, error)
	GetGroups() ([]map[string]any, error)
	SetGroupState(name string, property string, value any) error
	DeleteGroup(name string) error
	SetGroupLights(groupID string, lightIDs []string) error
	AddAPIKey(name string, expiresInSeconds float64) (map[string]any, error)
	ListAPIKeys() ([]map[string]any, error)
	DeleteAPIKey(key string) error
	SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (map[string]any, error)
}

// Client represents a connection to keylightd
type Client struct {
	logger *slog.Logger
	socket string
}

// New creates a new client
func New(logger *slog.Logger, socket string) *Client {
	if socket == "" {
		// Use the shared config utility to get the socket path
		socket = config.GetRuntimeSocketPath()
		logger.Debug("Using default socket path", "socket", socket)
	} else {
		logger.Debug("Using provided socket path", "socket", socket)
	}

	return &Client{
		logger: logger,
		socket: socket,
	}
}

// request sends a request to keylightd and returns the response
func (c *Client) request(req any, resp any) error {
	c.logger.Debug("Connecting to socket", "socket", c.socket)
	// Connect to socket
	conn, err := dial("unix", c.socket)
	if err != nil {
		c.logger.Error("Failed to connect to socket", "error", err, "socket", c.socket)
		return fmt.Errorf("failed to connect to socket: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	c.logger.Debug("Encoding request", "request", req)
	// Encode request
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		c.logger.Error("Failed to encode request", "error", err)
		return fmt.Errorf("failed to encode request: %w", err)
	}

	c.logger.Debug("Waiting for response")
	// Decode response only if resp is not nil
	if resp != nil {
		if err := json.NewDecoder(conn).Decode(resp); err != nil {
			c.logger.Error("Failed to decode response", "error", err)
			return fmt.Errorf("failed to decode response: %w", err)
		}
		c.logger.Debug("Received response", "response", resp)

		// Check for error in response
		if respMap, ok := resp.(map[string]any); ok {
			if err, ok := respMap["error"].(string); ok {
				c.logger.Error("Server returned error", "error", err)
				return fmt.Errorf("server error: %s", err)
			}
			c.logger.Debug("Response processed successfully")
		}
	} else {
		// When resp is nil, we still need to read and check for errors
		var tempResp map[string]any
		if err := json.NewDecoder(conn).Decode(&tempResp); err != nil {
			c.logger.Error("Failed to decode response", "error", err)
			return fmt.Errorf("failed to decode response: %w", err)
		}
		c.logger.Debug("Received response (nil target)", "response", tempResp)

		// Check for error in response
		if err, ok := tempResp["error"].(string); ok {
			c.logger.Error("Server returned error", "error", err)
			return fmt.Errorf("server error: %s", err)
		}
		c.logger.Debug("Response processed successfully (nil target)")
	}

	return nil
}

// GetVersion returns the running daemon's version information.
func (c *Client) GetVersion() (map[string]any, error) {
	var resp map[string]any
	if err := c.request(map[string]string{"action": "version"}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetLights returns all discovered lights
func (c *Client) GetLights() (map[string]any, error) {
	var resp map[string]any
	if err := c.request(map[string]string{"action": "list_lights"}, &resp); err != nil {
		return nil, err
	}

	// The server returns {"lights": {id: lightMap, ...}}, so extract the 'lights' field
	lightsField, ok := resp["lights"]
	if !ok {
		return nil, fmt.Errorf("no lights field in response")
	}
	lightsMap, ok := lightsField.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid lights format in response")
	}

	// Iterate through lights and convert lastseen string to time.Time
	for _, lightData := range lightsMap {
		if lightMap, ok := lightData.(map[string]any); ok {
			if lastSeenStr, ok := lightMap["lastseen"].(string); ok {
				if t, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
					lightMap["lastseen"] = t
				} else {
					c.logger.Error("Failed to parse lastseen time string", "error", err, "value", lastSeenStr)
				}
			}
		}
	}

	return lightsMap, nil
}

// GetLight returns the state of a specific light
func (c *Client) GetLight(id string) (map[string]any, error) {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "get_light",
		"data":   map[string]any{"id": id},
	}, &resp); err != nil {
		return nil, err
	}

	// If the response is wrapped in a "light" field, extract it
	if light, ok := resp["light"].(map[string]any); ok {
		resp = light
	}

	// Convert lastseen string to time.Time
	if lastSeenStr, ok := resp["lastseen"].(string); ok {
		if t, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
			resp["lastseen"] = t
		} else {
			c.logger.Error("Failed to parse lastseen time string", "error", err, "value", lastSeenStr)
		}
	}

	return resp, nil
}

// SetLightState sets the state of a specific light
func (c *Client) SetLightState(id string, property string, value any) error {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "set_light_state",
		"data": map[string]any{
			"id":       id,
			"property": property,
			"value":    value,
		},
	}, &resp); err != nil {
		return err
	}
	return nil
}

// CreateGroup creates a new group of lights
func (c *Client) CreateGroup(name string) error {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "create_group",
		"data":   map[string]any{"name": name},
	}, &resp); err != nil {
		return err
	}

	// If the response is wrapped in a "group" field, extract it
	if group, ok := resp["group"].(map[string]any); ok {
		resp = group
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
func (c *Client) GetGroup(id string) (map[string]any, error) {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "get_group",
		"data":   map[string]any{"id": id},
	}, &resp); err != nil {
		return nil, err
	}

	// If the response is wrapped in a "group" field, extract it
	if group, ok := resp["group"].(map[string]any); ok {
		resp = group
	}

	return resp, nil
}

// GetGroups returns all groups
func (c *Client) GetGroups() ([]map[string]any, error) {
	var resp map[string]any
	if err := c.request(map[string]string{
		"action": "list_groups",
	}, &resp); err != nil {
		return nil, err
	}

	groupsField, ok := resp["groups"]
	if !ok {
		return nil, nil
	}
	groupsSlice, ok := groupsField.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid groups format in response")
	}

	groups := make([]map[string]any, 0, len(groupsSlice))
	for _, g := range groupsSlice {
		if groupMap, ok := g.(map[string]any); ok {
			groups = append(groups, groupMap)
		}
	}
	return groups, nil
}

// SetGroupState sets the state of all lights in a group
func (c *Client) SetGroupState(id string, property string, value any) error {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "set_group_state",
		"data": map[string]any{
			"id":       id,
			"property": property,
			"value":    value,
		},
	}, &resp); err != nil {
		return err
	}
	return nil
}

// DeleteGroup deletes a group of lights
func (c *Client) DeleteGroup(id string) error {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "delete_group",
		"data":   map[string]any{"id": id},
	}, &resp); err != nil {
		return err
	}
	return nil
}

// SetGroupLights sets the lights in a group
func (c *Client) SetGroupLights(groupID string, lightIDs []string) error {
	var resp map[string]any
	if err := c.request(map[string]any{
		"action": "set_group_lights",
		"data": map[string]any{
			"id":     groupID,
			"lights": lightIDs,
		},
	}, &resp); err != nil {
		return err
	}

	// Check for error in response
	if err, ok := resp["error"].(string); ok {
		return fmt.Errorf("server error: %s", err)
	}

	return nil
}

// API Key Management Methods

// AddAPIKey tells keylightd to add a new API key.
func (c *Client) AddAPIKey(name string, expiresInSeconds float64) (map[string]any, error) {
	// Server expects: { "action": "apikey_add", "data": { "name": "...". "expires_in": "..." } }
	reqData := map[string]any{
		"name": name,
	}
	if expiresInSeconds > 0 {
		reqData["expires_in"] = fmt.Sprintf("%f", expiresInSeconds) // Server socket handler expects string seconds
	}

	apiRequest := map[string]any{
		"action": "apikey_add",
		"data":   reqData,
	}

	var serverResponse map[string]any
	if err := c.request(apiRequest, &serverResponse); err != nil {
		return nil, err
	}

	// Server sends: {"status": "ok", "id": "req_id_optional", "key": APIKeyObject}
	// The c.request method handles the general structure and potential top-level "error" field.
	// If we are here, c.request did not return an error, implying basic success.

	apiKeyData, ok := serverResponse["key"].(map[string]any)
	if !ok {
		// This case implies the server response was successful at a transport level,
		// but the expected "key" field containing the APIKey details is missing.
		// This indicates an unexpected response structure from the server for a successful apikey_add operation.
		c.logger.Error("apikey_add response missing 'key' field", "response", serverResponse)
		return nil, fmt.Errorf("server response for apikey_add missing 'key' field: %+v", serverResponse)
	}

	// Parse time strings in the returned key data, similar to ListAPIKeys
	for _, field := range []string{"created_at", "expires_at", "last_used_at"} {
		if valStr, ok := apiKeyData[field].(string); ok {
			// Handle zero time explicitly
			if valStr == "0001-01-01T00:00:00Z" {
				apiKeyData[field] = time.Time{} // Set to zero time
				continue
			}
			// Try parsing with RFC3339Nano first, then RFC3339
			if t, err := time.Parse(time.RFC3339Nano, valStr); err == nil {
				apiKeyData[field] = t
			} else if t, err := time.Parse(time.RFC3339, valStr); err == nil {
				apiKeyData[field] = t
			} else if valStr != "" { // Only warn if it's not an empty string that failed parsing
				c.logger.Warn("Failed to parse time string in AddAPIKey response", "field", field, "value", valStr, "error", err)
				// If parsing fails for a non-empty string, keep the original string
				apiKeyData[field] = valStr
			}
		}
	}

	return apiKeyData, nil
}

// ListAPIKeys lists all API keys
func (c *Client) ListAPIKeys() ([]map[string]any, error) {
	// Expect the server's wrapper object { "status": "ok", "keys": [...] }
	var serverResponse map[string]any
	if err := c.request(map[string]string{
		"action": "apikey_list",
	}, &serverResponse); err != nil {
		return nil, err
	}

	// Extract the actual list of keys from the "keys" field
	keysData, ok := serverResponse["keys"].([]any)
	if !ok {
		if errMsg, hasErr := serverResponse["error"].(string); hasErr {
			return nil, fmt.Errorf("server error: %s", errMsg)
		}
		return nil, fmt.Errorf("failed to parse 'keys' field from server response: %v", serverResponse)
	}

	apiKeys := make([]map[string]any, 0, len(keysData))
	for _, keyEntry := range keysData {
		keyMap, ok := keyEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid API key entry in server response: %v", keyEntry)
		}
		apiKeys = append(apiKeys, keyMap)
	}

	// Post-process response: parse time strings
	for _, keyData := range apiKeys { // Iterate over the extracted apiKeys
		for _, field := range []string{"created_at", "expires_at", "last_used_at"} {
			if valStr, ok := keyData[field].(string); ok {
				// Handle zero time explicitly
				if valStr == "0001-01-01T00:00:00Z" {
					keyData[field] = time.Time{} // Set to zero time
					continue
				}
				// Try parsing with RFC3339Nano first, then RFC3339
				if t, err := time.Parse(time.RFC3339Nano, valStr); err == nil {
					keyData[field] = t
				} else if t, err := time.Parse(time.RFC3339, valStr); err == nil {
					keyData[field] = t
				} else if valStr != "" { // Only warn if it's not an empty string that failed parsing
					c.logger.Warn("Failed to parse time string for API key", "field", field, "value", valStr, "error", err)
					// If parsing fails for a non-empty string, keep the original string
					keyData[field] = valStr
				}
			} else if _, isTime := keyData[field].(time.Time); isTime {
				// It's already a time.Time object, possibly from AddAPIKey response processing.
				// No action needed.
			}
		}
	}
	return apiKeys, nil
}

// DeleteAPIKey deletes an API key by its value
func (c *Client) DeleteAPIKey(key string) error {
	payload := map[string]any{
		"action": "apikey_delete",
		"data":   map[string]any{"key": key},
	}
	// We expect a response like {"status": "ok"} or an error response.
	// The generic response handling in c.request will return an error if the server sends one.
	var resp map[string]any
	return c.request(payload, &resp) // Using a concrete type for response to avoid Unmarshal(nil) error
}

// SetAPIKeyDisabledStatus sends a request to enable or disable an API key.
func (c *Client) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (map[string]any, error) {
	payload := map[string]any{
		"action": "apikey_set_disabled_status",
		"data": map[string]any{
			"key_or_name": keyOrName,
			"disabled":    strconv.FormatBool(disabled),
		},
	}
	var respData map[string]any
	if err := c.request(payload, &respData); err != nil {
		return nil, err
	}

	// Based on server.go, the response for this action is:
	// s.sendResponse(conn, id, map[string]any{"status": "ok", "key": updatedKey})
	// The c.request method handles the outer envelope, so respData here should be the map sent as the third arg to sendResponse.
	// Thus, respData should be map[string]any{"status":"ok", "key": map[string]any{...}}
	// We need to extract the nested "key" map which contains the actual APIKey fields.
	updatedKeyData, ok := respData["key"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format, missing 'key' field containing API key details in response data: %+v", respData)
	}
	return updatedKeyData, nil
}
