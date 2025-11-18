package main

import (
	_ "embed"

	"fyne.io/systray"
)

//go:embed assets/light-enabled.png
var iconEnabled []byte

//go:embed assets/light-disabled.png
var iconDisabled []byte

//go:embed assets/light-unknown.png
var iconUnknown []byte

// TrayManager handles the system tray functionality
type TrayManager struct {
	app         *App
	mShow       *systray.MenuItem
	mQuit       *systray.MenuItem
	windowShown bool
}

// NewTrayManager creates a new tray manager
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{
		app:         app,
		windowShown: false,
	}
}

// OnReady is called when systray is ready
func (t *TrayManager) OnReady() {
	systray.SetIcon(iconUnknown)
	systray.SetTitle("Keylight Control")
	systray.SetTooltip("Keylight Control")

	t.mShow = systray.AddMenuItem("Show", "Show the window")
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		for {
			select {
			case <-t.mShow.ClickedCh:
				t.ToggleWindow()
			case <-t.mQuit.ClickedCh:
				t.app.Quit()
				systray.Quit()
				return
			}
		}
	}()
}

// OnExit is called when systray exits
func (t *TrayManager) OnExit() {
	// Cleanup if needed
}

// ToggleWindow toggles the window visibility
func (t *TrayManager) ToggleWindow() {
	if t.windowShown {
		t.app.HideWindow()
		t.mShow.SetTitle("Show")
		t.windowShown = false
	} else {
		t.app.ShowWindow()
		t.mShow.SetTitle("Hide")
		t.windowShown = true
	}
}

// SetWindowShown updates the window shown state
func (t *TrayManager) SetWindowShown(shown bool) {
	t.windowShown = shown
	if shown {
		t.mShow.SetTitle("Hide")
	} else {
		t.mShow.SetTitle("Show")
	}
}

// UpdateIcon updates the tray icon based on light status
// onCount: number of lights that are on
// total: total number of lights
func (t *TrayManager) UpdateIcon(onCount, total int) {
	if total == 0 {
		systray.SetIcon(iconUnknown)
		systray.SetTooltip("Keylight Control - No lights")
	} else if onCount > 0 {
		systray.SetIcon(iconEnabled)
		systray.SetTooltip("Keylight Control - " + string(rune('0'+onCount)) + "/" + string(rune('0'+total)) + " on")
	} else {
		systray.SetIcon(iconDisabled)
		systray.SetTooltip("Keylight Control - All off")
	}
}

// UpdateIconFromStatus updates the icon from a Status struct
func (t *TrayManager) UpdateIconFromStatus(onCount, total int) {
	if total == 0 {
		systray.SetIcon(iconUnknown)
		systray.SetTooltip("Keylight Control - No lights")
		return
	}

	if onCount > 0 {
		systray.SetIcon(iconEnabled)
	} else {
		systray.SetIcon(iconDisabled)
	}

	// Format tooltip
	tooltip := "Keylight Control - "
	if onCount == 0 {
		tooltip += "All off"
	} else if onCount == total {
		tooltip += "All on"
	} else {
		tooltip += formatCount(onCount) + "/" + formatCount(total) + " on"
	}
	systray.SetTooltip(tooltip)
}

// formatCount formats a number as a string
func formatCount(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	// For numbers >= 10, convert properly
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if result == "" {
		return "0"
	}
	return result
}
