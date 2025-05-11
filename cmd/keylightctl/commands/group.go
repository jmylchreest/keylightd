package commands

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// At the top, add:
// import "./light.go" or declare var clientContextKey = &struct{}{} if not already present
// In all command handlers, change:
// client, ok := cmd.Context().Value("client").(client.ClientInterface)
// to:
// client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)

// NewGroupCommand creates the group command
func NewGroupCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage light groups",
	}

	cmd.AddCommand(
		newGroupListCommand(logger),
		newGroupAddCommand(logger),
		newGroupDeleteCommand(logger),
		newGroupGetCommand(logger),
		newGroupSetCommand(logger),
		newGroupEditCommand(logger),
	)

	return cmd
}

// newGroupListCommand creates the group list command
func newGroupListCommand(logger *slog.Logger) *cobra.Command {
	var parseable bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all light groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			groups, err := client.GetGroups()
			if err != nil {
				return fmt.Errorf("failed to get groups: %w", err)
			}

			if parseable {
				for _, group := range groups {
					fmt.Printf("id=%q name=%q lights=%q\n",
						group["id"],
						group["name"],
						group["lights"])
				}
				return nil
			}

			table := pterm.TableData{
				{"Group ID", "Name", "Lights"},
			}

			for _, group := range groups {
				lights := group["lights"].([]interface{})
				lightIDs := make([]string, len(lights))
				for i, light := range lights {
					lightIDs[i] = light.(string)
				}

				table = append(table, []string{
					group["id"].(string),
					group["name"].(string),
					strings.Join(lightIDs, ", "),
				})
			}

			pterm.DefaultTable.WithHasHeader().WithData(table).Render()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&parseable, "parseable", "p", false, "Output in parseable format")
	return cmd
}

// newGroupAddCommand creates the group add command
func newGroupAddCommand(logger *slog.Logger) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new light group",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Get name from args if provided
			if len(args) > 0 {
				name = args[0]
			}

			// Prompt for name if not provided
			if name == "" {
				var err error
				name, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show("Enter group name")
				if err != nil {
					return fmt.Errorf("failed to get group name: %w", err)
				}
				if name == "" {
					return fmt.Errorf("group name cannot be empty")
				}
			}

			logger.Debug("Creating group", "name", name)
			if err := client.CreateGroup(name); err != nil {
				return fmt.Errorf("failed to create group: %w", err)
			}

			pterm.Success.Printf("Created group: %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the group")
	return cmd
}

// newGroupDeleteCommand creates the group delete command
func newGroupDeleteCommand(logger *slog.Logger) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a light group",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Use identifier from args if provided
			if len(args) > 0 {
				var err error
				name, err = resolveGroupIdentifier(client, args[0])
				if err != nil {
					return err
				}
			}

			// Prompt for name if not provided
			if name == "" {
				var err error
				name, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show("Enter group name or ID")
				if err != nil {
					return fmt.Errorf("failed to get group name: %w", err)
				}
				if name == "" {
					return fmt.Errorf("group name cannot be empty")
				}
				// Resolve the entered name/ID
				name, err = resolveGroupIdentifier(client, name)
				if err != nil {
					return err
				}
			}

			if err := client.DeleteGroup(name); err != nil {
				return fmt.Errorf("failed to delete group: %w", err)
			}

			pterm.Success.Printf("Deleted group: %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name or ID of the group")
	return cmd
}

// newGroupGetCommand creates the group get command
func newGroupGetCommand(logger *slog.Logger) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a light group",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Get groups for selection
			groups, err := client.GetGroups()
			if err != nil {
				return fmt.Errorf("failed to get groups: %w", err)
			}

			if len(groups) == 0 {
				return fmt.Errorf("no groups found")
			}

			// Use identifier from args if provided
			if len(args) > 0 {
				var err error
				name, err = resolveGroupIdentifier(client, args[0])
				if err != nil {
					return err
				}
			}

			// Prompt for group if not provided
			if name == "" {
				// Create options for dropdown
				options := make([]string, len(groups))
				for i, group := range groups {
					options[i] = fmt.Sprintf("%s (%s)", group["id"].(string), group["name"].(string))
				}

				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(options).
					Show("Select a group")
				if err != nil {
					return fmt.Errorf("failed to select group: %w", err)
				}

				// Extract ID from selected option
				name = strings.Split(selected, " (")[0]
			}

			group, err := client.GetGroup(name)
			if err != nil {
				return fmt.Errorf("failed to get group: %w", err)
			}

			lights := group["lights"].([]interface{})
			lightIDs := make([]string, len(lights))
			for i, light := range lights {
				lightIDs[i] = light.(string)
			}

			table := pterm.TableData{
				{"Group ID", "Name", "Lights"},
				{
					group["id"].(string),
					group["name"].(string),
					strings.Join(lightIDs, ", "),
				},
			}

			pterm.DefaultTable.WithHasHeader().WithData(table).Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name or ID of the group")
	return cmd
}

