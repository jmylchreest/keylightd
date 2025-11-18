package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	version       string
	commit        string
	buildDate     string
	client        client.ClientInterface
	logger        *slog.Logger
	tray          *TrayManager
	customCSSPath string
}

// SetTrayManager sets the tray manager reference
func (a *App) SetTrayManager(tray *TrayManager) {
	a.tray = tray
}

// SetCustomCSSPath sets a custom path for the CSS file
func (a *App) SetCustomCSSPath(path string) {
	a.customCSSPath = path
}

// ShowWindow shows the main window
func (a *App) ShowWindow() {
	runtime.WindowShow(a.ctx)
	if a.tray != nil {
		a.tray.SetWindowShown(true)
	}
}

// HideWindow hides the main window
func (a *App) HideWindow() {
	runtime.WindowHide(a.ctx)
	if a.tray != nil {
		a.tray.SetWindowShown(false)
	}
}

// Quit exits the application
func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

// Settings represents the connection settings
type Settings struct {
	ConnectionType string `json:"connectionType"`
	SocketPath     string `json:"socketPath"`
	APIUrl         string `json:"apiUrl"`
	APIKey         string `json:"apiKey"`
}

// Light represents a light for the frontend
type Light struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	On           bool   `json:"on"`
	Brightness   int    `json:"brightness"`
	Temperature  int    `json:"temperature"`
	ProductName  string `json:"productName"`
	SerialNumber string `json:"serialNumber"`
}

// Group represents a group for the frontend
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	LightIDs    []string `json:"lightIds"`
	On          bool     `json:"on"`
	Brightness  int      `json:"brightness"`
	Temperature int      `json:"temperature"`
}

// Status represents the overall status
type Status struct {
	Lights   []Light `json:"lights"`
	Groups   []Group `json:"groups"`
	OnCount  int     `json:"onCount"`
	OffCount int     `json:"offCount"`
	Total    int     `json:"total"`
}

// NewApp creates a new App application struct
func NewApp(version, commit, buildDate string) *App {
	return &App{
		version:   version,
		commit:    commit,
		buildDate: buildDate,
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Set up logging
	a.logger = utils.SetupLogger("info", "text")

	// Get socket path
	socket := config.GetRuntimeSocketPath()

	// Create client
	a.client = client.New(a.logger, socket)

	// Start watching custom.css for changes
	go a.watchCustomCSS()
}

// SaveSettings saves the connection settings and reconnects the client
func (a *App) SaveSettings(settings Settings) error {
	if settings.ConnectionType == "http" {
		// Validate HTTP settings
		if settings.APIUrl == "" {
			return fmt.Errorf("API URL is required for HTTP connection")
		}
		if settings.APIKey == "" {
			return fmt.Errorf("API key is required for HTTP connection")
		}

		// Create HTTP client
		a.client = client.NewHTTP(a.logger, settings.APIUrl, settings.APIKey)
	} else {
		// Use provided socket path or default
		socketPath := settings.SocketPath
		if socketPath == "" {
			socketPath = config.GetRuntimeSocketPath()
		}

		// Create socket client
		a.client = client.New(a.logger, socketPath)
	}

	return nil
}

// GetSettings returns the current connection settings
func (a *App) GetSettings() Settings {
	return Settings{
		ConnectionType: "socket",
		SocketPath:     config.GetRuntimeSocketPath(),
		APIUrl:         "",
		APIKey:         "",
	}
}

// getConfigDir returns the config directory for keylightd-tray
func (a *App) getConfigDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configDir, "keylightd", "keylightd-tray")
}

// getCustomCSSPath returns the path to the custom CSS file
func (a *App) getCustomCSSPath() string {
	if a.customCSSPath != "" {
		return a.customCSSPath
	}
	return filepath.Join(a.getConfigDir(), "custom.css")
}

// GetCustomCSS returns the custom CSS content from the config directory
func (a *App) GetCustomCSS() string {
	cssPath := a.getCustomCSSPath()
	content, err := os.ReadFile(cssPath)
	if err != nil {
		return "" // Return empty if file doesn't exist
	}
	return string(content)
}

