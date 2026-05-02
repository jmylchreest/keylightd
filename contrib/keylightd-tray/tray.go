package main

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"

	"fyne.io/systray"
)

//go:embed assets/light-enabled.png
var iconEnabled []byte

//go:embed assets/light-disabled.png
var iconDisabled []byte

//go:embed assets/light-unknown.png
var iconUnknown []byte

// iconKey identifies which embedded icon was last sent to the systray.
// systray.SetIcon re-decodes the PNG and emits a DBus PropertiesChanged on
// every call, so we skip the call when the key is unchanged.
type iconKey int

const (
	iconKeyNone iconKey = iota
	iconKeyUnknown
	iconKeyEnabled
	iconKeyDisabled
)

// TrayManager handles the system tray functionality
type TrayManager struct {
	mu              sync.Mutex
	app             *App
	mShow           *systray.MenuItem
	mQuit           *systray.MenuItem
	windowShown     bool
	groupMenus      map[string]*systray.MenuItem
	lightMenus      map[string]*systray.MenuItem
	stopChan        chan struct{}
	menuBuilt       bool
	isBasicMenu     bool
	lastGroupCount  int
	lastLightCount  int
	lastIconKey     iconKey
	lastTooltip     string
	lastGroupTitles map[string]string
	lastLightTitles map[string]string
}

// NewTrayManager creates a new tray manager
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{
		app:             app,
		windowShown:     false,
		groupMenus:      make(map[string]*systray.MenuItem),
		lightMenus:      make(map[string]*systray.MenuItem),
		lastGroupTitles: make(map[string]string),
		lastLightTitles: make(map[string]string),
		stopChan:        make(chan struct{}),
	}
}

// diffEmit calls emit(next) only when next differs from *last, updating *last
// on emission. Used to suppress redundant systray/DBus signals when polled
// status is unchanged.
func diffEmit[T comparable](last *T, next T, emit func(T)) {
	if next != *last {
		emit(next)
		*last = next
	}
}

// diffEmitMap is the map-keyed sibling of diffEmit. Go does not allow taking
// the address of a map value, so this variant performs the lookup and update
// directly against the map.
func diffEmitMap[K, V comparable](last map[K]V, key K, next V, emit func(V)) {
	if last[key] != next {
		emit(next)
		last[key] = next
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
//
//nolint:misspell // British spelling intentional
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
	t.lastGroupTitles = make(map[string]string)
	t.lastLightTitles = make(map[string]string)

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
			t.lastGroupTitles[group.ID] = title
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
			t.lastLightTitles[light.ID] = title
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

// updateMenuTitles updates the titles of existing menu items based on current state.
// MenuItem.SetTitle re-emits the systray menu layout over DBus, so we diff
// against the last emitted title per item and skip when unchanged.
func (t *TrayManager) updateMenuTitles(status *Status) {
	for _, group := range status.Groups {
		if item, exists := t.groupMenus[group.ID]; exists {
			diffEmitMap(t.lastGroupTitles, group.ID, formatMenuTitle(group.Name, group.On), item.SetTitle)
		}
	}

	for _, light := range status.Lights {
		if item, exists := t.lightMenus[light.ID]; exists {
			diffEmitMap(t.lastLightTitles, light.ID, formatMenuTitle(light.Name, light.On), item.SetTitle)
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

// UpdateIconAndTooltip updates the icon and tooltip based on full status.
// Both calls hit DBus via godbus' encoder on every invocation, so we diff
// against the last emitted values and skip when nothing has changed.
func (t *TrayManager) UpdateIconAndTooltip(status *Status) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var (
		nextKey     iconKey
		nextIcon    []byte
		nextTooltip string
	)

	if status.Total == 0 {
		nextKey = iconKeyUnknown
		nextIcon = iconUnknown
		nextTooltip = "Keylight Control - No lights"
	} else {
		if status.OnCount > 0 {
			nextKey = iconKeyEnabled
			nextIcon = iconEnabled
		} else {
			nextKey = iconKeyDisabled
			nextIcon = iconDisabled
		}

		var b strings.Builder
		b.WriteString("Keylight Control\n")
		if len(status.Groups) > 0 {
			b.WriteString("\nGroups\n")
			for _, group := range status.Groups {
				b.WriteString(formatMenuTitle(group.Name, group.On) + "\n")
			}
		}
		if len(status.Lights) > 0 {
			b.WriteString("\nLights\n")
			for _, light := range status.Lights {
				b.WriteString(formatMenuTitle(light.Name, light.On) + "\n")
			}
		}
		nextTooltip = b.String()
	}

	if nextKey != t.lastIconKey {
		systray.SetIcon(nextIcon)
		t.lastIconKey = nextKey
	}
	diffEmit(&t.lastTooltip, nextTooltip, systray.SetTooltip)
}
