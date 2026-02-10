package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	logfilter "github.com/jmylchreest/slog-logfilter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/errors"
	"github.com/jmylchreest/keylightd/internal/logging"
	"github.com/jmylchreest/keylightd/internal/server"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/pkg/keylight"
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

			// Validate any configured log filters before applying
			level := v.GetString("logging.level")
			format := v.GetString("logging.format")
			filters := cfg.Config.Logging.Filters
			if len(filters) > 0 {
				if errs := logging.ValidateFilters(filters); len(errs) > 0 {
					// Log warnings but start with no filters rather than crashing
					errLogger := utils.SetupErrorLogger()
					errLogger.Warn("Invalid log filters in config, starting without filters",
						"errors", logging.FormatErrors(errs))
					filters = nil
				}
			}

			// Set up logging backed by slog-logfilter.
			// Using SetDefault so the package-level hot-reload functions
			// (logfilter.SetLevel, logfilter.SetFilters, etc.) work.
			logger := utils.SetupLoggerWithFilters(level, format, filters)
			utils.SetAsDefaultLogger(logger)

			logger.Info("Starting keylightd",
				"version", version,
				"commit", commit,
				"buildDate", buildDate,
			)
			if len(filters) > 0 {
				logger.Info("Log filters active", "count", len(filters))
			}

			manager := keylight.NewManager(logger)
			srv := server.New(logger, cfg, manager, server.VersionInfo{
				Version:   version,
				Commit:    commit,
				BuildDate: buildDate,
			})

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

			// Watch config file for logging hot-reload (level + filters).
			// Uses viper's built-in fsnotify watcher.
			configViper := cfg.Viper()
			if configViper != nil {
				configViper.OnConfigChange(func(e fsnotify.Event) {
					logger.Info("Config file changed, reloading logging configuration", "file", e.Name)
					reloadLoggingConfig(logger, configViper)
				})
				configViper.WatchConfig()
				logger.Debug("Config file watcher started")
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

// reloadLoggingConfig handles hot-reload of logging level and filters when
// the config file changes.  It validates filters before applying them; invalid
// filters are rejected and the existing configuration is kept.
func reloadLoggingConfig(logger *slog.Logger, v *viper.Viper) {
	// Re-read level
	newLevel := utils.ValidateLogLevel(v.GetString("config.logging.level"))
	slogLevel := utils.GetLogLevel(newLevel)
	logfilter.SetLevel(slogLevel)
	logger.Info("Log level updated", "level", newLevel)

	// Re-read filters from the raw config
	var loggingCfg config.LoggingConfig
	if err := v.UnmarshalKey("config.logging", &loggingCfg); err != nil {
		logger.Error("Failed to unmarshal logging config on reload", "error", err)
		return
	}

	if errs := logging.ValidateFilters(loggingCfg.Filters); len(errs) > 0 {
		logger.Warn("Invalid log filters in config reload, keeping existing filters",
			"errors", logging.FormatErrors(errs))
		return
	}

	logfilter.SetFilters(loggingCfg.Filters)
	logger.Info("Log filters reloaded", "count", len(loggingCfg.Filters))
}
