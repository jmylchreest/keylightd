package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	app := &App{}

	// Test with XDG_CONFIG_HOME set
	t.Run("with XDG_CONFIG_HOME", func(t *testing.T) {
		oldVal := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", oldVal)

		os.Setenv("XDG_CONFIG_HOME", "/tmp/test-config")
		result := app.getConfigDir()
		expected := "/tmp/test-config/keylightd/keylightd-tray"
		if result != expected {
			t.Errorf("getConfigDir() = %s, want %s", result, expected)
		}
	})

	// Test without XDG_CONFIG_HOME (uses ~/.config)
	t.Run("without XDG_CONFIG_HOME", func(t *testing.T) {
		oldVal := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", oldVal)

		os.Unsetenv("XDG_CONFIG_HOME")
		result := app.getConfigDir()
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".config", "keylightd", "keylightd-tray")
		if result != expected {
			t.Errorf("getConfigDir() = %s, want %s", result, expected)
		}
	})
}

func TestGetCustomCSSPath(t *testing.T) {
	// Test with custom path set
	t.Run("with custom path", func(t *testing.T) {
		app := &App{customCSSPath: "/custom/path/theme.css"}
		result := app.getCustomCSSPath()
		if result != "/custom/path/theme.css" {
			t.Errorf("getCustomCSSPath() = %s, want /custom/path/theme.css", result)
		}
	})

	// Test with default path
	t.Run("with default path", func(t *testing.T) {
		oldVal := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", oldVal)

		os.Setenv("XDG_CONFIG_HOME", "/tmp/test-config")
		app := &App{}
		result := app.getCustomCSSPath()
		expected := "/tmp/test-config/keylightd/keylightd-tray/custom.css"
		if result != expected {
			t.Errorf("getCustomCSSPath() = %s, want %s", result, expected)
		}
	})
}

func TestGetCustomCSS(t *testing.T) {
	// Create a temp directory and CSS file
	tmpDir := t.TempDir()
	cssPath := filepath.Join(tmpDir, "custom.css")
	cssContent := ":root { --bg-primary: #000; }"

	if err := os.WriteFile(cssPath, []byte(cssContent), 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{customCSSPath: cssPath}
	result := app.GetCustomCSS()
	if result != cssContent {
		t.Errorf("GetCustomCSS() = %s, want %s", result, cssContent)
	}

	// Test with non-existent file
	app2 := &App{customCSSPath: "/nonexistent/path/custom.css"}
	result2 := app2.GetCustomCSS()
	if result2 != "" {
		t.Errorf("GetCustomCSS() for non-existent file = %s, want empty string", result2)
	}
}

func TestGetVersion(t *testing.T) {
	app := &App{
		version:   "1.0.0",
		commit:    "abc123",
		buildDate: "2024-01-01",
	}

	result := app.GetVersion()
	expected := "1.0.0, commit: abc123, date: 2024-01-01"
	if result != expected {
		t.Errorf("GetVersion() = %s, want %s", result, expected)
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp("1.0.0", "abc123", "2024-01-01")

	if app.version != "1.0.0" {
		t.Errorf("NewApp version = %s, want 1.0.0", app.version)
	}
	if app.commit != "abc123" {
		t.Errorf("NewApp commit = %s, want abc123", app.commit)
	}
	if app.buildDate != "2024-01-01" {
		t.Errorf("NewApp buildDate = %s, want 2024-01-01", app.buildDate)
	}
}

func TestSetCustomCSSPath(t *testing.T) {
	app := &App{}
	app.SetCustomCSSPath("/custom/path.css")
	if app.customCSSPath != "/custom/path.css" {
		t.Errorf("SetCustomCSSPath() did not set path correctly, got %s", app.customCSSPath)
	}
}
