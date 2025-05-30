package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
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
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Build Date: %s\n", buildDate)
		},
	}
}
