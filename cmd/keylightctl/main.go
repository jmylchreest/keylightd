package main

import (
	"context"
	"os"

	"github.com/jmylchreest/keylightd/cmd/keylightctl/commands"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// Use the exported ClientContextKey from the commands package to ensure
// all command handlers retrieve the client from the same context key.
var clientContextKey = commands.ClientContextKey

func main() {
	// Load configuration first
	cfg, err := config.Load(config.ClientConfigFilename, "")
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger := utils.SetupErrorLogger()
			logger.Error("failed to load configuration", "error", err)
			os.Exit(1)
		}
		// If file not found, use defaults
		cfg = &config.Config{
			Config: config.ConfigBlock{
				Logging: config.LoggingConfig{
					Level:  config.LogLevelInfo,
					Format: config.LogFormatText,
				},
			},
		}
	}

	// Set up logging with the configured level and format
	logger := utils.SetupLogger(cfg.Config.Logging.Level, cfg.Config.Logging.Format)
	utils.SetAsDefaultLogger(logger)

	// Set socket path using the new utility function
	socket := config.GetRuntimeSocketPath()
	if cfg.Config.Server.UnixSocket != "" {
		socket = cfg.Config.Server.UnixSocket
	}

	// Use the NewRootCommand from the commands package
	rootCmd := commands.NewRootCommand(logger, version, commit, buildDate)

	apiClient := client.New(logger, socket)

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
