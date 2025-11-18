package commands

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/stretchr/testify/require"
)

type mockAPIKeyClient struct {
	client.ClientInterface
	failAdd    bool
	failDelete bool
	apiKeys    map[string]map[string]any
}

func (m *mockAPIKeyClient) AddAPIKey(name string, expiresInSeconds float64) (map[string]any, error) {
	if m.failAdd || m.apiKeys[name] != nil {
		return nil, errors.New("duplicate or failed to add API key")
	}
	key := map[string]any{"key": name + "-key", "name": name}
	m.apiKeys[name] = key
	return key, nil
}

func (m *mockAPIKeyClient) DeleteAPIKey(key string) error {
	if m.failDelete || m.apiKeys[key] == nil {
		return errors.New("not found")
	}
	delete(m.apiKeys, key)
	return nil
}

func (m *mockAPIKeyClient) ListAPIKeys() ([]map[string]any, error) {
	var out []map[string]any
	for _, v := range m.apiKeys {
		out = append(out, v)
	}
	return out, nil
}

// parseKeyValueOutput parses CLI output lines of the form 'Key   Value' into a map.
func parseKeyValueOutput(out string) map[string]string {
	result := make(map[string]string)
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := fields[0]
			value := strings.Join(fields[1:], " ")
			result[key] = value
		}
	}
	return result
}

func TestAPIKeyAddCommand_Duplicate(t *testing.T) {
	mock := &mockAPIKeyClient{apiKeys: map[string]map[string]any{}}
	mock.apiKeys["dupe"] = map[string]any{"key": "dupe-key", "name": "dupe"}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newAPIKeyAddCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"dupe", "0"})
	out := captureStdout(func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})
	kv := parseKeyValueOutput(out)
	require.Equal(t, "dupe", kv["Name"])
	require.Equal(t, "duplicate or failed to add API key", kv["Error"])
}

func TestAPIKeyDeleteCommand_NotFound(t *testing.T) {
	mock := &mockAPIKeyClient{apiKeys: map[string]map[string]any{}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newAPIKeyDeleteCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"notfound", "--yes"})
	out := captureStdout(func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})
	kv := parseKeyValueOutput(out)
	require.Equal(t, "notfound", kv["Key"])
	require.Equal(t, "not found", kv["Error"])
}
