package commands

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

type lightSetupClient interface {
	StartLightSetup(string) (map[string]any, error)
	GetLightSetup(string) (map[string]any, error)
	CancelLightSetup(string) (map[string]any, error)
	ForgetLightCredentials(string) error
}

type airMK2SetupClient interface {
	StartAirMK2Setup(string, keylight.AirMK2SetupOptions) (map[string]any, error)
}

func newLightSetupCommand() *cobra.Command {
	setup := &cobra.Command{Use: "setup", Short: "Manage a light's setup session over the HTTP API"}
	setup.AddCommand(newLightSetupStartCommand())
	for _, action := range []struct {
		name        string
		description string
		call        func(lightSetupClient, string) (map[string]any, error)
	}{
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

func newLightSetupStartCommand() *cobra.Command {
	var options keylight.AirMK2SetupOptions
	var passwordStdin bool
	start := &cobra.Command{
		Use: "start <id>", Short: "Start Air MK.2 setup", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := keylight.UnescapeRFC6763Label(args[0])
			if options.Peripheral == "" && options.SSID == "" && options.Port == 0 && !passwordStdin {
				c, ok := cmd.Context().Value(clientContextKey).(lightSetupClient)
				if !ok {
					return errors.New("light setup requires an HTTP API connection")
				}
				status, err := c.StartLightSetup(id)
				if err != nil {
					return err
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
			}
			c, ok := cmd.Context().Value(clientContextKey).(airMK2SetupClient)
			if !ok {
				return errors.New("light setup requires an HTTP API connection")
			}
			if options.Peripheral == "" || options.SSID == "" || options.Port < 1 || options.Port > 65535 {
				return errors.New("--peripheral, --ssid, and a valid --port are required")
			}
			if options.Security != 0 && !passwordStdin {
				return errors.New("use --password-stdin for a secured Wi-Fi network")
			}
			request := options
			if passwordStdin {
				password, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 67))
				if err != nil {
					return err
				}
				request.Password = strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r")
				clear(password)
			}
			status, err := c.StartAirMK2Setup(id, request)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
		},
	}
	start.Flags().StringVar(&options.Peripheral, "peripheral", "", "Bluetooth MAC address on Linux or peripheral UUID on macOS")
	start.Flags().StringVar(&options.SSID, "ssid", "", "Wi-Fi network name")
	start.Flags().Uint16Var(&options.Security, "security", 200, "Wi-Fi security code (200 for WPA2 AES, 0 for open)")
	start.Flags().IntVar(&options.Port, "port", 0, "WebSocket port advertised by the light")
	start.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read Wi-Fi password from standard input")
	return start
}
