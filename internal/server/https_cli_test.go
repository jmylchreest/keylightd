package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/keylightd/cmd/keylightctl/commands"
)

// Run the commands used by GNOME against both transports, including the
// pairing exchange and the verified profile read back from disk.
func TestPairedCLI(t *testing.T) {
	srv, _, _ := setupHTTPIntegrationTest(t)
	require.NoError(t, srv.Start())
	t.Cleanup(srv.Stop)
	run := func(input string, args ...string) (string, error) {
		cmd := commands.NewRootCommand(srv.logger, "test", "", "")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetIn(strings.NewReader(input))
		cmd.SetArgs(args)
		err := cmd.Execute()
		return strings.TrimSpace(out.String()), err
	}
	ok := func(input string, args ...string) string {
		t.Helper()
		value, err := run(input, args...)
		require.NoError(t, err)
		return value
	}
	admin := func(action string) string { return ok("", "--socket", srv.socketPath, "https", action) }
	require.Contains(t, admin("status"), `"enabled":false`)
	admin("enable")
	invite := admin("invite")
	profile := filepath.Join(t.TempDir(), "client", "profile.json")
	ok(invite, "--profile", profile, "https", "pair")
	info, err := os.Stat(profile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	_, err = run(invite, "--profile", profile, "https", "pair")
	require.ErrorContains(t, err, "already used")
	for _, route := range [][]string{{"--socket", srv.socketPath}, {"--profile", profile}} {
		request := func(method, resource, body string) string {
			t.Helper()
			return ok(body, route[0], route[1], "request", method, resource)
		}
		require.Contains(t, request("GET", "lights", ""), "test-light-1")
		require.Contains(t, request("GET", "lights/test-light-1", ""), "brightness")
		raw := request("POST", "groups", `{"name":"Desk"}`)
		var group struct{ ID string }
		require.NoError(t, json.Unmarshal([]byte(raw), &group))
		require.NotEmpty(t, group.ID)
		path := "groups/" + group.ID
		request("PUT", path+"/lights", `{"light_ids":["test-light-1"]}`)
		require.Contains(t, request("GET", path, ""), "test-light-1")
		require.Contains(t, request("GET", "groups", ""), group.ID)
		request("PUT", path+"/state", `{"brightness":42}`)
		request("POST", "lights/test-light-1/state", `{"on":true}`)
		request("DELETE", path, "")
		require.NotContains(t, request("GET", "groups", ""), group.ID)
	}
	admin("disable")
	admin("delete-identity")
	admin("enable")
	_, err = run("", "--profile", profile, "request", "GET", "lights")
	require.Error(t, err)
}
