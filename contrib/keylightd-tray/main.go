package main

import (
	"embed"
	"flag"
	"log"

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

func main() {
	// Prevent multiple instances (skipped during wails binding generation
	// via the "bindings" build tag — see lock_nobindings.go / lock_bindings.go)
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
