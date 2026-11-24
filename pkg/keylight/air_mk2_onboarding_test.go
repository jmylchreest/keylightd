package keylight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type mockAirPairer func(context.Context, AirMK2Provisioning) (string, int, error)

func (p mockAirPairer) Provision(ctx context.Context, material AirMK2Provisioning) (string, int, error) {
	return p(ctx, material)
}

func TestOnboardAirMK2(t *testing.T) {
	var server *httptest.Server
	pairer := mockAirPairer(func(_ context.Context, material AirMK2Provisioning) (string, int, error) {
		pair, err := tls.X509KeyPair(material.ServerCertificate, material.ServerKey)
		require.NoError(t, err)
		roots := x509.NewCertPool()
		require.True(t, roots.AppendCertsFromPEM(material.CA))
		upgrade := websocket.Upgrader{}
		var mu sync.Mutex
		var state any = map[string]any{"numberOfLights": 1, "lights": []map[string]int{{"on": 0, "brightness": 25, "temperature": 230}}}
		server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrade.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				var request airRPCRequest
				if err := conn.ReadJSON(&request); err != nil {
					return
				}
				var result any
				switch request.Method {
				case "get.elgato.accessory-info":
					result = AccessoryInfo{HardwareBoardType: 214, SerialNumber: "MOCK-AIR"}
				case "put.elgato.lights":
					mu.Lock()
					state = request.Params
					mu.Unlock()
					continue
				case "get.elgato.lights":
					mu.Lock()
					result = state
					mu.Unlock()
				default:
					t.Errorf("unexpected RPC method %q", request.Method)
					return
				}
				if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
					t.Errorf("write response: %v", err)
					return
				}
			}
		}))
		server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS12}
		server.StartTLS()
		t.Cleanup(server.Close)
		host, rawPort, err := net.SplitHostPort(server.Listener.Addr().String())
		require.NoError(t, err)
		port, err := strconv.Atoi(rawPort)
		require.NoError(t, err)
		return host, port, nil
	})
	ctx := context.Background()
	configPath := t.TempDir() + "/config.yaml"
	provider, err := NewAirMK2Discovery(nil, discardLogger())
	require.NoError(t, err)
	provider.SetAirMK2CredentialDirectory(configPath)
	manager := NewManager(discardLogger())
	manager.SetAirMK2Discovery(provider)
	entry := &ServiceEntry{Name: "fake._elg._tcp.local.", AddrV4: net.IPv4(127, 0, 0, 1), Port: 9123}
	pending, client, ok := provider.openDiscovered(ctx, entry)
	require.True(t, ok)
	require.Nil(t, client)
	require.Equal(t, []string{"credentials_missing"}, pending.NeedsConfiguration)
	manager.addPendingDevice(pending)
	status, err := manager.StartAirMK2Setup(ctx, pending.ID, pairer)
	require.NoError(t, err)
	require.Equal(t, "configuring", status.Stage)
	require.Eventually(t, func() bool {
		status, ok := manager.DeviceSetupStatus(pending.ID)
		return ok && status.Stage == "ready"
	}, time.Second, time.Millisecond)
	require.ErrorContains(t, manager.ConfigureAirMK2(ctx, pending.ID, pairer), "already has stored credentials")

	restarted, err := NewAirMK2Discovery(nil, discardLogger())
	require.NoError(t, err)
	restarted.SetAirMK2CredentialDirectory(configPath)
	entry = makeServiceEntry(t, server, entry.Name)
	light, client, ok := restarted.openDiscovered(ctx, entry)
	require.True(t, ok)
	require.Empty(t, light.NeedsConfiguration)
	require.Equal(t, "MOCK-AIR", light.SerialNumber)
	require.Equal(t, pending.ID, light.ID)
	afterRestart := NewManager(discardLogger())
	afterRestart.addDevice(ctx, light, client)
	ready, err := afterRestart.GetLight(ctx, light.ID)
	require.NoError(t, err)
	require.Equal(t, 25, ready.Brightness)
	require.True(t, ready.Experimental)
	require.NoError(t, afterRestart.SetLightBrightness(ctx, light.ID, 56))
	ready, err = afterRestart.GetLight(ctx, light.ID)
	require.NoError(t, err)
	require.Equal(t, 56, ready.Brightness)
}
