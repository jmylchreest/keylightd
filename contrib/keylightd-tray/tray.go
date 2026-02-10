package main

import (
	_ "embed"
	"strconv"
	"sync"

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
	mu             sync.Mutex
	app            *App
	mShow          *systray.MenuItem
	mQuit          *systray.MenuItem
	windowShown    bool
	groupMenus     map[string]*systray.MenuItem
	lightMenus     map[string]*systray.MenuItem
	stopChan       chan struct{}
	menuBuilt      bool
	isBasicMenu    bool
	lastGroupCount int
	lastLightCount int
}

// NewTrayManager creates a new tray manager
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{
		app:         app,
		windowShown: false,
		groupMenus:  make(map[string]*systray.MenuItem),
		lightMenus:  make(map[string]*systray.MenuItem),
		stopChan:    make(chan struct{}),
	}
}

// OnReady is called when systray is ready
func (t *TrayManager) OnReady() {
	systray.SetIcon(iconUnknown)
	systray.SetTitle("Keylight Control")
	systray.SetTooltip("Keylight Control")

	// Set left-click to toggle window
	systray.SetOnTapped(func() {
		t.ToggleWindow()
	})

	// Right-click shows menu automatically (default behavior)
	// SetOnSecondaryTapped is optional - menu shows by default on right-click
	// but we can set it explicitly if we want to customize behavior later
	systray.SetOnSecondaryTapped(func() {
		// Menu will be shown automatically
	})

	// Build initial basic menu
	t.buildBasicMenu()
}

// UpdateMenu updates the menu based on the current status (called by app when status changes).
// A mutex serialises access so that concurrent GetStatus polls (which overlap when
// light HTTP requests are slow/timing out) cannot race through rebuildMenuStructure
// and leave the menu half-built (e.g. missing the Quit item).
func (t *TrayManager) UpdateMenu(status *Status) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if we need to rebuild (number of groups/lights changed OR upgrading from basic menu)
	groupCount := len(status.Groups)
	lightCount := len(status.Lights)
	needsRebuild := !t.menuBuilt || t.isBasicMenu || groupCount != t.lastGroupCount || lightCount != t.lastLightCount

	if needsRebuild {
		t.rebuildMenuStructure(status)
		t.lastGroupCount = groupCount
		t.lastLightCount = lightCount
		t.menuBuilt = true
		t.isBasicMenu = false
	} else {
		// Just update existing menu item titles
		t.updateMenuTitles(status)
	}
}

// buildBasicMenu builds a basic Show/Quit menu when status is unavailable
func (t *TrayManager) buildBasicMenu() {
	t.mShow = systray.AddMenuItem("Show", "Show the window")
	t.mQuit = systray.AddMenuItem("Quit", "Quit the application")
	t.menuBuilt = true
	t.isBasicMenu = true

	go t.handleShowQuitClicks()
}

// rebuildMenuStructure completely rebuilds the menu structure
func (t *TrayManager) rebuildMenuStructure(status *Status) {
	// Stop existing handlers if rebuilding
	if t.menuBuilt {
		// Signal to stop old handlers
		close(t.stopChan)
		// Create new stop channel
		t.stopChan = make(chan struct{})
		// Reset menu completely
		systray.ResetMenu()
	}

	// Clear maps
	t.groupMenus = make(map[string]*systray.MenuItem)
	t.lightMenus = make(map[string]*systray.MenuItem)

	// 1. Show/Hide at top
	t.mShow = systray.AddMenuItem("Show", "Show the window")

	// 2. Groups section
	if len(status.Groups) > 0 {
		groupsHeader := systray.AddMenuItem("Groups", "Groups section")
		groupsHeader.Disable()

		for _, group := range status.Groups {
			title := formatMenuTitle(group.Name, group.On)
			item := systray.AddMenuItem(title, "Toggle group")
			t.groupMenus[group.ID] = item
			go t.handleGroupMenuItem(group.ID, item)
		}
	}

	// 3. All lights section
	if len(status.Lights) > 0 {
		lightsHeader := systray.AddMenuItem("Lights", "Lights section")
		lightsHeader.Disable()

		for _, light := range status.Lights {
			title := formatMenuTitle(light.Name, light.On)
			item := systray.AddMenuItem(title, "Toggle light")
			t.lightMenus[light.ID] = item
			go t.handleLightMenuItem(light.ID, item)
		}
	}

	// 4. Separator + Quit at bottom
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Start handlers for show/quit
	go t.handleShowQuitClicks()
}