// newGroupSetCommand creates the group set command
func newGroupSetCommand(logger *slog.Logger) *cobra.Command {
	var name string
	var property string
	var value interface{}

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set properties for all lights in a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Get groups for selection
			groups, err := client.GetGroups()
			if err != nil {
				return fmt.Errorf("failed to get groups: %w", err)
			}

			if len(groups) == 0 {
				return fmt.Errorf("no groups found")
			}

			// Use identifier from args if provided
			if len(args) > 0 {
				var err error
				name, err = resolveGroupIdentifier(client, args[0])
				if err != nil {
					return err
				}
			}

			// Prompt for group if not provided
			if name == "" {
				// Create options for dropdown
				options := make([]string, len(groups))
				for i, group := range groups {
					options[i] = fmt.Sprintf("%s (%s)", group["id"].(string), group["name"].(string))
				}

				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(options).
					Show("Select a group")
				if err != nil {
					return fmt.Errorf("failed to select group: %w", err)
				}

				// Extract ID from selected option
				name = strings.Split(selected, " (")[0]
			}

			// Use property from args if provided
			if len(args) > 1 {
				property = args[1]
				// Validate property
				switch strings.ToLower(property) {
				case "on", "brightness", "temperature":
					// Valid property
				default:
					return fmt.Errorf("invalid property: %s. Must be one of: on, brightness, temperature", property)
				}
			}

			// Prompt for property if not provided
			if property == "" {
				var err error
				property, err = pterm.DefaultInteractiveSelect.WithOptions([]string{"on", "brightness", "temperature"}).Show("Select property to set")
				if err != nil {
					return fmt.Errorf("failed to select property: %w", err)
				}
			}

			// Use value from args if provided
			if len(args) > 2 {
				switch property {
				case "on":
					value = args[2] == "true" || args[2] == "on"
				case "brightness":
					brightness, err := strconv.Atoi(args[2])
					if err != nil {
						return fmt.Errorf("invalid brightness value: %w", err)
					}
					value = brightness
					// Clamp brightness to valid range (0-100)
					if value.(int) < 0 {
						value = 0
					} else if value.(int) > 100 {
						value = 100
					}
				case "temperature":
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
				}
			}

			// Prompt for value if not provided
			if value == nil {
				switch property {
				case "on":
					selected, err := pterm.DefaultInteractiveSelect.
						WithOptions([]string{"On", "Off"}).
						Show("Select power state")
					if err != nil {
						return fmt.Errorf("failed to get power state: %w", err)
					}
					value = selected == "On"

				case "brightness":
					brightness, err := pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show("Enter brightness (0-100)")
					if err != nil {
						return fmt.Errorf("failed to get brightness value: %w", err)
					}
					value, err = strconv.Atoi(brightness)
					if err != nil {
						return fmt.Errorf("invalid brightness value: %w", err)
					}
					// Clamp brightness to valid range (0-100)
					if value.(int) < 0 {
						value = 0
					} else if value.(int) > 100 {
						value = 100
					}

				case "temperature":
					temp, err := pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show("Enter temperature (2900-7000K)")
					if err != nil {
						return fmt.Errorf("failed to get temperature value: %w", err)
					}
					value, err = strconv.Atoi(temp)
					if err != nil {
						return fmt.Errorf("invalid temperature value: %w", err)
					}
					// Clamp temperature to valid range
					if value.(int) < 2900 {
						value = 2900
					} else if value.(int) > 7000 {
						value = 7000
					}
					// Convert to mireds for display
					mireds := 1000000 / value.(int)
					if mireds > 344 {
						mireds = 344
					} else if mireds < 143 {
						mireds = 143
					}
					pterm.Info.Printf("Setting temperature to %dK (%d mireds)\n", value.(int), mireds)
				}
			}

			if err := client.SetGroupState(name, property, value); err != nil {
				return fmt.Errorf("failed to set group state: %w", err)
			}

			pterm.Success.Printf("Updated group %s: %s = %v\n", name, property, value)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name or ID of the group")
	cmd.Flags().StringVar(&property, "property", "", "Property to set (on, brightness, temperature)")
	cmd.Flags().Var(newValueFlag(&value), "value", "Value to set")
	return cmd
}

