package client

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// newTestServer creates a test HTTP server with the given handler map.
func newTestServer(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *HTTPClient) {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, handler)
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewHTTP(testLogger(), server.URL, "test-api-key")
	return server, client
}

func jsonHandler(statusCode int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}
}

// === GetLights ===

func TestHTTPClient_GetLights(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights": jsonHandler(200, map[string]any{
			"light-1": map[string]any{"id": "light-1", "name": "Light 1", "brightness": 50},
		}),
	})

	lights, err := client.GetLights()
	require.NoError(t, err)
	assert.Contains(t, lights, "light-1")
}

func TestHTTPClient_GetLights_Error(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights": jsonHandler(401, map[string]any{"error": "unauthorized"}),
	})

	_, err := client.GetLights()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// === GetLight ===

func TestHTTPClient_GetLight(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights/light-1": jsonHandler(200, map[string]any{
			"id": "light-1", "name": "Light 1", "brightness": 50,
		}),
	})

	light, err := client.GetLight("light-1")
	require.NoError(t, err)
	assert.Equal(t, "light-1", light["id"])
}

func TestHTTPClient_GetLight_NotFound(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights/no-such": jsonHandler(404, map[string]any{"error": "not found"}),
	})

	_, err := client.GetLight("no-such")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// === SetLightState ===

func TestHTTPClient_SetLightState(t *testing.T) {
	var receivedBody map[string]any

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/lights/light-1/state": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		},
	})

	err := client.SetLightState("light-1", "brightness", 80)
	require.NoError(t, err)
	assert.Equal(t, float64(80), receivedBody["brightness"])
}

// === GetGroups ===

func TestHTTPClient_GetGroups(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/groups": jsonHandler(200, []map[string]any{
			{"id": "g1", "name": "Office", "lights": []string{"l1"}},
		}),
	})

	groups, err := client.GetGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "Office", groups[0]["name"])
}

func TestHTTPClient_GetGroups_Empty(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/groups": jsonHandler(200, nil),
	})

	groups, err := client.GetGroups()
	require.NoError(t, err)
	assert.Empty(t, groups)
}

// === CreateGroup ===

func TestHTTPClient_CreateGroup(t *testing.T) {
	var receivedBody map[string]any

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/groups": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": "g1", "name": "Office"})
		},
	})

	err := client.CreateGroup("Office")
	require.NoError(t, err)
	assert.Equal(t, "Office", receivedBody["name"])
}

// === GetGroup ===

func TestHTTPClient_GetGroup(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/groups/g1": jsonHandler(200, map[string]any{
			"id": "g1", "name": "Office", "lights": []string{"l1"},
		}),
	})

	group, err := client.GetGroup("g1")
	require.NoError(t, err)
	assert.Equal(t, "Office", group["name"])
}

// === DeleteGroup ===

func TestHTTPClient_DeleteGroup(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/groups/g1": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})

	err := client.DeleteGroup("g1")
	require.NoError(t, err)
}

func TestHTTPClient_DeleteGroup_NotFound(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/groups/no-such": jsonHandler(404, map[string]any{"error": "not found"}),
	})

	err := client.DeleteGroup("no-such")
	assert.Error(t, err)
}

// === SetGroupState ===

func TestHTTPClient_SetGroupState(t *testing.T) {
	var receivedBody map[string]any

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /api/v1/groups/g1/state": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		},
	})

	err := client.SetGroupState("g1", "on", true)
	require.NoError(t, err)
	assert.Equal(t, true, receivedBody["on"])
}

// === SetGroupLights ===

func TestHTTPClient_SetGroupLights(t *testing.T) {
	var receivedBody map[string]any

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /api/v1/groups/g1/lights": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		},
	})

	err := client.SetGroupLights("g1", []string{"l1", "l2"})
	require.NoError(t, err)
	lightIDs := receivedBody["light_ids"].([]any)
	assert.Len(t, lightIDs, 2)
}

// === API Key operations ===

func TestHTTPClient_AddAPIKey(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/apikeys": jsonHandler(201, map[string]any{
			"name": "test-key", "key": "abc123",
		}),
	})

	resp, err := client.AddAPIKey("test-key", 3600)
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp["key"])
}

func TestHTTPClient_AddAPIKey_NoExpiration(t *testing.T) {
	var receivedBody map[string]any

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/apikeys": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"name": "test-key", "key": "abc"})
		},
	})

	_, err := client.AddAPIKey("test-key", 0)
	require.NoError(t, err)
	// expires_in should not be in the body when 0
	_, hasExpires := receivedBody["expires_in"]
	assert.False(t, hasExpires)
}

func TestHTTPClient_ListAPIKeys(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/apikeys": jsonHandler(200, []map[string]any{
			{"name": "key-a"}, {"name": "key-b"},
		}),
	})

	keys, err := client.ListAPIKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

func TestHTTPClient_DeleteAPIKey(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/apikeys/abc123": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(204)
		},
	})

	err := client.DeleteAPIKey("abc123")
	require.NoError(t, err)
}

func TestHTTPClient_SetAPIKeyDisabledStatus(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /api/v1/apikeys/test-key/disabled": jsonHandler(200, map[string]any{
			"name": "test-key", "disabled": true,
		}),
	})

	resp, err := client.SetAPIKeyDisabledStatus("test-key", true)
	require.NoError(t, err)
	assert.Equal(t, true, resp["disabled"])
}

// === API key header test ===

func TestHTTPClient_SendsAPIKeyHeader(t *testing.T) {
	var receivedKey string

	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights": func(w http.ResponseWriter, r *http.Request) {
			receivedKey = r.Header.Get("X-API-Key")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{})
		},
	})

	_, _ = client.GetLights()
	assert.Equal(t, "test-api-key", receivedKey)
}

// === Error handling ===

func TestHTTPClient_ServerDown(t *testing.T) {
	// Create client pointing to closed server
	client := NewHTTP(testLogger(), "http://127.0.0.1:1", "key")
	_, err := client.GetLights()
	assert.Error(t, err)
}

func TestHTTPClient_InvalidJSON(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/lights": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("not json"))
		},
	})

	_, err := client.GetLights()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

// === NewHTTP constructor ===

func TestNewHTTP_TrimsTrailingSlash(t *testing.T) {
	c := NewHTTP(testLogger(), "http://example.com/", "key")
	assert.Equal(t, "http://example.com", c.baseURL)
}

func TestNewHTTP_NoTrailingSlash(t *testing.T) {
	c := NewHTTP(testLogger(), "http://example.com", "key")
	assert.Equal(t, "http://example.com", c.baseURL)
}
