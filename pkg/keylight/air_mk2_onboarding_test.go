package keylight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

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
		server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrade.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			var request airRPCRequest
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			var result any
			switch request.Method {
			case "get.elgato.accessory-info":
				result = AccessoryInfo{HardwareBoardType: 214, SerialNumber: "MOCK-AIR"}
			case "get.elgato.lights":
				result = map[string]any{"numberOfLights": 1, "lights": []map[string]int{{"on": 0, "brightness": 25, "temperature": 230}}}
			default:
				t.Errorf("unexpected RPC method %q", request.Method)
				return
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
				t.Errorf("write response: %v", err)
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
	identity, err := OnboardAirMK2(context.Background(), pairer)
	require.NoError(t, err)
	require.Equal(t, "MOCK-AIR", identity.Info.SerialNumber)
	require.NotEmpty(t, identity.ClientKey)
	require.Equal(t, server.Listener.Addr().String(), identity.Address)
}