// valueFlag implements the flag.Value interface for the value flag
type valueFlag struct {
	value *interface{}
}

func newValueFlag(value *interface{}) *valueFlag {
	return &valueFlag{value: value}
}

func (f *valueFlag) String() string {
	if f.value == nil {
		return ""
	}
	return fmt.Sprintf("%v", *f.value)
}

func (f *valueFlag) Set(s string) error {
	// Try to parse as integer first
	if i, err := strconv.Atoi(s); err == nil {
		*f.value = i
		return nil
	}

	// Try to parse as boolean
	if b, err := strconv.ParseBool(s); err == nil {
		*f.value = b
		return nil
	}

	// Use as string
	*f.value = s
	return nil
}

func (f *valueFlag) Type() string {
	return "value"
}

// newGroupEditCommand creates the group edit command
func newGroupEditCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [groupid] [lightid...]",
		Short: "Edit the lights in a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Get all groups
			groups, err := client.GetGroups()
			if err != nil {
				return fmt.Errorf("failed to get groups: %w", err)
			}

			// Get all lights
			lights, err := client.GetLights()
			if err != nil {
				return fmt.Errorf("failed to get lights: %w", err)
			}

			// Get group ID
			var groupID string
			if len(args) > 0 {
				var err error
				groupID, err = resolveGroupIdentifier(client, args[0])
				if err != nil {
					return err
				}
			} else {
				// Create options for group selection
				options := make([]string, len(groups))
				for i, group := range groups {
					options[i] = fmt.Sprintf("%s (%s)", group["id"], group["name"])
				}

				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(options).
					Show("Select a group")
				if err != nil {
					return fmt.Errorf("failed to select group: %w", err)
				}

				// Extract ID from selected option
				groupID = strings.Split(selected, " (")[0]
			}

			// Get current group
			group, err := client.GetGroup(groupID)
			if err != nil {
				return fmt.Errorf("failed to get group: %w", err)
			}

			// Get current lights in group
			currentLights := make(map[string]bool)
			if lights, ok := group["lights"].([]interface{}); ok {
				for _, light := range lights {
					currentLights[light.(string)] = true
				}
			}

			// If light IDs are provided as arguments, use those
			if len(args) > 1 {
				newLightIDs := args[1:]
				if err := client.SetGroupLights(groupID, newLightIDs); err != nil {
					return fmt.Errorf("failed to set group lights: %w", err)
				}
				pterm.Success.Printf("Updated group %s with lights: %s\n", groupID, strings.Join(newLightIDs, ", "))
				return nil
			}

			// Create options for light selection
			options := make([]string, 0, len(lights))
			for id, light := range lights {
				lightMap := light.(map[string]interface{})
				selected := ""
				if currentLights[id] {
					selected = " ✓"
				}
				options = append(options, fmt.Sprintf("%s (%v)%s", id, lightMap["productname"], selected))
			}

			// Show multi-select for lights
			selected, err := pterm.DefaultInteractiveMultiselect.
				WithOptions(options).
				Show("Select lights for the group")
			if err != nil {
				return fmt.Errorf("failed to select lights: %w", err)
			}

			// Extract IDs from selected options
			newLightIDs := make([]string, len(selected))
			for i, option := range selected {
				newLightIDs[i] = strings.Split(option, " (")[0]
			}

			// Update group lights
			if err := client.SetGroupLights(groupID, newLightIDs); err != nil {
				return fmt.Errorf("failed to set group lights: %w", err)
			}

			pterm.Success.Printf("Updated group %s with lights: %s\n", groupID, strings.Join(newLightIDs, ", "))
			return nil
		},
	}

	return cmd
}

// resolveGroupIdentifier takes either a group name or ID and returns the group ID
func resolveGroupIdentifier(client client.ClientInterface, identifier string) (string, error) {
	groups, err := client.GetGroups()
	if err != nil {
		return "", fmt.Errorf("failed to get groups: %w", err)
	}

	// First try to find by name
	for _, group := range groups {
		if group["name"].(string) == identifier {
			return group["id"].(string), nil
		}
	}

	// If not found by name, check if it's a valid ID
	for _, group := range groups {
		if group["id"].(string) == identifier {
			return identifier, nil
		}
	}

	return "", fmt.Errorf("no group found with name or ID: %s", identifier)
}
