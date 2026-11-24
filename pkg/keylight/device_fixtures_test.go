package keylight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestElgatoDeviceFixtures(t *testing.T) {
	for _, fixture := range []struct {
		file  string
		board int
		model string
	}{
		{"key-light", 53, "Elgato Key Light"},
		{"key-light-air", 200, "Elgato Key Light Air"},
		{"key-light-mk2", 205, "Elgato Key Light MK.2"},
		{"ring-light", 201, "Elgato Ring Light"},
		{"key-light-mini", 202, "Elgato Key Light Mini"},
		{"key-light-neo", 210, ""},
	} {
		t.Run(fixture.file, func(t *testing.T) {
			info, err := os.ReadFile("testdata/elgato/" + fixture.file + ".json")
			require.NoError(t, err)
			type state struct {
				Count  int              `json:"numberOfLights"`
				Lights []map[string]int `json:"lights"`
			}
			current := state{1, []map[string]int{{"on": 0, "brightness": 30, "temperature": 250}}}
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/elgato/accessory-info":
					_, _ = w.Write(info)
				case fixture.board == 202 && r.Method == http.MethodGet && r.URL.Path == "/elgato/battery-info":
					_, _ = w.Write([]byte(`{"powerSource":1,"level":95.28,"status":2}`))
				case r.Method == http.MethodGet && r.URL.Path == "/elgato/lights":
					_ = json.NewEncoder(w).Encode(current)
				case r.Method == http.MethodPut && r.URL.Path == "/elgato/lights":
					var next state
					if json.NewDecoder(r.Body).Decode(&next) != nil || next.Count != 1 || len(next.Lights) != 1 {
						http.Error(w, "invalid state", http.StatusBadRequest)
						return
					}
					current = next
					w.WriteHeader(http.StatusOK)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			ctx := context.Background()
			entry := makeServiceEntry(t, server, fixture.file)
			light, client, accepted := openElgatoLight(ctx, entry, discardLogger())
			require.Equal(t, fixture.model != "", accepted)
			if accepted {
				require.Equal(t, fixture.model, light.Model)
				require.Equal(t, fixture.board == 202, light.Experimental)
			} else {
				// Candidate fixtures exercise the wire client independently of registry admission.
				client = NewKeyLightClient(entry.AddrV4.String(), entry.Port, discardLogger())
			}
			if fixture.board == 202 {
				manager := NewManager(discardLogger())
				manager.addDevice(ctx, light, client)
				battery := manager.GetLights()[light.ID].BatteryInfo
				require.NotNil(t, battery)
				require.Equal(t, 95.28, battery.Level)
			}
			identity, err := client.GetAccessoryInfo(ctx)
			require.NoError(t, err)
			require.Equal(t, fixture.board, identity.HardwareBoardType)
			before, err := client.GetLightState(ctx)
			require.NoError(t, err)
			require.Len(t, before.Lights, 1)
			require.Zero(t, before.Lights[0].On)
			require.NoError(t, client.SetLightState(ctx, true, 62, 210))
			after, err := client.GetLightState(ctx)
			require.NoError(t, err)
			require.Len(t, after.Lights, 1)
			require.Equal(t, 1, after.Lights[0].On)
			require.Equal(t, 62, after.Lights[0].Brightness)
			require.Equal(t, 210, after.Lights[0].Temperature)
		})
	}
}

func TestDeviceStateFixtureFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		respond   func(http.ResponseWriter, *http.Request)
		wantError string
	}{
		{"503", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}, "503"},
		{"malformed", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{invalid"))
		}, "invalid character"},
		{"stalled", func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.respond))
			defer server.Close()
			entry := makeServiceEntry(t, server, tc.name)
			client := NewKeyLightClient(entry.AddrV4.String(), entry.Port, discardLogger())
			ctx := context.Background()
			if tc.wantError == "" {
				deadline, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
				ctx = deadline
			}
			state, err := client.GetLightState(ctx)
			require.Nil(t, state)
			if tc.wantError == "" {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
		})
	}
}
