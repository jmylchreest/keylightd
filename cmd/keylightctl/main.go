package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jmylchreest/keylightd/cmd/keylightctl/commands"
	"github.com/jmylchreest/keylightd/pkg/client"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create client
	c := client.New(logger, "/run/user/1000/keylightd.sock")

	// Create root command
	rootCmd := commands.NewRootCommand(logger, version, commit, buildDate)

	// Create context with client
	ctx := context.WithValue(context.Background(), "client", c)

	// Execute command with context
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Error("Error executing command", "error", err)
		os.Exit(1)
	}
}

func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
