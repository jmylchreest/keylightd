package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmylchreest/keylightd/config"
	"github.com/jmylchreest/keylightd/internal/server"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	var logLevel, logFormat string
	var discoveryInterval int

	rootCmd := &cobra.Command{
		Use:   "keylightd",
		Short: "Elgato Keylight Daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			level := getLogLevel(logLevel)
			var handler slog.Handler
			if logFormat == "json" {
				handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
			} else {
				handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
			}
			logger := slog.New(handler)

			logger.Info("Starting keylightd",
				"version", version,
				"commit", commit,
				"buildDate", buildDate,
			)

			cfg, err := config.Load()
			if err != nil {
				logger.Error("Failed to load configuration", "error", err)
				return err
			}

			// Override discovery interval if set via flag
			if discoveryInterval > 0 {
				cfg.Discovery.Interval = discoveryInterval
			}

			manager := keylight.NewManager(logger)
			srv := server.New(logger, manager, &server.Config{
				UnixSocket: cfg.Server.UnixSocket,
				AllowLocal: true,
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				if err := manager.DiscoverLights(ctx, time.Duration(cfg.Discovery.Interval)*time.Second); err != nil {
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

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := srv.Stop(shutdownCtx); err != nil {
				logger.Error("Error stopping server", "error", err)
			}

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text, json)")
	rootCmd.PersistentFlags().IntVar(&discoveryInterval, "discovery-interval", 0, "discovery interval in seconds (minimum 5 seconds, 0 to use config value)")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Build Date: %s\n", buildDate)
		},
	})

	if err := rootCmd.Execute(); err != nil {
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
