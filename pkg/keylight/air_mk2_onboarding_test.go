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
	"github.com/grandcat/zeroconf"
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
	configured, err := manager.GetLight(ctx, pending.ID)
	require.NoError(t, err)
	require.Empty(t, configured.NeedsConfiguration)
	require.True(t, configured.ManagedCredentials)
	require.ErrorContains(t, manager.ConfigureAirMK2(ctx, pending.ID, pairer), "does not require")

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
	afterRestart.SetAirMK2Discovery(restarted)
	afterRestart.addDevice(ctx, light, client)
	ready, err := afterRestart.GetLight(ctx, light.ID)
	require.NoError(t, err)
	require.Equal(t, 25, ready.Brightness)
	require.True(t, ready.Experimental)
	require.True(t, ready.ManagedCredentials)
	require.NoError(t, afterRestart.SetLightBrightness(ctx, light.ID, 56))
	ready, err = afterRestart.GetLight(ctx, light.ID)
	require.NoError(t, err)
	require.Equal(t, 56, ready.Brightness)
	require.NoError(t, afterRestart.ForgetAirMK2(light.ID))
	afterRestart.addDevice(ctx, light, client)
	require.ErrorIs(t, afterRestart.ForgetAirMK2(light.ID), ErrLightNotFound)
	forgotten, err := afterRestart.GetLight(ctx, light.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"credentials_missing"}, forgotten.NeedsConfiguration)
	require.False(t, forgotten.ManagedCredentials)
	require.Error(t, afterRestart.SetLightBrightness(ctx, light.ID, 60))
	_, client, ok = restarted.openDiscovered(ctx, entry)
	require.True(t, ok)
	require.Nil(t, client)

	// A factory-reset unit has no manager record before BLE setup.
	factory := NewManager(discardLogger())
	factory.SetAirMK2Discovery(restarted)
	blinks := make(chan struct{}, 1)
	confirmedPairer := mockAirPairer(func(ctx context.Context, material AirMK2Provisioning) (string, int, error) {
		if err := material.Confirm(ctx, func() error { blinks <- struct{}{}; return nil }); err != nil {
			return "", 0, err
		}
		network, err := material.WaitWiFi(ctx)
		if err != nil {
			return "", 0, err
		}
		if network.SSID != "fixture" {
			return "", 0, SetupWiFiJoinFailed
		}
		return pairer.Provision(ctx, material)
	})
	const instance = "Factory Air"
	const discoveredID = instance + "._elg._tcp.local."
	handoff := func(ctx context.Context, ip net.IP, verify func(context.Context, string, int) error) (string, int, error) {
		port := server.Listener.Addr().(*net.TCPAddr).Port
		entries := make(chan *zeroconf.ServiceEntry, 2)
		entries <- &zeroconf.ServiceEntry{ServiceRecord: zeroconf.ServiceRecord{Instance: "Wrong IP", Service: "_elg._tcp", Domain: "local."}, AddrIPv4: []net.IP{net.IPv4(192, 0, 2, 1)}, Port: port, Text: []string{airMK2TLSText}}
		entries <- &zeroconf.ServiceEntry{ServiceRecord: zeroconf.ServiceRecord{Instance: instance, Service: "_elg._tcp", Domain: "local."}, AddrIPv4: []net.IP{ip}, Port: port, Text: []string{airMK2TLSText}}
		return findProvisionedAirMK2(ctx, ip, entries, verify)
	}
	_, err = factory.startNearbyAirMK2Setup(t.Context(), confirmedPairer, handoff)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status, _ = factory.DeviceSetupStatus(NearbyAirMK2SetupID)
		return status.Stage == "awaiting_confirmation"
	}, time.Second, time.Millisecond)
	require.Empty(t, factory.GetLights())
	require.False(t, factory.ConfirmDeviceSetup(discoveredID, status.ConfirmationID))
	require.False(t, factory.ConfirmDeviceSetup(NearbyAirMK2SetupID, "stale"))
	require.False(t, factory.BlinkDeviceSetup(NearbyAirMK2SetupID, "stale"))
	require.True(t, factory.BlinkDeviceSetup(NearbyAirMK2SetupID, status.ConfirmationID))
	select {
	case <-blinks:
	case <-time.After(time.Second):
		t.Fatal("selected light did not blink again")
	}
	require.True(t, factory.ConfirmDeviceSetup(NearbyAirMK2SetupID, status.ConfirmationID))
	require.Eventually(t, func() bool {
		status, _ = factory.DeviceSetupStatus(NearbyAirMK2SetupID)
		return status.Stage == "awaiting_wifi"
	}, time.Second, time.Millisecond)
	network := AirMK2WiFiOptions{SSID: "fixture"}
	require.False(t, factory.JoinDeviceSetupWiFi(NearbyAirMK2SetupID, "stale", network))
	require.True(t, factory.JoinDeviceSetupWiFi(NearbyAirMK2SetupID, status.WiFiID, network))
	require.Eventually(t, func() bool {
		status, _ = factory.DeviceSetupStatus(NearbyAirMK2SetupID)
		return status.Stage == "ready"
	}, 4*time.Second, time.Millisecond)
	require.Equal(t, discoveredID, status.ConfiguredID)
	require.Contains(t, factory.GetLights(), discoveredID)
	require.NotContains(t, factory.GetLights(), NearbyAirMK2SetupID)

}
