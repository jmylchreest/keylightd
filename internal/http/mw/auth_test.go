package mw

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jmylchreest/keylightd/internal/apikey"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSetup creates an apikey.Manager with a valid API key for testing.
func testSetup(t *testing.T) (*apikey.Manager, *config.APIKey) {
	t.Helper()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfg, err := config.Load("config.yaml", cfgPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mgr := apikey.NewManager(cfg, logger)

	key, err := mgr.CreateAPIKey("test-key", 0)
	require.NoError(t, err)

	return mgr, key
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// --- RawAPIKeyAuth tests ---

func TestRawAPIKeyAuth_ValidBearerToken(t *testing.T) {
	mgr, key := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestRawAPIKeyAuth_ValidXAPIKeyHeader(t *testing.T) {
	mgr, key := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", key.Key)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRawAPIKeyAuth_MissingKey(t *testing.T) {
	mgr, _ := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when key is missing")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "API key required")
}

func TestRawAPIKeyAuth_InvalidKey(t *testing.T) {
	mgr, _ := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with invalid key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key-12345")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Unauthorized")
}

func TestRawAPIKeyAuth_DisabledKey(t *testing.T) {
	mgr, key := testSetup(t)
	logger := testLogger()

	// Disable the key
	_, err := mgr.SetAPIKeyDisabledStatus(key.Name, true)
	require.NoError(t, err)

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with disabled key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "disabled")
}

func TestRawAPIKeyAuth_ExpiredKey(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfg, err := config.Load("config.yaml", cfgPath)
	require.NoError(t, err)

	logger := testLogger()
	mgr := apikey.NewManager(cfg, logger)

	// Create a key that expires in 50ms
	key, err := mgr.CreateAPIKey("expiring-key", 50*time.Millisecond)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(75 * time.Millisecond)

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with expired key")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "expired")
}

func TestRawAPIKeyAuth_BearerPrefixPrecedence(t *testing.T) {
	mgr, key := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// If Authorization header has "Bearer " prefix, it should be used even if X-API-Key is set
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRawAPIKeyAuth_AuthorizationWithoutBearerFallsToXAPIKey(t *testing.T) {
	mgr, key := testSetup(t)
	logger := testLogger()

	handler := RawAPIKeyAuth(logger, mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Authorization header without "Bearer " prefix should fall through to X-API-Key
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "not-a-bearer-token")
	req.Header.Set("X-API-Key", key.Key)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- operationRequiresAuth tests ---

func TestOperationRequiresAuth_WithSecurity(t *testing.T) {
	op := &huma.Operation{
		Security: []map[string][]string{
			{SecurityScheme: {}},
		},
	}
	assert.True(t, operationRequiresAuth(op))
}

func TestOperationRequiresAuth_WithoutSecurity(t *testing.T) {
	op := &huma.Operation{}
	assert.False(t, operationRequiresAuth(op))
}

func TestOperationRequiresAuth_WithOtherSecurityScheme(t *testing.T) {
	op := &huma.Operation{
		Security: []map[string][]string{
			{"otherScheme": {}},
		},
	}
	assert.False(t, operationRequiresAuth(op))
}

func TestOperationRequiresAuth_EmptySecuritySlice(t *testing.T) {
	op := &huma.Operation{
		Security: []map[string][]string{},
	}
	assert.False(t, operationRequiresAuth(op))
}

// --- keyPrefix tests ---

func TestKeyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"long key", "abcdefghij", "abcd"},
		{"exactly 4", "abcd", "abcd"},
		{"short key", "ab", "ab"},
		{"empty key", "", ""},
		{"single char", "x", "x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, keyPrefix(tc.key))
		})
	}
}