// handleShowQuitClicks handles clicks on Show and Quit menu items
func (t *TrayManager) handleShowQuitClicks() {
	for {
		select {
		case <-t.mShow.ClickedCh:
			t.ToggleWindow()
		case <-t.mQuit.ClickedCh:
			close(t.stopChan)
			t.app.Quit()
			systray.Quit()
			return
		case <-t.stopChan:
			return
		}
	}
}

// handleGroupMenuItem handles clicks on a specific group menu item
func (t *TrayManager) handleGroupMenuItem(groupID string, item *systray.MenuItem) {
	for {
		select {
		case <-item.ClickedCh:
			t.toggleGroup(groupID)
		case <-t.stopChan:
			return
		}
	}
}

// handleLightMenuItem handles clicks on a specific light menu item
func (t *TrayManager) handleLightMenuItem(lightID string, item *systray.MenuItem) {
	for {
		select {
		case <-item.ClickedCh:
			t.toggleLight(lightID)
		case <-t.stopChan:
			return
		}
	}
}

// toggleGroup toggles a group's state
func (t *TrayManager) toggleGroup(groupID string) {
	status, err := t.app.GetStatus()
	if err != nil {
		return
	}

	for _, group := range status.Groups {
		if group.ID == groupID {
			_ = t.app.SetGroupState(groupID, "on", !group.On)
			return
		}
	}
}

// toggleLight toggles a light's state
func (t *TrayManager) toggleLight(lightID string) {
	status, err := t.app.GetStatus()
	if err != nil {
		return
	}

	for _, light := range status.Lights {
		if light.ID == lightID {
			_ = t.app.SetLightState(lightID, "on", !light.On)
			return
		}
	}
}

// updateMenuTitles updates the titles of existing menu items based on current state
func (t *TrayManager) updateMenuTitles(status *Status) {
	for _, group := range status.Groups {
		if item, exists := t.groupMenus[group.ID]; exists {
			item.SetTitle(formatMenuTitle(group.Name, group.On))
		}
	}

	for _, light := range status.Lights {
		if item, exists := t.lightMenus[light.ID]; exists {
			item.SetTitle(formatMenuTitle(light.Name, light.On))
		}
	}
}

// formatCount formats a count as a string for display in menus and tooltips.
func formatCount(count int) string {
	return strconv.Itoa(count)
}

// formatMenuTitle formats a menu item title with optional checkmark
func formatMenuTitle(name string, checked bool) string {
	if checked {
		return "✓ " + name
	}
	return "  " + name
}

// OnExit is called when systray exits
func (t *TrayManager) OnExit() {
	// Cleanup if needed
}

// ToggleWindow toggles the window visibility.
// Called from tray click handlers — updates state directly to avoid the
// app.ShowWindow → tray.SetWindowShown callback loop that would deadlock the mutex.
func (t *TrayManager) ToggleWindow() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.windowShown {
		t.windowShown = false
		t.mShow.SetTitle("Show")
		// Call runtime directly to avoid app.HideWindow → tray.SetWindowShown re-lock
		t.app.hideWindowDirect()
	} else {
		t.windowShown = true
		t.mShow.SetTitle("Hide")
		t.app.showWindowDirect()
	}
}

// SetWindowShown updates the window shown state (called from app.go when
// the window is shown/hidden externally, e.g. via Wails runtime).
func (t *TrayManager) SetWindowShown(shown bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.windowShown = shown
	if shown {
		t.mShow.SetTitle("Hide")
	} else {
		t.mShow.SetTitle("Show")
	}
}

// UpdateIconAndTooltip updates the icon and tooltip based on full status
func (t *TrayManager) UpdateIconAndTooltip(status *Status) {
	if status.Total == 0 {
		systray.SetIcon(iconUnknown)
		systray.SetTooltip("Keylight Control - No lights")
		return
	}

	// Update icon based on on/off count
	if status.OnCount > 0 {
		systray.SetIcon(iconEnabled)
	} else {
		systray.SetIcon(iconDisabled)
	}

	// Build detailed tooltip with groups and lights using same format as menu
	tooltip := "Keylight Control\n"

	// Add groups section
	if len(status.Groups) > 0 {
		tooltip += "\nGroups\n"
		for _, group := range status.Groups {
			tooltip += formatMenuTitle(group.Name, group.On) + "\n"
		}
	}

	// Add lights section
	if len(status.Lights) > 0 {
		tooltip += "\nLights\n"
		for _, light := range status.Lights {
			tooltip += formatMenuTitle(light.Name, light.On) + "\n"
		}
	}

	systray.SetTooltip(tooltip)
}
