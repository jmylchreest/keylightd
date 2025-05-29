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

// Define the same clientContextKey as in commands/light.go and group.go
var clientContextKey = &struct{}{}

func main() {
	var logLevel, logFormat string
	var configFile string

	// Load configuration first
	cfg, err := config.Load(config.ClientConfigFilename, configFile)
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

	// Override config with command line flags if set
	if logLevel != "" {
		cfg.Config.Logging.Level = logLevel
	}
	if logFormat != "" {
		cfg.Config.Logging.Format = logFormat
	}

	// Set up logging with configured level
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

	// Check for socket flag override
	if socketFlag, _ := rootCmd.PersistentFlags().GetString("socket"); socketFlag != "" {
		socket = socketFlag
	}

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

// Using utils.GetLogLevel instead