// watchCustomCSS watches the custom.css file for changes and notifies the frontend
func (a *App) watchCustomCSS() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer func() {
		_ = watcher.Close()
	}()

	// Get CSS path and watch its directory
	cssPath := a.getCustomCSSPath()
	cssDir := filepath.Dir(cssPath)
	if cssDir == "" {
		return
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		return
	}

	err = watcher.Add(cssDir)
	if err != nil {
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) == "custom.css" {
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
					// Emit event to frontend to reload CSS
					runtime.EventsEmit(a.ctx, "reload-custom-css")
				}
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	// Cleanup if needed
}

// GetVersion returns the app version
func (a *App) GetVersion() string {
	return fmt.Sprintf("%s, commit: %s, date: %s", a.version, a.commit, a.buildDate)
}

// GetStatus returns the current status of all lights and groups
func (a *App) GetStatus() (*Status, error) {
	lights, err := a.client.GetLights()
	if err != nil {
		return nil, fmt.Errorf("failed to get lights: %w", err)
	}

	groups, err := a.client.GetGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}

	status := &Status{
		Lights: make([]Light, 0),
		Groups: make([]Group, 0),
	}

	// Process lights
	lightMap := make(map[string]Light)
	for id, lightData := range lights {
		lightInfo := lightData.(map[string]any)
		light := a.convertLight(id, lightInfo)
		lightMap[id] = light
		status.Lights = append(status.Lights, light)

		if light.On {
			status.OnCount++
		} else {
			status.OffCount++
		}
	}
	status.Total = len(status.Lights)

	// Sort lights by name (case-insensitive)
	sort.Slice(status.Lights, func(i, j int) bool {
		return strings.ToLower(status.Lights[i].Name) < strings.ToLower(status.Lights[j].Name)
	})

	// Process groups
	for _, groupData := range groups {
		group := a.convertGroup(groupData, lightMap)
		status.Groups = append(status.Groups, group)
	}

	// Sort groups by name (case-insensitive)
	sort.Slice(status.Groups, func(i, j int) bool {
		return strings.ToLower(status.Groups[i].Name) < strings.ToLower(status.Groups[j].Name)
	})

	// Update tray icon based on light status
	if a.tray != nil {
		a.tray.UpdateIconFromStatus(status.OnCount, status.Total)
	}

	return status, nil
}

