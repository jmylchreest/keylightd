//go:build !bindings && !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// acquireLock attempts to create a PID lockfile. If another instance is
// already running, it prints a message and exits. The lockfile is removed
// automatically when the process exits.
func acquireLock() func() {
	dir := os.TempDir()
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dir = xdg
	}
	lockPath := filepath.Join(dir, "keylightd-tray.pid")

	// Check for an existing lockfile
	if data, err := os.ReadFile(lockPath); err == nil { //nolint:gosec // G703: lockPath is from os.UserCacheDir, not user input
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Signal 0 checks if the process exists without actually signaling it
			if err := syscall.Kill(pid, 0); err == nil {
				fmt.Fprintf(os.Stderr, "keylightd-tray is already running (pid %d)\n", pid)
				os.Exit(1)
			}
		}
		// Stale lockfile — remove it
		_ = os.Remove(lockPath) //nolint:gosec // G703: lockPath is from trusted sources, not user input
	}

	// Write our PID
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil { //nolint:gosec // G703: lockPath is from trusted sources, not user input
		fmt.Fprintf(os.Stderr, "failed to create lockfile %s: %v\n", lockPath, err)
		os.Exit(1)
	}

	return func() { _ = os.Remove(lockPath) }
}
