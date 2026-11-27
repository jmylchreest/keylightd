package server

import (
	"encoding/json"
	"errors"

	"github.com/jmylchreest/keylightd/internal/http/handlers"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

func (s *Server) handleLightSetup(r socketRequest) socketActionResult {
	var request struct {
		Network        keylight.AirMK2WiFiOptions   `json:"network"`
		Operation      string                       `json:"operation"`
		LightID        string                       `json:"lightId"`
		ConfirmationID string                       `json:"confirmationId"`
		Options        *keylight.AirMK2SetupOptions `json:"options"`
	}
	data, err := json.Marshal(r.data)
	if err != nil || json.Unmarshal(data, &request) != nil || request.LightID == "" {
		s.sendError(r.conn, r.id, "invalid setup request")
		return socketContinue
	}
	h := handlers.LightHandler{Lights: s.lights, SetupContext: s.rootCtx}
	input := &handlers.GetLightInput{ID: request.LightID}
	var status *handlers.SetupOutput
	switch request.Operation {
	case "start":
		status, err = h.StartSetup(r.ctx, &handlers.StartSetupInput{ID: request.LightID, Body: request.Options})
	case "status":
		status, err = h.SetupStatus(r.ctx, input)
	case "cancel":
		status, err = h.CancelSetup(r.ctx, input)
	case "wifi":
		wifi := &handlers.JoinSetupInput{ID: request.LightID}
		wifi.Body.WiFiID = request.ConfirmationID
		wifi.Body.Network = request.Network
		_, err = h.JoinSetup(r.ctx, wifi)
	case "blink", "confirm":
		confirmation := &handlers.ConfirmSetupInput{ID: request.LightID}
		confirmation.Body.ConfirmationID = request.ConfirmationID
		if request.Operation == "blink" {
			_, err = h.BlinkSetup(r.ctx, confirmation)
		} else {
			_, err = h.ConfirmSetup(r.ctx, confirmation)
		}
	case "forget":
		_, err = h.ForgetSetup(r.ctx, input)
	default:
		err = errors.New("unknown setup operation")
	}
	if err != nil {
		s.sendError(r.conn, r.id, err.Error())
	} else if status != nil {
		s.sendResponse(r.conn, r.id, map[string]any{
			"deviceId": status.Body.DeviceID, "stage": status.Body.Stage,
			"confirmationId": status.Body.ConfirmationID, "configuredId": status.Body.ConfiguredID,
			"failureReason": status.Body.FailureReason, "wifiId": status.Body.WiFiID,
		})
	} else {
		s.sendResponse(r.conn, r.id, map[string]any{})
	}
	return socketContinue
}
