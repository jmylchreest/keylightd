package commands

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jmylchreest/keylightd/pkg/client"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// NewAPIKeyCommand creates the apikey command group.
func NewAPIKeyCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "api-key",
		Short:   "Manage API keys for keylightd",
		Aliases: []string{"api"},
	}

	cmd.AddCommand(
		newAPIKeyListCommand(logger),
		newAPIKeyAddCommand(logger),
		newAPIKeyDeleteCommand(logger),
		newAPIKeySetEnabledCommand(logger),
	)

	return cmd
}

func obfuscateAPIKey(key string) string {
	if len(key) > 8 {
		return key[:4] + "..." + key[len(key)-4:]
	}
	return key
}

func newAPIKeyListCommand(logger *slog.Logger) *cobra.Command {
	var parseable bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all API keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			keys, err := apiClient.ListAPIKeys()
			if err != nil {
				return fmt.Errorf("failed to list API keys: %w", err)
			}

			if len(keys) == 0 {
				pterm.Info.Println("No API keys found.")
				return nil
			}

			if parseable {
				for _, keyMap := range keys {
					keyStr, _ := keyMap["key"].(string) // Full key for parseable output
					name, _ := keyMap["name"].(string)
					disabledBool, _ := keyMap["disabled"].(bool)
					enabledBool := !disabledBool // New: reflect enabled status

					createdAt, _ := keyMap["created_at"].(time.Time)
					expiresAt, _ := keyMap["expires_at"].(time.Time)
					lastUsedAt, _ := keyMap["last_used_at"].(time.Time)

					// Format time.Time objects for parseable output, handle zero times
					createdAtOutput := ""
					if !createdAt.IsZero() {
						createdAtOutput = createdAt.Format(time.RFC3339Nano)
					}
					expiresAtOutput := ""
					if !expiresAt.IsZero() {
						expiresAtOutput = expiresAt.Format(time.RFC3339Nano)
					}
					lastUsedAtOutput := ""
					if !lastUsedAt.IsZero() {
						lastUsedAtOutput = lastUsedAt.Format(time.RFC3339Nano)
					}

					fmt.Printf("name=%s key=%s created_at=%s expires_at=%s last_used_at=%s enabled=%t\n",
						strconv.Quote(name), strconv.Quote(keyStr), createdAtOutput, expiresAtOutput, lastUsedAtOutput, enabledBool)
				}
				return nil
			}

			table := pterm.TableData{{"Name", "Key (Partial)", "Created At", "Expires At", "Last Used", "Enabled"}}
			for _, keyMap := range keys {
				keyStr, _ := keyMap["key"].(string)
				name, _ := keyMap["name"].(string)
				disabledBool, _ := keyMap["disabled"].(bool)
				enabledBool := !disabledBool // New: reflect enabled status

				// Assert directly to time.Time as the client now ensures this type
				createdAt, _ := keyMap["created_at"].(time.Time)
				expiresAt, _ := keyMap["expires_at"].(time.Time)
				lastUsedAt, _ := keyMap["last_used_at"].(time.Time)

				partialKey := obfuscateAPIKey(keyStr)

				table = append(table, []string{
					name,
					partialKey,
					formatTimeForDisplay(createdAt),
					formatTimeForDisplay(expiresAt),
					formatTimeForDisplay(lastUsedAt),
					strconv.FormatBool(enabledBool),
				})
			}
			pterm.DefaultTable.WithHasHeader().WithData(table).Render()
			return nil
		},
	}
	cmd.Flags().BoolVarP(&parseable, "parseable", "p", false, "Output in parseable format")
	return cmd
}

