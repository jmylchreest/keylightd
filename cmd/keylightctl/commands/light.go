package commands

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/jmylchreest/keylightd/pkg/keylight"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var clientContextKey = &struct{}{}

// NewLightCommand creates the light command
func NewLightCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "light",
		Short: "Manage individual lights",
	}

	cmd.AddCommand(
		newLightListCommand(),
		newLightGetCommand(),
		newLightSetCommand(logger),
	)

	return cmd
}

// newLightListCommand creates the light list command
func newLightListCommand() *cobra.Command {
	var parseable bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered lights",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			lights, err := c.GetLights()
			if err != nil {
				return fmt.Errorf("failed to get lights: %w", err)
			}

			if len(lights) == 0 {
				if parseable {
					return nil
				}
				pterm.Info.Println("No lights discovered")
				return nil
			}

			if parseable {
				for id, light := range lights {
					lightMap := light.(map[string]any)
					fmt.Println(LightParseable(id, lightMap))
				}
				return nil
			}

			// Create a table for each light
			for id, light := range lights {
				lightMap := light.(map[string]any)
				table := LightTableData(id, lightMap)
				pterm.DefaultTable.WithData(table).Render()
				pterm.Println() // Add a blank line between lights
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&parseable, "parseable", "p", false, "Output in parseable format (key=value)")
	return cmd
}

