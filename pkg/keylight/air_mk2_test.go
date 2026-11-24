package keylight

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestAirMK2ClientRoundTrip(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var state *LightState
		for {
			var req airRPCRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			var result any
			switch req.Method {
			case "get.elgato.accessory-info":
				result = AccessoryInfo{ProductName: "Elgato Key Light Air MK.2", HardwareBoardType: 214, SerialNumber: "TEST-AIR"}
			case "put.elgato.lights":
				state = req.Params
				continue
			case "get.elgato.lights":
				result = state
			default:
				t.Errorf("unexpected method %s", req.Method)
				return
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "notification.elgato.lights"}); err != nil {
				return
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
				return
			}
		}
	}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()
	entry := makeServiceEntry(t, server, "air-mk2")
	transport, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok)
	config := transport.TLSClientConfig.Clone()
	config.Certificates = server.TLS.Certificates
	client, err := NewAirMK2Client(entry.AddrV4.String(), entry.Port, config)
	require.NoError(t, err)
	info, err := client.GetAccessoryInfo(context.Background())
	require.NoError(t, err)
	require.Equal(t, 214, info.HardwareBoardType)
	require.NoError(t, client.SetLightState(context.Background(), true, 55, 220))

	ctx := context.Background()
	var unconfigured *AirMK2Discovery
	pending, registered, ok := unconfigured.openDiscovered(ctx, entry)
	require.True(t, ok)
	require.Nil(t, registered)
	require.Equal(t, []string{"credentials_missing"}, pending.NeedsConfiguration)
	manager := NewManager(discardLogger())
	manager.addPendingDevice(pending)
	snapshot, err := manager.GetLight(ctx, pending.ID)
	require.NoError(t, err)
	require.Equal(t, pending.NeedsConfiguration, snapshot.NeedsConfiguration)

	provider := &AirMK2Discovery{
		devices: []airMK2Endpoint{{host: entry.AddrV4.String(), port: entry.Port, client: client}},
		logger:  discardLogger(),
	}
	ready, registered, ok := provider.openDiscovered(ctx, entry)
	require.True(t, ok)
	require.Same(t, client, registered)
	require.Equal(t, pending.ID, ready.ID)
	require.Empty(t, ready.NeedsConfiguration)
	require.Equal(t, "TEST-AIR", ready.SerialNumber)
}
