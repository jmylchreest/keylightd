package keylight

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// LightState represents the state of a Key Light
type LightState struct {
	NumberOfLights int `json:"numberOfLights"`
	Lights         []struct {
		On          int `json:"on"`
		Brightness  int `json:"brightness"`
		Temperature int `json:"temperature"`
	} `json:"lights"`
}

// AccessoryInfo represents the device information
type AccessoryInfo struct {
	ProductName         string   `json:"productName"`
	HardwareBoardType   int      `json:"hardwareBoardType"`
	FirmwareBuildNumber int      `json:"firmwareBuildNumber"`
	FirmwareVersion     string   `json:"firmwareVersion"`
	SerialNumber        string   `json:"serialNumber"`
	DisplayName         string   `json:"displayName"`
	Features            []string `json:"features"`
}

// KeyLightClient handles HTTP communication with a Key Light device
type KeyLightClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewKeyLightClient creates a new client for a Key Light device
func NewKeyLightClient(ip string, port int, logger *slog.Logger, httpClient ...*http.Client) *KeyLightClient {
	if logger == nil {
		logger = slog.Default()
	}
	var hc *http.Client
	if len(httpClient) > 0 && httpClient[0] != nil {
		hc = httpClient[0]
	} else {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &KeyLightClient{
		baseURL:    fmt.Sprintf("http://%s:%d/elgato", ip, port),
		httpClient: hc,
		logger:     logger,
	}
}

// GetAccessoryInfo retrieves basic device information
func (c *KeyLightClient) GetAccessoryInfo(ctx context.Context) (*AccessoryInfo, error) {
	url := c.baseURL + "/accessory-info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("light: /accessory-info request failed", "url", url, "error", err)
		return nil, fmt.Errorf("failed to get accessory info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.logger.Error("light: /accessory-info request failed", "url", url, "error", err)
		return nil, err
	}

	var info AccessoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		c.logger.Error("light: /accessory-info decode failed", "url", url, "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("light: /accessory-info response", "url", url, "info", info)
	return &info, nil
}

// GetLightState retrieves the current state of the light
func (c *KeyLightClient) GetLightState(ctx context.Context) (*LightState, error) {
	url := c.baseURL + "/lights"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("light: /lights request failed", "url", url, "error", err)
		return nil, fmt.Errorf("failed to get light state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.logger.Error("light: /lights request failed", "url", url, "error", err)
		return nil, err
	}

	var state LightState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		c.logger.Error("light: /lights decode failed", "url", url, "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("light: /lights response", "url", url, "state", state)
	return &state, nil
}

// SetLightState updates the state of the light
func (c *KeyLightClient) SetLightState(ctx context.Context, on bool, brightness, temperature int) error {
	// Validate brightness range (3-100)
	if brightness < 3 {
		brightness = 3
	} else if brightness > 100 {
		brightness = 100
	}

	payload := LightState{
		NumberOfLights: 1,
		Lights: []struct {
			On          int `json:"on"`
			Brightness  int `json:"brightness"`
			Temperature int `json:"temperature"`
		}{
			{
				On:          boolToInt(on),
				Brightness:  brightness,
				Temperature: temperature,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/lights"
	c.logger.Debug("setting light state",
		"url", url,
		"on", on,
		"brightness", brightness,
		"mireds", temperature,
		"payload", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set light state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	c.logger.Debug("light state updated successfully")
	return nil
}
