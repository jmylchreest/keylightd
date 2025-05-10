package keylight

import (
	"bytes"
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
func NewKeyLightClient(ip string, port int, logger *slog.Logger) *KeyLightClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &KeyLightClient{
		baseURL:    fmt.Sprintf("http://%s:%d/elgato", ip, port),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}
}

// GetAccessoryInfo retrieves basic device information
func (c *KeyLightClient) GetAccessoryInfo() (*AccessoryInfo, error) {
	url := c.baseURL + "/accessory-info"
	c.logger.Debug("getting accessory info", "url", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessory info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var info AccessoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("got accessory info", "info", info)
	return &info, nil
}

// GetLightState retrieves the current state of the light
func (c *KeyLightClient) GetLightState() (*LightState, error) {
	url := c.baseURL + "/lights"
	c.logger.Debug("getting light state", "url", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get light state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var state LightState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("got light state", "state", state)
	return &state, nil
}

// SetLightState updates the state of the light
func (c *KeyLightClient) SetLightState(on bool, brightness, temperature int) error {
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

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
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

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// convertTemperatureToDevice converts from Kelvin to mireds (device units)
// The Elgato Key Light uses mireds (micro reciprocal degrees) for temperature control
// Mireds = 1,000,000 / Kelvin
// Device range: 344 mireds (2900K) to 143 mireds (7000K)
func convertTemperatureToDevice(kelvin int) int {
	// Clamp input to valid range
	if kelvin < 2900 {
		kelvin = 2900
	} else if kelvin > 7000 {
		kelvin = 7000
	}

	// Convert Kelvin to mireds
	// Mireds = 1,000,000 / Kelvin
	mireds := 1000000 / kelvin

	// Clamp mireds to device range
	if mireds > 344 {
		mireds = 344 // 2900K
	} else if mireds < 143 {
		mireds = 143 // 7000K
	}

	return mireds
}

// convertDeviceToTemperature converts from mireds (device units) to Kelvin
// The Elgato Key Light uses mireds (micro reciprocal degrees) for temperature control
// Kelvin = 1,000,000 / mireds
func convertDeviceToTemperature(mireds int) int {
	// Clamp input to valid range
	if mireds > 344 {
		mireds = 344 // 2900K
	} else if mireds < 143 {
		mireds = 143 // 7000K
	}

	// Convert mireds to Kelvin
	// Kelvin = 1,000,000 / mireds
	kelvin := 1000000 / mireds

	return kelvin
}