func newAPIKeyAddCommand(logger *slog.Logger) *cobra.Command {
	var name string
	var expiresIn string // This will hold flag value and interactive input

	cmd := &cobra.Command{
		Use:   "add [name] [duration]",
		Short: "Add a new API key. Duration can be like 30d, 24h, 720h, or 0 for never.",
		Args:  cobra.MaximumNArgs(2), // Allow name and optional duration as arguments
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			// Get name: from arg 1, then flag, then prompt
			if len(args) > 0 {
				name = args[0]
			}
			// Flag value for 'name' is already bound to the 'name' variable by Cobra

			if name == "" {
				var err error
				name, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show("Enter a friendly name for the API key")
				if err != nil {
					return fmt.Errorf("failed to get API key name: %w", err)
				}
				if name == "" {
					return fmt.Errorf("API key name cannot be empty")
				}
			}

			// Get duration: from arg 2, then flag, then prompt
			// The 'expiresIn' variable is bound to the --expires-in flag.
			// If arg 2 is present, it overrides the flag value for duration.
			if len(args) > 1 {
				expiresIn = args[1]
			}

			if expiresIn == "" { // If not from arg2 or flag
				var err error
				expiresIn, err = pterm.DefaultInteractiveTextInput.
					WithMultiLine(false).
					WithDefaultText("Enter duration until key expires (e.g., 30d, 24h, 0 for never). Leave empty or 0 for no expiry.").
					Show()
				if err != nil {
					return fmt.Errorf("failed to get expiry duration: %w", err)
				}
			}

			var expiresInDuration time.Duration
			if expiresIn != "" && expiresIn != "0" { // "0" or empty means never expires
				var err error
				expiresInDuration, err = time.ParseDuration(expiresIn)
				if err != nil {
					return fmt.Errorf("invalid duration format \"%s\". Use formats like 300s, 1.5h, 24h, 30d, or 0 for never: %w", expiresIn, err)
				}
			}

			createdKey, err := apiClient.AddAPIKey(name, expiresInDuration.Seconds())
			if err != nil {
				return fmt.Errorf("failed to add API key: %w", err)
			}

			keyStr, _ := createdKey["key"].(string)
			keyName, _ := createdKey["name"].(string)
			expiresAtStr, _ := createdKey["expires_at"].(string)

			pterm.Success.Println("API Key created successfully!")
			pterm.Info.Println("  Name:    ", keyName)
			pterm.Warning.Println("  Key:     ", keyStr, "(Store this securely! It will not be shown again.)")
			if expiresAtStr != "" && expiresAtStr != "0001-01-01T00:00:00Z" {
				expiresAt, errParse := time.Parse(time.RFC3339, expiresAtStr)
				if errParse == nil {
					pterm.Info.Println("  Expires: ", expiresAt.Format(time.RFC1123))
				} else {
					pterm.Info.Println("  Expires: ", expiresAtStr, "(raw)") // Show raw if parsing failed
				}
			} else {
				pterm.Info.Println("  Expires: Never")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Friendly name for the API key (overridden by positional argument)")
	cmd.Flags().StringVar(&expiresIn, "expires-in", "", "Duration until key expires (e.g., 720h, 30d, 0 or empty for never). Overridden by positional argument.")
	return cmd
}

func newAPIKeyDeleteCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [key_string]",
		Short: "Delete an API key",
		Args:  cobra.MaximumNArgs(1), // Key string is optional
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			keyToDelete := ""
			if len(args) > 0 {
				keyToDelete = args[0]
			} else {
				// No key provided, fetch and let user select
				keys, err := apiClient.ListAPIKeys()
				if err != nil {
					return fmt.Errorf("failed to list API keys for selection: %w", err)
				}
				if len(keys) == 0 {
					pterm.Info.Println("No API keys found to delete.")
					return nil
				}

				options := []string{}
				keyMapForSelection := make(map[string]string) // map display string to actual key

				for _, apiKey := range keys {
					name, _ := apiKey["name"].(string)
					fullKey, _ := apiKey["key"].(string)
					displayString := fmt.Sprintf("%s (%s)", name, obfuscateAPIKey(fullKey))
					options = append(options, displayString)
					keyMapForSelection[displayString] = fullKey
				}

				selectedOption, err := pterm.DefaultInteractiveSelect.
					WithDefaultText("Select API key to delete").
					WithOptions(options).
					Show()
				if err != nil {
					return fmt.Errorf("API key selection failed: %w", err)
				}
				keyToDelete = keyMapForSelection[selectedOption]
			}

			if keyToDelete == "" {
				return fmt.Errorf("no API key specified or selected for deletion")
			}

			// Confirm before deleting
			confirm, _ := pterm.DefaultInteractiveConfirm.
				WithDefaultText(fmt.Sprintf("Are you sure you want to delete API key %s?", obfuscateAPIKey(keyToDelete))).
				WithDefaultValue(false). // Default to no
				Show()

			if !confirm {
				pterm.Info.Println("API key deletion cancelled.")
				return nil
			}

			if err := apiClient.DeleteAPIKey(keyToDelete); err != nil {
				return fmt.Errorf("failed to delete API key: %w", err)
			}

			pterm.Success.Printf("API Key '%s' deleted successfully.\\n", obfuscateAPIKey(keyToDelete))
			return nil
		},
	}
	return cmd
}

