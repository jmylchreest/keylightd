package commands

import (
	"log/slog"

	"github.com/spf13/cobra"
)

// getLoggerFromCmd returns the slog.Logger from the root command context
func getLoggerFromCmd(cmd *cobra.Command) *slog.Logger {
	if root := cmd.Root(); root != nil {
		if logger, ok := root.Context().Value("logger").(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}
