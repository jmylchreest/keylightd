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
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// Set up Viper for configuration
	v := viper.New()
	v.SetEnvPrefix("KEYLIGHT")
	v.AutomaticEnv()

	// Set up command line flags
	pflag.String("log-level", "info", "Log level (debug, info, warn, error)")
	pflag.String("log-format", "text", "Log format (text, json)")
	pflag.String("config", "", "Path to config file")
	pflag.Int("discovery-interval", 30, "Discovery interval in seconds")
	pflag.Parse()

	// Bind flags to Viper - this ensures flags take precedence
	v.BindPFlag("logging.level", pflag.Lookup("log-level"))
	v.BindPFlag("logging.format", pflag.Lookup("log-format"))
	v.BindPFlag("discovery.interval", pflag.Lookup("discovery-interval"))

	// Load configuration
	cfg, err := config.Load("keylightd.yaml", v.GetString("config"))
	if err != nil {
		// Create a basic logger for the error
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Set up logging with configured level - Viper will use flag value if set
	level := getLogLevel(v.GetString("logging.level"))
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
	srv := server.New(logger, manager, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := manager.DiscoverLights(ctx, time.Duration(cfg.Discovery.Interval)*time.Second); err != nil {
			logger.Error("Error discovering lights", "error", err)
		}
	}()

	if err := srv.Start(); err != nil {
		logger.Error("Failed to start server", "error", err)
		return
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
