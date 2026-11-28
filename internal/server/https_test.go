package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/jmylchreest/keylightd/internal/pairing"
	"github.com/jmylchreest/keylightd/pkg/client"
)

func TestHTTPSPairingLifecycle(t *testing.T) {
	srv, existingKey, original := setupHTTPIntegrationTest(t)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { srv.Stop() })
	local := client.New(srv.logger, srv.socketPath)
	op := func(action string) map[string]any {
		t.Helper()
		resp, err := local.HTTPS(action, "")
		require.NoError(t, err)
		return resp
	}
	issue := func() string { return op("pair.create")["invitation"].(string) }
	plain := client.NewHTTP(srv.logger, original, existingKey)
	_, err := plain.GetLights()
	require.NoError(t, err)
	pin, ok := op("https.enable")["fingerprint"].(string)
	require.True(t, ok)
	require.NotEmpty(t, pin)
	token := issue()
	changed, err := pairing.Decode(token)
	require.NoError(t, err)
	changed.Fingerprint = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	wrong, _ := changed.Encode()
	_, err = client.Redeem(t.Context(), srv.logger, wrong)
	require.ErrorContains(t, err, "fingerprint")
	existing, err := client.NewPaired(srv.logger, client.PairedProfile{URL: changed.URL, Fingerprint: pin, APIKey: existingKey})
	require.NoError(t, err)
	_, err = existing.GetLights()
	require.NoError(t, err)
	_, err = plain.GetLights()
	require.Error(t, err)
	profile, err := client.Redeem(t.Context(), srv.logger, token)
	require.NoError(t, err)
	_, err = client.Redeem(t.Context(), srv.logger, token)
	require.ErrorContains(t, err, "already used")
	remote, err := client.NewPaired(srv.logger, profile)
	require.NoError(t, err)
	lights, err := remote.GetLights()
	require.NoError(t, err)
	require.Contains(t, lights, "test-light-1")
	watch, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch, err := remote.Watch(watch)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		srv.eventBus.Publish(events.Event{Type: "changed"})
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond)
	unused := issue()
	_, err = local.HTTPS("https.delete-identity", "")
	require.Error(t, err)
	op("https.disable")
	select {
	case _, open := <-ch:
		if open {
			select {
			case _, open = <-ch:
				require.False(t, open)
			case <-time.After(time.Second):
				t.Fatal("watch remained open")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("watch remained open")
	}
	require.Equal(t, pin, op("https.enable")["fingerprint"])
	_, err = client.Redeem(t.Context(), srv.logger, unused)
	require.Error(t, err)
	srv.Stop()
	cfg, err := config.Load("config", srv.cfg.Viper().ConfigFileUsed())
	require.NoError(t, err)
	srv = New(srv.logger, cfg, srv.lights, srv.versionInfo)
	require.NoError(t, srv.Start())
	_, err = remote.GetLights()
	require.NoError(t, err)
	require.Equal(t, true, op("https.status")["enabled"])
	op("https.disable")
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, original+"/healthz", nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	_, err = plain.GetLights()
	require.NoError(t, err)
	op("https.delete-identity")
	require.NotEqual(t, pin, op("https.enable")["fingerprint"])
	_, err = remote.GetLights()
	require.ErrorContains(t, err, "certificate")
}
