package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/pterm/pterm"
)

// WaybarOutput represents the JSON format expected by waybar custom modules
type WaybarOutput struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

// LightJSON represents a light in JSON format
type LightJSON struct {
	ID              string `json:"id"`
	ProductName     string `json:"product_name"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
	FirmwareBuild   int    `json:"firmware_build"`
	On              bool   `json:"on"`
	Brightness      int    `json:"brightness"`
	Temperature     int    `json:"temperature"`
	TemperatureK    int    `json:"temperature_kelvin"`
	IP              string `json:"ip"`
	Port            int    `json:"port"`
	LastSeen        int64  `json:"last_seen"`
}

// GroupJSON represents a group in JSON format
type GroupJSON struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Lights []string `json:"lights"`
}

// StatusJSON represents the overall status in JSON format
type StatusJSON struct {
	Lights   []LightJSON `json:"lights"`
	Groups   []GroupJSON `json:"groups"`
	OnCount  int         `json:"on_count"`
	OffCount int         `json:"off_count"`
	Total    int         `json:"total"`
}

// LightToJSON converts a light map to LightJSON struct
func LightToJSON(id string, light map[string]any) LightJSON {
	id = keylight.UnescapeRFC6763Label(id)

	tempDevice := 0
	if v, ok := light["temperature"].(int); ok {
		tempDevice = v
	}
	tempKelvin := keylight.ConvertDeviceToTemperature(tempDevice)

	firmwareBuild := 0
	if v, ok := light["firmwarebuild"].(int); ok {
		firmwareBuild = v
	}

	port := 0
	if v, ok := light["port"].(int); ok {
		port = v
	}

	brightness := 0
	if v, ok := light["brightness"].(int); ok {
		brightness = v
	}

	on := false
	if v, ok := light["on"].(bool); ok {
		on = v
	}

	lastSeen := int64(0)
	if t, ok := light["lastseen"].(time.Time); ok && !t.IsZero() {
		lastSeen = t.Unix()
	}

	return LightJSON{
		ID:              id,
		ProductName:     fmt.Sprintf("%v", light["productname"]),
		SerialNumber:    fmt.Sprintf("%v", light["serialnumber"]),
		FirmwareVersion: fmt.Sprintf("%v", light["firmwareversion"]),
		FirmwareBuild:   firmwareBuild,
		On:              on,
		Brightness:      brightness,
		Temperature:     tempDevice,
		TemperatureK:    tempKelvin,
		IP:              fmt.Sprintf("%v", light["ip"]),
		Port:            port,
		LastSeen:        lastSeen,
	}
}

// GroupToJSON converts a group map to GroupJSON struct
func GroupToJSON(group map[string]any) GroupJSON {
	id := group["id"].(string)
	name := group["name"].(string)

	var lightIDs []string
	if lights, ok := group["lights"].([]any); ok {
		lightIDs = make([]string, len(lights))
		for i, light := range lights {
			lightIDs[i] = light.(string)
		}
	}

	return GroupJSON{
		ID:     id,
		Name:   name,
		Lights: lightIDs,
	}
}

// FormatWaybarOutput creates waybar-compatible JSON output
func FormatWaybarOutput(lights map[string]any) string {
	onCount := 0
	offCount := 0
	totalBrightness := 0

	var tooltipLines []string

	for id, light := range lights {
		lightMap := light.(map[string]any)
		name := keylight.UnescapeRFC6763Label(id)

		on := false
		if v, ok := lightMap["on"].(bool); ok {
			on = v
		}

		brightness := 0
		if v, ok := lightMap["brightness"].(int); ok {
			brightness = v
		}

		tempDevice := 0
		if v, ok := lightMap["temperature"].(int); ok {
			tempDevice = v
		}
		tempKelvin := keylight.ConvertDeviceToTemperature(tempDevice)

		if on {
			onCount++
			totalBrightness += brightness
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s: %d%% @ %dK", name, brightness, tempKelvin))
		} else {
			offCount++
			tooltipLines = append(tooltipLines, fmt.Sprintf("%s: off", name))
		}
	}

	total := onCount + offCount

	// Determine class based on state
	class := "off"
	if onCount > 0 {
		class = "on"
	}

	// Calculate average brightness percentage
	avgBrightness := 0
	if onCount > 0 {
		avgBrightness = totalBrightness / onCount
	}

	// Format text
	text := fmt.Sprintf("%d/%d", onCount, total)

	// Format tooltip
	tooltip := fmt.Sprintf("Lights: %d on, %d off\n%s", onCount, offCount, strings.Join(tooltipLines, "\n"))

	output := WaybarOutput{
		Text:       text,
		Tooltip:    tooltip,
		Class:      class,
		Percentage: avgBrightness,
	}

	jsonBytes, _ := json.Marshal(output)
	return string(jsonBytes)
}

// LightTableData returns the table data for a light, with bold ID and value
func LightTableData(id string, light map[string]any) pterm.TableData {
	id = keylight.UnescapeRFC6763Label(id)
	tempDevice := 0
	if v, ok := light["temperature"].(int); ok {
		tempDevice = v
	}
	tempKelvin := keylight.ConvertDeviceToTemperature(tempDevice)
	return pterm.TableData{
		[]string{pterm.Bold.Sprint("ID"), pterm.Bold.Sprint(id)},
		[]string{"Product", fmt.Sprintf("%v", light["productname"])},
		[]string{"Serial", fmt.Sprintf("%v", light["serialnumber"])},
		[]string{"Firmware", fmt.Sprintf("%v (build %v)", light["firmwareversion"], light["firmwarebuild"])},
		[]string{"On", fmt.Sprintf("%v", light["on"])},
		[]string{"Temperature", fmt.Sprintf("%v (%dK)", tempDevice, tempKelvin)},
		[]string{"Brightness", fmt.Sprintf("%v", light["brightness"])},
		[]string{"IP", fmt.Sprintf("%v", light["ip"])},
		[]string{"Port", fmt.Sprintf("%v", light["port"])},
		[]string{"Last Seen", formatLastSeen(light["lastseen"])},
	}
}

// formatLastSeen formats the LastSeen time for display
func formatLastSeen(lastSeen any) string {
	if t, ok := lastSeen.(time.Time); ok && !t.IsZero() {
		// Format the time in a human-readable format, e.g., RFC1123Z
		return t.Format(time.RFC1123Z)
	}
	return "N/A"
}

// LightParseable returns the parseable key=value string for a light
func LightParseable(id string, light map[string]any) string {
	id = keylight.UnescapeRFC6763Label(id)
	lastSeenUnix := "0"
	if t, ok := light["lastseen"].(time.Time); ok && !t.IsZero() {
		lastSeenUnix = fmt.Sprintf("%d", t.Unix())
	}
	tempDevice := 0
	if v, ok := light["temperature"].(int); ok {
		tempDevice = v
	}
	tempKelvin := keylight.ConvertDeviceToTemperature(tempDevice)
	return fmt.Sprintf(
		"id=\"%s\" productname=\"%v\" serialnumber=\"%v\" firmwareversion=\"%v\" firmwarebuild=%v on=%v brightness=%v temperature=%v temperature_kelvin=%v ip=\"%v\" port=%v lastseen=%s",
		id,
		light["productname"],
		light["serialnumber"],
		light["firmwareversion"],
		light["firmwarebuild"],
		light["on"],
		light["brightness"],
		light["temperature"],
		tempKelvin,
		light["ip"],
		light["port"],
		lastSeenUnix,
	)
}

// GroupParseable returns the parseable string for a group (id, name, lights as comma-separated)
func GroupParseable(group map[string]any) string {
	id := group["id"].(string)
	name := group["name"].(string)
	lights := group["lights"].([]any)
	lightIDs := make([]string, len(lights))
	for i, light := range lights {
		lightIDs[i] = light.(string)
	}
	return fmt.Sprintf("id=\"%s\" name=\"%s\" lights=\"%s\"", id, name, strings.Join(lightIDs, ","))
}

// PrintPromptResult prints a colored, bold header, an optional warning banner (bold, red, no prefix), and a space-aligned key/value list.
// status: "success", "error", or "warn"
// title: e.g., "API Key Created"
// warning: optional warning message (empty if not needed)
// fields: slice of [key, value] pairs
func PrintPromptResult(status, title, warning string, fields [][2]string) {
	boldTitle := pterm.Style{pterm.Bold}.Sprint(title)
	// Print header
	switch status {
	case "success":
		pterm.Success.Println(boldTitle)
	case "error":
		pterm.Error.Println(boldTitle)
	case "warn":
		pterm.Warning.Println(boldTitle)
	default:
		pterm.Info.Println(boldTitle)
	}

	// Print warning banner if present (bold, red, no prefix)
	if warning != "" {
		banner := pterm.Style{pterm.FgRed, pterm.Bold}.Sprint(warning)
		fmt.Println(banner)
	}

	// Calculate max key width for alignment
	maxKeyLen := 0
	for _, kv := range fields {
		if l := len(kv[0]); l > maxKeyLen {
			maxKeyLen = l
		}
	}
	// Print key/value pairs with alignment
	for _, kv := range fields {
		fmt.Printf("%-*s  %s\n", maxKeyLen, kv[0], kv[1])
	}
}
