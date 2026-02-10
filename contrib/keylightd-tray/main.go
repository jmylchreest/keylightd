package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// acquireLock attempts to create a PID lockfile. If another instance is
// already running, it prints a message and exits. The lockfile is removed
// automatically when the process exits.
func acquireLock() func() {
	dir := os.TempDir()
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dir = xdg
	}
	lockPath := filepath.Join(dir, "keylightd-tray.pid")

	// Check for an existing lockfile
	if data, err := os.ReadFile(lockPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Signal 0 checks if the process exists without actually signaling it
			if err := syscall.Kill(pid, 0); err == nil {
				fmt.Fprintf(os.Stderr, "keylightd-tray is already running (pid %d)\n", pid)
				os.Exit(1)
			}
		}
		// Stale lockfile — remove it
		os.Remove(lockPath)
	}

	// Write our PID
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lockfile %s: %v\n", lockPath, err)
		os.Exit(1)
	}

	return func() { os.Remove(lockPath) }
}

func main() {
	// Prevent multiple instances
	cleanup := acquireLock()
	defer cleanup()

	// Parse command-line flags
	customCSSPath := flag.String("css", "", "Path to custom CSS file (default: $XDG_CONFIG_HOME/keylightd/keylightd-tray/custom.css)")
	flag.Parse()

	// Create application with options
	app := NewApp(version, commit, buildDate)
	app.SetCustomCSSPath(*customCSSPath)
	tray := NewTrayManager(app)
	app.SetTrayManager(tray)

	// Run systray in a goroutine
	go systray.Run(tray.OnReady, tray.OnExit)

	err := wails.Run(&options.App{
		Title:             "Keylight Control",
		Width:             380,
		Height:            600,
		MinWidth:          320,
		MinHeight:         400,
		StartHidden:       true, // Start hidden, show via tray
		HideWindowOnClose: true, // Close to tray instead of quit
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 46, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []any{
			app,
		},
		Linux: &linux.Options{
			ProgramName: "keylightd-tray",
		},
		Frameless: false,
	})

	if err != nil {
		log.Fatal(err)
	}
}