// newLightGetCommand creates the light get command
func newLightGetCommand() *cobra.Command {
	var parseable bool
	cmd := &cobra.Command{
		Use:   "get [id] [property]",
		Short: "Get information about a light",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			lights, err := c.GetLights()
			if err != nil {
				return fmt.Errorf("failed to get lights: %w", err)
			}

			var lightID string
			if len(args) > 0 {
				lightID = args[0]
			} else {
				// Sort light IDs alphabetically
				ids := make([]string, 0, len(lights))
				for id := range lights {
					ids = append(ids, id)
				}
				sort.Strings(ids)

				// Create options for dropdown
				options := make([]string, len(ids))
				for i, id := range ids {
					lightMap := lights[id].(map[string]any)
					options[i] = fmt.Sprintf("%s (%v)", id, lightMap["productname"])
				}

				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(options).
					Show("Select a light")
				if err != nil {
					return fmt.Errorf("failed to select light: %w", err)
				}

				// Extract ID from selected option
				lightID = strings.Split(selected, " (")[0]
			}

			// Normalize user-provided ID if it might be escaped
			lightID = keylight.UnescapeRFC6763Label(lightID)

			light, err := c.GetLight(lightID)
			if err != nil {
				return fmt.Errorf("failed to get light: %w", err)
			}

			// If a specific property was requested, only show that
			if len(args) > 1 {
				property := strings.ToLower(args[1])
				value, ok := light[property]
				if !ok {
					return fmt.Errorf("invalid property: %s", property)
				}
				if parseable {
					fmt.Printf("%s=%v\n", property, value)
				} else {
					fmt.Println(value)
				}
				return nil
			}

			// Show all properties
			if parseable {
				fmt.Println(LightParseable(lightID, light))
			} else {
				table := LightTableData(lightID, light)
				pterm.DefaultTable.WithData(table).Render()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&parseable, "parseable", "p", false, "Output in parseable format (key=value)")
	return cmd
}

// newLightSetCommand creates the light set command
func newLightSetCommand(_ *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [id] [property] [value]",
		Short: "Set a light property",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			lights, err := c.GetLights()
			if err != nil {
				return fmt.Errorf("failed to get lights: %w", err)
			}

			// Get light ID
			var lightID string
			if len(args) > 0 {
				lightID = args[0]
			} else {
				// Sort light IDs alphabetically
				ids := make([]string, 0, len(lights))
				for id := range lights {
					ids = append(ids, id)
				}
				sort.Strings(ids)

				// Create options for dropdown
				options := make([]string, len(ids))
				for i, id := range ids {
					lightMap := lights[id].(map[string]any)
					options[i] = fmt.Sprintf("%s (%v)", id, lightMap["productname"])
				}

				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(options).
					Show("Select a light")
				if err != nil {
					return fmt.Errorf("failed to select light: %w", err)
				}

				// Extract ID from selected option
				lightID = strings.Split(selected, " (")[0]
			}

			// Normalize user-provided ID if it might be escaped
			lightID = keylight.UnescapeRFC6763Label(lightID)

			// Get property
			var property string
			if len(args) > 1 {
				property = args[1]
				// Validate property
				switch strings.ToLower(property) {
				case "on", "brightness", "temperature":
					// Valid property
				default:
					return fmt.Errorf("invalid property: %s. Must be one of: on, brightness, temperature", property)
				}
			} else {
				// Show dropdown for property selection
				property, err = pterm.DefaultInteractiveSelect.
					WithOptions([]string{"On", "Brightness", "Temperature"}).
					Show("Select property to set")
				if err != nil {
					return fmt.Errorf("failed to select property: %w", err)
				}
			}

			// Convert display property name to lowercase for the API
			propertyLower := strings.ToLower(property)

			// Get value
			var value any
			switch propertyLower {
			case "on":
				if len(args) > 2 {
					value = args[2] == "true" || args[2] == "on"
				} else {
					selected, err := pterm.DefaultInteractiveSelect.
						WithOptions([]string{"On", "Off"}).
						Show("Select power state")
					if err != nil {
						return fmt.Errorf("failed to get power state: %w", err)
					}
					value = selected == "On"
				}
			case "brightness":
				if len(args) > 2 {
					brightness, err := strconv.Atoi(args[2])
					if err != nil {
						return fmt.Errorf("invalid brightness value: %w", err)
					}
					value = brightness
				} else {
					result, err := pterm.DefaultInteractiveTextInput.
						WithMultiLine(false).
						Show("Enter brightness (0-100)")
					if err != nil {
						return fmt.Errorf("failed to get brightness value: %w", err)
					}
					brightness, err := strconv.Atoi(result)
					if err != nil {
						return fmt.Errorf("invalid brightness value: %w", err)
					}
					value = brightness
				}
			case "temperature":
				if len(args) > 2 {
					temp, err := strconv.Atoi(args[2])
					if err != nil {
						return fmt.Errorf("invalid temperature value: %w", err)
					}
					// Clamp temperature to valid range
					if temp < 2900 {
						temp = 2900
					} else if temp > 7000 {
						temp = 7000
					}
					// Convert to mireds for display
					mireds := 1000000 / temp
					if mireds > 344 {
						mireds = 344
					} else if mireds < 143 {
						mireds = 143
					}
					pterm.Info.Printf("Setting temperature to %dK (%d mireds)\n", temp, mireds)
					value = temp
				} else {
					result, err := pterm.DefaultInteractiveTextInput.
						WithMultiLine(false).
						Show("Enter temperature (2900K-7000K, warm to cool)")
					if err != nil {
						return fmt.Errorf("failed to get temperature value: %w", err)
					}
					temp, err := strconv.Atoi(result)
					if err != nil {
						return fmt.Errorf("invalid temperature value: %w", err)
					}
					// Clamp temperature to valid range
					if temp < 2900 {
						temp = 2900
					} else if temp > 7000 {
						temp = 7000
					}
					// Convert to mireds for display
					mireds := 1000000 / temp
					if mireds > 344 {
						mireds = 344
					} else if mireds < 143 {
						mireds = 143
					}
					pterm.Info.Printf("Setting temperature to %dK (%d mireds)\n", temp, mireds)
					value = temp
				}
			}

			if err := c.SetLightState(lightID, propertyLower, value); err != nil {
				return fmt.Errorf("failed to set light state: %w", err)
			}

			pterm.Success.Println("Light state updated successfully")
			return nil
		},
	}
	return cmd
}
