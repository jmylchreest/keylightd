package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/keylightd/pkg/client"
)

// Define a custom type for context keys to avoid collisions
type loggerContextKey struct{}

// NewRootCommand creates the root command
func NewRootCommand(logger *slog.Logger, version, commit, buildDate string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keylightctl",
		Short: "Control Key Lights",
	}

	// Add global flags
	cmd.PersistentFlags().String("socket", "", "Path to keylightd socket")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().String("log-format", "text", "Log format (text, json)")

	// Add commands
	cmd.AddCommand(newVersionCommand(version, commit, buildDate))
	cmd.AddCommand(NewLightCommand(logger))
	cmd.AddCommand(NewGroupCommand(logger))
	cmd.AddCommand(NewAPIKeyCommand(logger))
	cmd.PersistentFlags().String("profile", "", "Use a paired HTTPS profile")
	httpsCmd := newHTTPSCommand(logger)
	cmd.AddCommand(httpsCmd, newRequestCommand())
	cmd.PersistentPreRunE = func(command *cobra.Command, _ []string) error {
		if command.Parent() == httpsCmd {
			return nil
		}
		socket, _ := command.Flags().GetString("socket")
		path, _ := command.Flags().GetString("profile")
		if path != "" && socket != "" {
			return errors.New("choose one of --profile and --socket")
		}
		var c client.ClientInterface
		if path != "" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var profile client.PairedProfile
			if err := json.Unmarshal(raw, &profile); err != nil {
				return err
			}
			if profile.APIKey == "" {
				return errors.New("missing paired API key")
			}
			c, err = client.NewPaired(logger, profile)
			if err != nil {
				return err
			}
		} else if socket != "" {
			c = client.New(logger, socket)
		}
		if c != nil {
			command.SetContext(context.WithValue(command.Context(), ClientContextKey, c))
		}
		return nil
	}

	if logger != nil {
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		cmd.SetContext(context.WithValue(parent, loggerContextKey{}, logger))
	}

	return cmd
}

// newVersionCommand creates the version command
func newVersionCommand(version, commit, buildDate string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Client:\n")
			fmt.Printf("  Version:    %s\n", version)
			fmt.Printf("  Commit:     %s\n", commit)
			fmt.Printf("  Build Date: %s\n", buildDate)

			// Try to query the daemon for its version
			if c, ok := cmd.Context().Value(ClientContextKey).(client.ClientInterface); ok {
				resp, err := c.GetVersion()
				if err == nil {
					fmt.Printf("\nDaemon:\n")
					if v, ok := resp["version"].(string); ok {
						fmt.Printf("  Version:    %s\n", v)
					}
					if c, ok := resp["commit"].(string); ok {
						fmt.Printf("  Commit:     %s\n", c)
					}
					if d, ok := resp["build_date"].(string); ok {
						fmt.Printf("  Build Date: %s\n", d)
					}
				} else {
					fmt.Printf("\nDaemon: not reachable\n")
				}
			}
		},
	}
}