func newAPIKeySetEnabledCommand(logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-enabled [key_or_name] [true|false]",
		Short: "Set the enabled status of an API key (true for enabled, false for disabled).",
		Long:  "Set the enabled status of an API key. \nIf key_or_name is not provided, an interactive selection will be shown. \nIf the boolean status (true/false or enabled/disabled) is not provided, an interactive selection for enabled/disabled will be shown.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, ok := cmd.Context().Value(clientContextKey).(client.ClientInterface)
			if !ok {
				return fmt.Errorf("client not found in context")
			}

			var keyToUpdate string
			var desiredEnabledState bool // Represents the desired state for "Enabled"
			var statusArgProvided bool

			if len(args) > 0 {
				keyToUpdate = args[0]
			}

			if len(args) > 1 {
				statusStr := strings.ToLower(args[1])
				if statusStr == "true" || statusStr == "enabled" {
					desiredEnabledState = true
					statusArgProvided = true
				} else if statusStr == "false" || statusStr == "disabled" {
					desiredEnabledState = false
					statusArgProvided = true
				} else {
					return fmt.Errorf("invalid status argument: %s. Must be true, false, enabled, or disabled", args[1])
				}
			}

			if keyToUpdate == "" {
				keys, err := apiClient.ListAPIKeys()
				if err != nil {
					return fmt.Errorf("failed to list API keys for selection: %w", err)
				}
				if len(keys) == 0 {
					pterm.Info.Println("No API keys found.")
					return nil
				}
				options := []string{}
				keyMapForSelection := make(map[string]string)
				for _, apiKey := range keys {
					name, _ := apiKey["name"].(string)
					fullKey, _ := apiKey["key"].(string)
					disabledStatus, _ := apiKey["disabled"].(bool)
					enabledStatus := !disabledStatus
					displayString := fmt.Sprintf("%s (%s) - Enabled: %t", name, obfuscateAPIKey(fullKey), enabledStatus)
					options = append(options, displayString)
					keyMapForSelection[displayString] = fullKey
				}
				selectedOption, err := pterm.DefaultInteractiveSelect.WithOptions(options).WithDefaultText("Select API key to update").Show()
				if err != nil {
					return fmt.Errorf("API key selection failed: %w", err)
				}
				keyToUpdate = keyMapForSelection[selectedOption]
			}

			if !statusArgProvided {
				statusOptions := []string{"Enabled", "Disabled"}
				selectedStatus, err := pterm.DefaultInteractiveSelect.WithOptions(statusOptions).WithDefaultText("Set API key status to").Show()
				if err != nil {
					return fmt.Errorf("status selection failed: %w", err)
				}
				if selectedStatus == "Enabled" {
					desiredEnabledState = true
				} else { // "Disabled"
					desiredEnabledState = false
				}
			}

			// apiClient.SetAPIKeyDisabledStatus expects the *disabled* status.
			// So, we pass the inverse of desiredEnabledState.
			actualDisabledStateToSet := !desiredEnabledState
			updatedKey, err := apiClient.SetAPIKeyDisabledStatus(keyToUpdate, actualDisabledStateToSet)
			if err != nil {
				return fmt.Errorf("failed to set API key enabled status: %w", err)
			}

			updatedName, _ := updatedKey["name"].(string)
			// The returned 'updatedKey' map contains the 'disabled' field.
			// To show 'enabled' status, we invert it.
			returnedDisabledStatus, _ := updatedKey["disabled"].(bool)
			finalEnabledStatus := !returnedDisabledStatus

			pterm.Success.Printf("API key '%s' (%s) status set to: Enabled=%t\n", updatedName, obfuscateAPIKey(keyToUpdate), finalEnabledStatus)
			return nil
		},
	}
	return cmd
}

// formatTimeForDisplay helper for consistent time formatting.
// Handles zero time and RFC3339 parsing errors gracefully for display.
func formatTimeForDisplay(t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 { // Check for zero time or very early dates
		return "Never"
	}
	return t.Format(time.RFC1123) // "Mon, 02 Jan 2006 15:04:05 MST"
}
