package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jmylchreest/keylightd/cmd/keylightctl/commands"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	var logLevel, logFormat string
	var configFile string

	// Load configuration first
	cfg, err := config.Load("keylightctl.yaml", configFile)
	if err != nil {
		// Only log error if it's not a missing config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Create a basic logger for the error
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			logger.Error("failed to load configuration", "error", err)
			os.Exit(1)
		}
		// Create default config if file not found
		cfg = &config.Config{
			Logging: config.LoggingConfig{
				Level:  "info",
				Format: "text",
			},
		}
	}

	// Override config with command line flags if set
	if logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	if logFormat != "" {
		cfg.Logging.Format = logFormat
	}

	// Set up logging with configured level
	level := getLogLevel(cfg.Logging.Level)
	var handler slog.Handler
	if cfg.Logging.Format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Set socket path from config
	socket := "/run/user/1000/keylightd.sock"
	if cfg.Server.UnixSocket != "" {
		socket = cfg.Server.UnixSocket
	}

	client := client.New(logger, socket)

	// Create context with client and logger
	ctx := context.WithValue(context.Background(), "client", client)
	ctx = context.WithValue(ctx, "logger", logger)

	rootCmd := &cobra.Command{
		Use:   "keylightctl",
		Short: "Elgato Keylight Control",
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text, json)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to configuration file")

	// Create commands with logger
	rootCmd.AddCommand(commands.NewGroupCommand(logger))
	rootCmd.AddCommand(commands.NewLightCommand(logger))

	if err := rootCmd.ExecuteContext(ctx); err != nil {
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
