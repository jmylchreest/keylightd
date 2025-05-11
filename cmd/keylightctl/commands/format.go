package commands

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

// LightTableData returns the table data for a light, with bold ID and value
func LightTableData(id string, light map[string]interface{}) pterm.TableData {
	return pterm.TableData{
		[]string{pterm.Bold.Sprint("ID"), pterm.Bold.Sprint(id)},
		[]string{"Product", fmt.Sprintf("%v", light["productname"])},
		[]string{"Serial", fmt.Sprintf("%v", light["serialnumber"])},
		[]string{"Firmware", fmt.Sprintf("%v (build %v)", light["firmwareversion"], light["firmwarebuild"])},
		[]string{"On", fmt.Sprintf("%v", light["on"])},
		[]string{"Temperature", fmt.Sprintf("%v", light["temperature"])},
		[]string{"Brightness", fmt.Sprintf("%v", light["brightness"])},
		[]string{"IP", fmt.Sprintf("%v", light["ip"])},
		[]string{"Port", fmt.Sprintf("%v", light["port"])},
	}
}

// LightParseable returns the parseable key=value string for a light
func LightParseable(id string, light map[string]interface{}) string {
	return fmt.Sprintf(
		"id=\"%s\" productname=\"%v\" serialnumber=\"%v\" firmwareversion=\"%v\" firmwarebuild=%v on=%v brightness=%v temperature=%v ip=\"%v\" port=%v",
		id,
		light["productname"],
		light["serialnumber"],
		light["firmwareversion"],
		light["firmwarebuild"],
		light["on"],
		light["brightness"],
		light["temperature"],
		light["ip"],
		light["port"],
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
