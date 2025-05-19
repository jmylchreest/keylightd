package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/server"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "keylightd",
		Short: "Elgato Keylight Daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.New()
			v.SetEnvPrefix("KEYLIGHT")
			v.AutomaticEnv()

			// Bind flags to viper
			v.BindPFlag("logging.level", cmd.PersistentFlags().Lookup("log-level"))
			v.BindPFlag("logging.format", cmd.PersistentFlags().Lookup("log-format"))
			v.BindPFlag("discovery.interval", cmd.PersistentFlags().Lookup("discovery-interval"))
			v.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

			// Load configuration
			cfg, err := config.Load("keylightd.yaml", v.GetString("config"))
			if err != nil {
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
				logger.Error("failed to load configuration", "error", err)
				os.Exit(1)
			}

			// Set up logging with configured level
			level := utils.GetLogLevel(v.GetString("logging.level"))
			format := v.GetString("logging.format")
			var handler slog.Handler
			if format == "json" {
				handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
			} else {
				handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
			}
			logger := slog.New(handler)
			slog.SetDefault(logger)

			logger.Info("Starting keylightd",
				"version", version,
				"commit", commit,
				"buildDate", buildDate,
			)

			manager := keylight.NewManager(logger)
			srv := server.New(logger, cfg, manager)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				if err := manager.DiscoverLights(ctx, time.Duration(cfg.Config.Discovery.Interval)*time.Second); err != nil {
					logger.Error("Error discovering lights", "error", err)
				}
			}()

			if err := srv.Start(); err != nil {
				logger.Error("Failed to start server", "error", err)
				return err
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan
			logger.Info("Shutting down...")
			cancel()

			srv.Stop()
			return nil
		},
	}

	// Define flags using Cobra (pflag under the hood)
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().String("config", "", "Path to config file")
	rootCmd.PersistentFlags().Int("discovery-interval", 30, "Discovery interval in seconds")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Using utils.GetLogLevel instead
