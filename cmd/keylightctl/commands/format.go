package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/pterm/pterm"
)

// LightTableData returns the table data for a light, with bold ID and value
func LightTableData(id string, light map[string]interface{}) pterm.TableData {
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
func formatLastSeen(lastSeen interface{}) string {
	if t, ok := lastSeen.(time.Time); ok && !t.IsZero() {
		// Format the time in a human-readable format, e.g., RFC1123Z
		return t.Format(time.RFC1123Z)
	}
	return "N/A"
}

// LightParseable returns the parseable key=value string for a light
func LightParseable(id string, light map[string]interface{}) string {
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
func GroupParseable(group map[string]interface{}) string {
	id := group["id"].(string)
	name := group["name"].(string)
	lights := group["lights"].([]interface{})
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
