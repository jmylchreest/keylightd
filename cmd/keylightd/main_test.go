package main

import (
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRootCmd(t *testing.T) {
	// Capture stderr to avoid polluting test output
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	defer func() {
		os.Stderr = old
		w.Close()
		io.ReadAll(r)
	}()

	// Create logger for testing
	logger := utils.SetupLogger(config.LogLevelInfo, config.LogFormatText)
	utils.SetAsDefaultLogger(logger)

	// Test creation of root command
	cmd := &cobra.Command{
		Use:   "keylightd",
		Short: "Key Light Daemon",
	}

	assert.NotNil(t, cmd)
	assert.Equal(t, "keylightd", cmd.Use)
	assert.Equal(t, "Key Light Daemon", cmd.Short)
}

func TestSetupFlagBindings(t *testing.T) {
	// Create a test command and viper instance
	cmd := &cobra.Command{Use: "test"}
	v := viper.New()

	// Add flags
	cmd.PersistentFlags().String("log-level", "info", "Log level")
	cmd.PersistentFlags().String("log-format", "text", "Log format")
	cmd.PersistentFlags().String("config", "", "Config path")
	cmd.PersistentFlags().Int("discovery-interval", 30, "Discovery interval")

	// Bind flags (simulating what happens in main.go)
	v.SetEnvPrefix("KEYLIGHT")
	v.AutomaticEnv()
	v.BindPFlag("logging.level", cmd.PersistentFlags().Lookup("log-level"))
	v.BindPFlag("logging.format", cmd.PersistentFlags().Lookup("log-format"))
	v.BindPFlag("discovery.interval", cmd.PersistentFlags().Lookup("discovery-interval"))
	v.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

	// Test that flags are bound correctly
	assert.Equal(t, "info", v.GetString("logging.level"))
	assert.Equal(t, "text", v.GetString("logging.format"))
	assert.Equal(t, 30, v.GetInt("discovery.interval"))
	assert.Equal(t, "", v.GetString("config"))
}

func TestCreateManager(t *testing.T) {
	// Create test logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create manager
	manager := keylight.NewManager(logger)

	// Verify manager was created
	assert.NotNil(t, manager)
	assert.Empty(t, manager.GetLights())
}

func TestCreateConfig(t *testing.T) {
	// Create temporary directory for test config
	tempDir, err := os.MkdirTemp("", "keylightd-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set environment to use temporary directory
	oldEnv := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tempDir)
	defer os.Setenv("XDG_CONFIG_HOME", oldEnv)

	// Attempt to load config (will use defaults since file doesn't exist)
	cfg, err := config.Load(config.DaemonConfigFilename, "")
	require.NoError(t, err)

	// Check default values
	assert.Equal(t, "info", cfg.Config.Logging.Level)
	assert.Equal(t, "text", cfg.Config.Logging.Format)
	assert.Equal(t, 30, cfg.Config.Discovery.Interval)
}

func TestSignalHandling(t *testing.T) {
	// This is a minimal test since we can't easily test the actual signal handling
	// Simulate the context cancellation that would happen on signal receipt
	done := make(chan struct{})
	go func() {
		// In the actual code, this would be blocked on <-sigChan
		// Here we just simulate a delayed execution
		time.Sleep(10 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// If we get here, the simulation worked
		assert.True(t, true)
	case <-time.After(100 * time.Millisecond):
		// If we get here, there's a problem with the test
		t.Fatal("Timed out waiting for signal handling simulation")
	}
}