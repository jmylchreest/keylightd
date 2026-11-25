package client

import "github.com/jmylchreest/keylightd/pkg/keylight"

type socketLightSetupRequest struct {
	Action string         `json:"action"`
	Data   map[string]any `json:"data"`
}

func (c *Client) lightSetup(action, id, confirmationID string, options *keylight.AirMK2SetupOptions) (map[string]any, error) {
	request := socketLightSetupRequest{"light_setup", map[string]any{
		"operation": action, "lightId": id, "confirmationId": confirmationID, "options": options,
	}}
	var response map[string]any
	err := c.request(request, &response)
	return response, err
}

func (c *Client) StartAirMK2Setup(id string, options keylight.AirMK2SetupOptions) (map[string]any, error) {
	return c.lightSetup("start", id, "", &options)
}

func (c *Client) GetLightSetup(id string) (map[string]any, error) {
	return c.lightSetup("status", id, "", nil)
}

func (c *Client) CancelLightSetup(id string) (map[string]any, error) {
	return c.lightSetup("cancel", id, "", nil)
}

func (c *Client) BlinkLightSetup(id, confirmationID string) error {
	_, err := c.lightSetup("blink", id, confirmationID, nil)
	return err
}

func (c *Client) ConfirmLightSetup(id, confirmationID string) error {
	_, err := c.lightSetup("confirm", id, confirmationID, nil)
	return err
}

func (c *Client) ForgetLightCredentials(id string) error {
	_, err := c.lightSetup("forget", id, "", nil)
	return err
}

func (c *Client) JoinLightSetup(id, wifiID string, options keylight.AirMK2WiFiOptions) error {
	request := socketLightSetupRequest{"light_setup", map[string]any{
		"operation": "wifi", "lightId": id, "confirmationId": wifiID, "network": options,
	}}
	var response map[string]any
	return c.request(request, &response)
}
