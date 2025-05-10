package commands

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/spf13/cobra"
)

// newGroupCommand creates the group command
func newGroupCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage groups of lights",
	}

	cmd.AddCommand(
		newGroupCreateCommand(),
		newGroupGetCommand(),
		newGroupSetCommand(),
	)

	return cmd
}

// newGroupCreateCommand creates the group create command
func newGroupCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new group of lights",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLoggerFromCmd(cmd)
			name := args[0]
			c := client.New(logger, cmd.Flag("socket").Value.String())
			if err := c.CreateGroup(name); err != nil {
				return fmt.Errorf("failed to create group: %w", err)
			}

			fmt.Printf("Created group %s\n", name)
			return nil
		},
	}

	return cmd
}

// newGroupGetCommand creates the group get command
func newGroupGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get the state of all lights in a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLoggerFromCmd(cmd)
			name := args[0]
			c := client.New(logger, cmd.Flag("socket").Value.String())
			state, err := c.GetGroup(name)
			if err != nil {
				return fmt.Errorf("failed to get group state: %w", err)
			}

			fmt.Printf("%s: %v\n", name, state)
			return nil
		},
	}

	return cmd
}

// newGroupSetCommand creates the group set command
func newGroupSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name> <property> <value>",
		Short: "Set a property for all lights in a group",
		Long: `Set a property for all lights in a group.
Properties:
  brightness: 0-100
  warmth: 0-100
  power: on/off
  name: new group name`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLoggerFromCmd(cmd)
			name := args[0]
			property := args[1]
			valueStr := args[2]

			var value interface{}
			var err error

			switch property {
			case "brightness", "warmth":
				value, err = strconv.Atoi(valueStr)
				if err != nil {
					return fmt.Errorf("invalid value for %s: %w", property, err)
				}
				if v := value.(int); v < 0 || v > 100 {
					return fmt.Errorf("%s must be between 0 and 100", property)
				}
			case "power":
				switch valueStr {
				case "on":
					value = true
				case "off":
					value = false
				default:
					return fmt.Errorf("power must be 'on' or 'off'")
				}
			case "name":
				value = valueStr
			default:
				return fmt.Errorf("unknown property: %s", property)
			}

			c := client.New(logger, cmd.Flag("socket").Value.String())
			if err := c.SetGroupState(name, property, value); err != nil {
				return fmt.Errorf("failed to set group state: %w", err)
			}

			fmt.Printf("Set %s to %v\n", property, value)
			return nil
		},
	}

	return cmd
}
