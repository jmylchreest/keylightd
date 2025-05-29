package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigBaseDir(t *testing.T) {
	// Save original environment
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	tests := []struct {
		name           string
		xdgConfigHome  string
		expectedSuffix string
		description    string
	}{
		{
			name:           "system_service",
			xdgConfigHome:  "/etc/keylightd",
			expectedSuffix: "/etc/keylightd",
			description:    "System service should use /etc/keylightd directly",
		},
		{
			name:           "user_default",
			xdgConfigHome:  "",
			expectedSuffix: "/.config/keylightd",
			description:    "User with default XDG should use ~/.config/keylightd",
		},
		{
			name:           "user_custom_xdg",
			xdgConfigHome:  "/home/user/myconfigs",
			expectedSuffix: "/home/user/myconfigs/keylightd",
			description:    "User with custom XDG should append keylightd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			
			result := GetConfigBaseDir()
			
			if tt.name == "user_default" {
				// For default case, check it ends with the expected suffix
				if !filepath.IsAbs(result) || !endsWithSuffix(result, tt.expectedSuffix) {
					t.Errorf("GetConfigBaseDir() = %v, expected to end with %v", result, tt.expectedSuffix)
				}
			} else {
				// For explicit cases, check exact match
				if result != tt.expectedSuffix {
					t.Errorf("GetConfigBaseDir() = %v, expected %v", result, tt.expectedSuffix)
				}
			}
		})
	}
}

func TestGetConfigPath(t *testing.T) {
	// Save original environment
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)

	tests := []struct {
		name          string
		xdgConfigHome string
		filename      string
		expected      string
	}{
		{
			name:          "system_daemon_config",
			xdgConfigHome: "/etc/keylightd",
			filename:      "keylightd.yaml",
			expected:      "/etc/keylightd/keylightd.yaml",
		},
		{
			name:          "user_client_config",
			xdgConfigHome: "",
			filename:      "keylightctl.yaml",
			expected:      "", // Will be checked with suffix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			
			result := GetConfigPath(tt.filename)
			
			if tt.expected != "" {
				if result != tt.expected {
					t.Errorf("GetConfigPath(%v) = %v, expected %v", tt.filename, result, tt.expected)
				}
			} else {
				// For user default case, check it's a valid path
				expectedSuffix := "/.config/keylightd/" + tt.filename
				if !filepath.IsAbs(result) || !endsWithSuffix(result, expectedSuffix) {
					t.Errorf("GetConfigPath(%v) = %v, expected to end with %v", tt.filename, result, expectedSuffix)
				}
			}
		})
	}
}

// Helper function to check if a path ends with a specific suffix
func endsWithSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}