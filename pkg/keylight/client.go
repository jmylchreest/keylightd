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

// doGet performs a GET request to the given path and JSON-decodes the response into result.
func (c *KeyLightClient) doGet(ctx context.Context, path string, result any) error {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL is from discovered light address
	if err != nil {
		c.logger.Error("light: request failed", "url", url, "error", err)
		return fmt.Errorf("failed to get %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.logger.Error("light: request failed", "url", url, "error", err)
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		c.logger.Error("light: decode failed", "url", url, "error", err)
		return fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("light: response", "url", url, "result", result)
	return nil
}

// GetAccessoryInfo retrieves basic device information
func (c *KeyLightClient) GetAccessoryInfo(ctx context.Context) (*AccessoryInfo, error) {
	var info AccessoryInfo
	if err := c.doGet(ctx, "/accessory-info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetLightState retrieves the current state of the light
func (c *KeyLightClient) GetLightState(ctx context.Context) (*LightState, error) {
	var state LightState
	if err := c.doGet(ctx, "/lights", &state); err != nil {
		return nil, err
	}
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

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL is from discovered light address
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