// GetLights returns all discovered lights
func (a *App) GetLights() ([]Light, error) {
	lights, err := a.client.GetLights()
	if err != nil {
		return nil, fmt.Errorf("failed to get lights: %w", err)
	}

	result := make([]Light, 0, len(lights))
	for id, lightData := range lights {
		lightInfo := lightData.(map[string]any)
		result = append(result, a.convertLight(id, lightInfo))
	}

	// Sort by name (case-insensitive)
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// GetGroups returns all groups
func (a *App) GetGroups() ([]Group, error) {
	lights, err := a.client.GetLights()
	if err != nil {
		return nil, fmt.Errorf("failed to get lights: %w", err)
	}

	groups, err := a.client.GetGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}

	// Build light map
	lightMap := make(map[string]Light)
	for id, lightData := range lights {
		lightInfo := lightData.(map[string]any)
		lightMap[id] = a.convertLight(id, lightInfo)
	}

	result := make([]Group, 0, len(groups))
	for _, groupData := range groups {
		result = append(result, a.convertGroup(groupData, lightMap))
	}

	// Sort by name (case-insensitive)
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// SetLightState sets a property on a light
func (a *App) SetLightState(id string, property string, value any) error {
	return a.client.SetLightState(id, property, value)
}

// SetGroupState sets a property on all lights in a group
func (a *App) SetGroupState(id string, property string, value any) error {
	return a.client.SetGroupState(id, property, value)
}

// CreateGroup creates a new group
func (a *App) CreateGroup(name string) error {
	return a.client.CreateGroup(name)
}

// DeleteGroup deletes a group
func (a *App) DeleteGroup(id string) error {
	return a.client.DeleteGroup(id)
}

// SetGroupLights sets the lights in a group
func (a *App) SetGroupLights(groupID string, lightIDs []string) error {
	return a.client.SetGroupLights(groupID, lightIDs)
}

// convertLight converts the API light data to our Light struct
func (a *App) convertLight(id string, data map[string]any) Light {
	// Use the display name from the light data, fall back to unescaped ID
	name := ""
	if v, ok := data["name"].(string); ok && v != "" {
		name = v
	} else {
		name = keylight.UnescapeRFC6763Label(id)
	}

	on := false
	if v, ok := data["on"].(bool); ok {
		on = v
	}

	brightness := 0
	if v, ok := data["brightness"].(float64); ok {
		brightness = int(v)
	} else if v, ok := data["brightness"].(int); ok {
		brightness = v
	}

	tempDevice := 0
	if v, ok := data["temperature"].(float64); ok {
		tempDevice = int(v)
	} else if v, ok := data["temperature"].(int); ok {
		tempDevice = v
	}
	tempKelvin := keylight.ConvertDeviceToTemperature(tempDevice)

	productName := ""
	if v, ok := data["productname"].(string); ok {
		productName = v
	}

	serialNumber := ""
	if v, ok := data["serialnumber"].(string); ok {
		serialNumber = v
	}

	return Light{
		ID:           id,
		Name:         name,
		On:           on,
		Brightness:   brightness,
		Temperature:  tempKelvin,
		ProductName:  productName,
		SerialNumber: serialNumber,
	}
}

// convertGroup converts the API group data to our Group struct
func (a *App) convertGroup(data map[string]any, lightMap map[string]Light) Group {
	id := data["id"].(string)
	name := data["name"].(string)

	var lightIDs []string
	if lights, ok := data["lights"].([]any); ok {
		lightIDs = make([]string, len(lights))
		for i, l := range lights {
			lightIDs[i] = l.(string)
		}
	}

	// Use first light's values for display (same as GNOME extension)
	// Group is "on" if any light is on
	on := false
	brightness := 50    // Default
	temperature := 4500 // Default

	for _, lightID := range lightIDs {
		if light, exists := lightMap[lightID]; exists {
			if light.On {
				on = true
			}
		}
	}

	// Get first light's values for sliders
	if len(lightIDs) > 0 {
		if firstLight, exists := lightMap[lightIDs[0]]; exists {
			brightness = firstLight.Brightness
			temperature = firstLight.Temperature
		}
	}

	return Group{
		ID:          id,
		Name:        name,
		LightIDs:    lightIDs,
		On:          on,
		Brightness:  brightness,
		Temperature: temperature,
	}
}

// Ping checks if the backend connection is working
func (a *App) Ping() error {
	_, err := a.client.GetLights()
	return err
}

// GetRefreshInterval returns the suggested refresh interval in milliseconds
func (a *App) GetRefreshInterval() int {
	return 1000 // 1 second
}

// SetWindowHeight adjusts window height to fit content, respecting max height
func (a *App) SetWindowHeight(contentHeight int, maxHeight int) {
	// Add padding for header and footer
	totalHeight := contentHeight + 100

	// Clamp to max height
	if maxHeight > 0 && totalHeight > maxHeight {
		totalHeight = maxHeight
	}

	// Minimum height
	if totalHeight < 200 {
		totalHeight = 200
	}

	runtime.WindowSetSize(a.ctx, 380, totalHeight)
}

// GetWindowSize returns current window dimensions
func (a *App) GetWindowSize() map[string]int {
	w, h := runtime.WindowGetSize(a.ctx)
	return map[string]int{"width": w, "height": h}
}

// FormatLastSeen formats a timestamp for display
func (a *App) FormatLastSeen(timestamp int64) string {
	if timestamp == 0 {
		return "Never"
	}
	t := time.Unix(timestamp, 0)
	return t.Format("15:04:05")
}
