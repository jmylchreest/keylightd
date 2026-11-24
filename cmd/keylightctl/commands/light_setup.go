package commands

import (
	"encoding/json"
	"errors"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

type lightSetupClient interface {
	StartLightSetup(string) (map[string]any, error)
	GetLightSetup(string) (map[string]any, error)
	CancelLightSetup(string) (map[string]any, error)
	ForgetLightCredentials(string) error
}

func newLightSetupCommand() *cobra.Command {
	setup := &cobra.Command{Use: "setup", Short: "Manage a light's setup session over the HTTP API"}
	for _, action := range []struct {
		name        string
		description string
		call        func(lightSetupClient, string) (map[string]any, error)
	}{
		{"start", "Start setup", lightSetupClient.StartLightSetup},
		{"status", "Show setup status", lightSetupClient.GetLightSetup},
		{"cancel", "Cancel setup", lightSetupClient.CancelLightSetup},
		{"forget", "Remove saved credentials", func(c lightSetupClient, id string) (map[string]any, error) {
			return map[string]any{"status": "forgotten"}, c.ForgetLightCredentials(id)
		}},
	} {
		setup.AddCommand(&cobra.Command{Use: action.name + " <id>", Short: action.description, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			c, ok := cmd.Context().Value(clientContextKey).(lightSetupClient)
			if !ok {
				return errors.New("light setup requires an HTTP API connection")
			}
			status, err := action.call(c, keylight.UnescapeRFC6763Label(args[0]))
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
		}})
	}
	return setup
}
