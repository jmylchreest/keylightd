package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

type setupTestManager struct {
	keylight.LightManager
	status keylight.SetupStatus
}

func (m *setupTestManager) StartAirMK2Setup(_ context.Context, id string, _ keylight.AirMK2Pairer) (keylight.SetupStatus, error) {
	m.status = keylight.SetupStatus{DeviceID: id, Stage: "configuring"}
	return m.status, nil
}

func (m *setupTestManager) DeviceSetupStatus(id string) (keylight.SetupStatus, bool) {
	return m.status, m.status.DeviceID == id
}

func (m *setupTestManager) CancelDeviceSetup(id string) bool {
	if m.status.DeviceID != id {
		return false
	}
	m.status.Stage = "canceling"
	return true
}

func (m *setupTestManager) ForgetAirMK2(id string) error {
	if m.status.DeviceID != id {
		return keylight.ErrLightNotFound
	}
	m.status = keylight.SetupStatus{}
	return nil
}

type setupTestPairer struct{}

func (setupTestPairer) Provision(context.Context, keylight.AirMK2Provisioning) (string, int, error) {
	return "", 0, errors.New("pairing is handled by the test manager")
}

func TestSetupHandlers(t *testing.T) {
	ctx := t.Context()
	h := &LightHandler{Lights: &setupTestManager{}, SetupContext: ctx}
	input := &GetLightInput{ID: "desk"}
	_, err := h.StartSetup(ctx, &StartSetupInput{ID: input.ID})
	var statusErr huma.StatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, 400, statusErr.GetStatus())

	_, err = h.StartSetup(ctx, &StartSetupInput{ID: input.ID, Body: &keylight.AirMK2SetupOptions{
		Peripheral: "00:11:22:33:44:55", SSID: "test",
	}})
	require.ErrorContains(t, err, "Use nearby setup")

	h.PairerForLight = func(id string) (keylight.AirMK2Pairer, error) {
		require.Equal(t, "desk", id)
		return setupTestPairer{}, nil
	}
	out, err := h.StartSetup(ctx, &StartSetupInput{ID: input.ID})
	require.NoError(t, err)
	require.Equal(t, "configuring", out.Body.Stage)
	out, err = h.SetupStatus(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "desk", out.Body.DeviceID)
	out, err = h.CancelSetup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "canceling", out.Body.Stage)
	_, err = h.ForgetSetup(ctx, input)
	require.NoError(t, err)
	_, err = h.SetupStatus(ctx, input)
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, 404, statusErr.GetStatus())
}
