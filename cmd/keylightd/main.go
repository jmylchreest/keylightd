package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/errors"
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
		Use:     "keylightd",
		Short:   "Key Light Daemon",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate),
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
			cfg, err := config.Load(config.DaemonConfigFilename, v.GetString("config"))
			if err != nil {
				logger := utils.SetupErrorLogger()
				logger.Error("failed to load configuration", "error", err)
				os.Exit(1)
			}

			// Set up logging with configured level
			logger := utils.SetupLogger(v.GetString("logging.level"), v.GetString("logging.format"))
			utils.SetAsDefaultLogger(logger)

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
				// Convert interval from seconds to duration
				interval := time.Duration(cfg.Config.Discovery.Interval) * time.Second
				// Start supervised discovery loop that auto-restarts on panic,
				// and exits cleanly when ctx is canceled.
				manager.StartDiscoveryWithRestart(ctx, interval)
				logger.Debug("Discovery routine terminated")
			}()

			if err := srv.Start(); err != nil {
				return errors.LogErrorAndReturn(logger, err, "Failed to start server")
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
	rootCmd.PersistentFlags().String("log-level", config.LogLevelInfo, "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", config.LogFormatText, "Log format (text, json)")
	rootCmd.PersistentFlags().String("config", "", "Path to config file")
	rootCmd.PersistentFlags().Int("discovery-interval", int(config.DefaultDiscoveryInterval.Seconds()),
		fmt.Sprintf("Discovery interval in seconds (minimum: %d)", int(config.MinDiscoveryInterval.Seconds())))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Using utils.GetLogLevel instead
