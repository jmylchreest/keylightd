package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/keylightd/pkg/client"
)

// Desktop clients use the same authenticated transports and state units as the CLI.
func newRequestCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "request METHOD RESOURCE",
		Short:        "Access a light or group resource as JSON, reading updates from stdin",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ok := cmd.Context().Value(ClientContextKey).(client.ClientInterface)
			if !ok {
				return errors.New("no connection configured")
			}
			value, err := lightRequest(c, args[0], args[1], cmd.InOrStdin())
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(value)
		},
	}
}

func lightRequest(c client.ClientInterface, method, path string, input io.Reader) (any, error) {
	resource, rest, _ := strings.Cut(path, "/")
	id, action, _ := strings.Cut(rest, "/")
	if resource != "lights" && resource != "groups" {
		return nil, errors.New("expected a light or group resource")
	}
	switch {
	case method == "GET" && action == "":
		if resource == "lights" {
			if id == "" {
				return c.GetLights()
			}
			return c.GetLight(id)
		}
		if id == "" {
			return c.GetGroups()
		}
		return c.GetGroup(id)
	case method == "POST" && path == "groups":
		var body struct{ Name string }
		if err := json.NewDecoder(input).Decode(&body); err != nil {
			return nil, err
		}
		creator, ok := c.(interface {
			CreateGroupDetails(string) (map[string]any, error)
		})
		if !ok {
			return nil, errors.New("group creation unavailable")
		}
		return creator.CreateGroupDetails(body.Name)
	case method == "DELETE" && resource == "groups" && id != "" && action == "":
		return map[string]bool{"success": true}, c.DeleteGroup(id)
	case method == "PUT" && resource == "groups" && id != "" && action == "lights":
		var body struct {
			IDs []string `json:"light_ids"`
		}
		if err := json.NewDecoder(input).Decode(&body); err != nil {
			return nil, err
		}
		if body.IDs == nil {
			return nil, errors.New("light_ids must be an array")
		}
		return map[string]string{"status": "ok"}, c.SetGroupLights(id, body.IDs)
	case action == "state" && id != "" && ((resource == "lights" && method == "POST") || (resource == "groups" && method == "PUT")):
		var state map[string]any
		if err := json.NewDecoder(input).Decode(&state); err != nil {
			return nil, err
		}
		if len(state) != 1 {
			return nil, errors.New("expected one light property")
		}
		for key, value := range state {
			if key != "on" && key != "brightness" && key != "temperature" {
				return nil, errors.New("unknown light property")
			}
			set := c.SetLightState
			if resource == "groups" {
				set = c.SetGroupState
			}
			return map[string]string{"status": "ok"}, set(id, key, value)
		}
	}
	return nil, fmt.Errorf("unknown light request %s %s", method, path)
}
