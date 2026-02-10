package commands

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/stretchr/testify/require"
)

// Use the same clientContextKey as in light.go
// var clientContextKey = &struct{}{} // already defined in light.go

type mockGroupClient struct {
	groups map[string]map[string]any
	fail   bool
}

var _ client.ClientInterface = (*mockGroupClient)(nil)

func (m *mockGroupClient) GetVersion() (map[string]any, error)        { return nil, nil }
func (m *mockGroupClient) GetLights() (map[string]any, error)         { return nil, nil }
func (m *mockGroupClient) GetLight(id string) (map[string]any, error) { return nil, nil }
func (m *mockGroupClient) SetLightState(id string, property string, value any) error {
	return nil
}
func (m *mockGroupClient) SetGroupState(name string, property string, value any) error {
	return nil
}
func (m *mockGroupClient) SetGroupLights(groupID string, lightIDs []string) error { return nil }
func (m *mockGroupClient) CreateGroup(name string) error {
	if m.fail {
		return errors.New("create group failed")
	}
	m.groups[name] = map[string]any{"name": name, "id": name, "lights": []string{"light1"}}
	return nil
}
func (m *mockGroupClient) GetGroup(name string) (map[string]any, error) {
	if m.fail {
		return nil, errors.New("get group failed")
	}
	g, ok := m.groups[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return g, nil
}
func (m *mockGroupClient) GetGroups() ([]map[string]any, error) {
	if m.fail {
		return nil, errors.New("get groups failed")
	}
	var out []map[string]any
	for _, g := range m.groups {
		out = append(out, g)
	}
	return out, nil
}
func (m *mockGroupClient) DeleteGroup(name string) error {
	if m.fail {
		return errors.New("delete group failed")
	}
	delete(m.groups, name)
	return nil
}

// API Key Management Mocks (satisfy client.ClientInterface)
func (m *mockGroupClient) AddAPIKey(name string, expiresInSeconds float64) (map[string]any, error) {
	if m.fail {
		return nil, errors.New("add api key failed")
	}
	// Simple mock: doesn't actually store/return a real key structure for group tests
	return map[string]any{"key": "mockapikey", "name": name}, nil
}

func (m *mockGroupClient) ListAPIKeys() ([]map[string]any, error) {
	if m.fail {
		return nil, errors.New("list api keys failed")
	}
	return []map[string]any{}, nil // Return empty list for group tests
}

func (m *mockGroupClient) DeleteAPIKey(key string) error {
	if m.fail {
		return errors.New("delete api key failed")
	}
	return nil
}

func (m *mockGroupClient) SetAPIKeyDisabledStatus(keyOrName string, disabled bool) (map[string]any, error) {
	if m.fail {
		return nil, errors.New("set api key disabled status failed")
	}
	return map[string]any{"key": keyOrName, "disabled": disabled}, nil
}

func TestGroupListCommand(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{
		"group1": {"id": "group1", "name": "Group 1", "lights": []any{"light1"}},
	}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupListCommand(logger)
	cmd.SetContext(ctx)
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestGroupAddCommand(t *testing.T) {
	mock := &mockGroupClient{groups: make(map[string]map[string]any)}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupAddCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"newgroup"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestGroupAddCommand_Duplicate(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{}}
	mock.groups["dupe"] = map[string]any{"id": "dupe", "name": "dupe", "lights": []any{}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupAddCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"dupe"})
	out := captureStdout(func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})
	kv := parseKeyValueOutput(out)
	require.Equal(t, "dupe", kv["Name"])
}

func TestGroupDeleteCommand(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{"group1": {"id": "group1", "name": "Group 1", "lights": []any{}}}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupDeleteCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"group1", "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestGroupDeleteCommand_GroupNotFound(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupDeleteCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"notfound", "--yes"})
	out := captureStdout(func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})
	kv := parseKeyValueOutput(out)
	require.Equal(t, "notfound", kv["Input"])
}

func TestGroupGetCommand(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{"group1": {"id": "group1", "name": "Group 1", "lights": []any{}}}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupGetCommand(logger)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"group1"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestGroupSetCommand(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{"group1": {"id": "group1", "name": "Group 1", "lights": []any{}}}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupSetCommand(logger)
	cmd.SetContext(ctx)
	// Provide all required args: group, property, value (no prompt)
	cmd.SetArgs([]string{"group1", "on", "true"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestGroupEditCommand(t *testing.T) {
	mock := &mockGroupClient{groups: map[string]map[string]any{"group1": {"id": "group1", "name": "Group 1", "lights": []any{"light1"}}}}
	ctx := context.WithValue(context.Background(), clientContextKey, mock)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	cmd := newGroupEditCommand(logger)
	cmd.SetContext(ctx)
	// Provide group and new light IDs as args (no prompt)
	cmd.SetArgs([]string{"group1", "light1"})
	err := cmd.Execute()
	require.NoError(t, err)
}
