package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jmylchreest/keylightd/cmd/keylightctl/commands"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// Define the same clientContextKey as in commands/light.go and group.go
var clientContextKey = &struct{}{}

func main() {
	var logLevel, logFormat string
	var configFile string

	// Load configuration first
	cfg, err := config.Load("keylightctl.yaml", configFile)
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			logger.Error("failed to load configuration", "error", err)
			os.Exit(1)
		}
		// If file not found, use defaults
		cfg = &config.Config{
			Config: config.ConfigBlock{
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "text",
				},
			},
		}
	}

	// Override config with command line flags if set
	if logLevel != "" {
		cfg.Config.Logging.Level = logLevel
	}
	if logFormat != "" {
		cfg.Config.Logging.Format = logFormat
	}

	// Set up logging with configured level
	level := utils.GetLogLevel(cfg.Config.Logging.Level)
	var handler slog.Handler
	if cfg.Config.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Set socket path from config
	socket := config.GetRuntimeSocketPath()
	if cfg.Config.Server.UnixSocket != "" {
		socket = cfg.Config.Server.UnixSocket
	}

	apiClient := client.New(logger, socket)

	// Use the NewRootCommand from the commands package
	rootCmd := commands.NewRootCommand(logger, version, commit, buildDate)

	// Get the context initialized by NewRootCommand (which includes the logger)
	// and add the apiClient to it.
	ctx := rootCmd.Context()
	if ctx == nil { // Should not happen if NewRootCommand sets it up
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, clientContextKey, apiClient)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// Using utils.GetLogLevel instead
