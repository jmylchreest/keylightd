package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// HTTPClient represents an HTTP connection to keylightd
type HTTPClient struct {
	logger  *slog.Logger
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewHTTP creates a new HTTP client
func NewHTTP(logger *slog.Logger, baseURL string, apiKey string) *HTTPClient {
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &HTTPClient{
		logger:  logger,
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// request performs an HTTP request and decodes the JSON response
func (c *HTTPClient) request(method, path string, body any, resp any) error {
	url := c.baseURL + path
	c.logger.Debug("HTTP request", "method", method, "url", url)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	// Execute request
	httpResp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("HTTP request failed", "error", err)
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error status codes
	if httpResp.StatusCode >= 400 {
		c.logger.Error("HTTP error response", "status", httpResp.StatusCode, "body", string(respBody))
		return fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Decode response if needed
	if resp != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, resp); err != nil {
			c.logger.Error("Failed to decode response", "error", err, "body", string(respBody))
			return fmt.Errorf("failed to decode response: %w", err)
		}
		c.logger.Debug("Received response", "response", resp)
	}

	return nil
}

// GetVersion returns the running daemon's version information.
func (c *HTTPClient) GetVersion() (map[string]any, error) {
	var resp map[string]any
	if err := c.request("GET", "/api/v1/version", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetLights returns all lights
func (c *HTTPClient) GetLights() (map[string]any, error) {
	var resp map[string]any
	err := c.request("GET", "/api/v1/lights", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetLight returns a specific light
func (c *HTTPClient) GetLight(id string) (map[string]any, error) {
	var resp map[string]any
	err := c.request("GET", "/api/v1/lights/"+id, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// SetLightState sets a property on a light
func (c *HTTPClient) SetLightState(id string, property string, value any) error {
	body := map[string]any{
		property: value,
	}
	return c.request("POST", "/api/v1/lights/"+id+"/state", body, nil)
}

// CreateGroup creates a new group
func (c *HTTPClient) CreateGroup(name string) error {
	body := map[string]any{
		"name": name,
	}
	return c.request("POST", "/api/v1/groups", body, nil)
}

// GetGroup returns a specific group
func (c *HTTPClient) GetGroup(id string) (map[string]any, error) {
	var resp map[string]any
	err := c.request("GET", "/api/v1/groups/"+id, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetGroups returns all groups
func (c *HTTPClient) GetGroups() ([]map[string]any, error) {
	var resp []map[string]any
	err := c.request("GET", "/api/v1/groups", nil, &resp)
	if err != nil {
		return nil, err
	}
	// Ensure we return an empty slice instead of nil
	if resp == nil {
		return []map[string]any{}, nil
	}
	return resp, nil
}

// SetGroupState sets a property on all lights in a group
func (c *HTTPClient) SetGroupState(id string, property string, value any) error {
	body := map[string]any{
		property: value,
	}
	return c.request("PUT", "/api/v1/groups/"+id+"/state", body, nil)
}

// DeleteGroup deletes a group
func (c *HTTPClient) DeleteGroup(id string) error {
	return c.request("DELETE", "/api/v1/groups/"+id, nil, nil)
}

// SetGroupLights sets the lights in a group
func (c *HTTPClient) SetGroupLights(groupID string, lightIDs []string) error {
	body := map[string]any{
		"light_ids": lightIDs,
	}
	return c.request("PUT", "/api/v1/groups/"+groupID+"/lights", body, nil)
}

// AddAPIKey creates a new API key
func (c *HTTPClient) AddAPIKey(name string, expiresInSeconds float64) (map[string]any, error) {
	body := map[string]any{
		"name": name,
	}
	if expiresInSeconds > 0 {
		body["expires_in"] = fmt.Sprintf("%.0fs", expiresInSeconds)
	}
	var resp map[string]any
	err := c.request("POST", "/api/v1/apikeys", body, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListAPIKeys returns all API keys
func (c *HTTPClient) ListAPIKeys() ([]map[string]any, error) {
	var resp []map[string]any
	err := c.request("GET", "/api/v1/apikeys", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteAPIKey deletes an API key
func (c *HTTPClient) DeleteAPIKey(key string) error {
	return c.request("DELETE", "/api/v1/apikeys/"+key, nil, nil)
}

// SetAPIKeyDisabledStatus enables or disables an API key
func (c *HTTPClient) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (map[string]any, error) {
	body := map[string]any{
		"disabled": disabled,
	}
	var resp map[string]any
	err := c.request("PUT", "/api/v1/apikeys/"+keyOrName+"/disabled", body, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
