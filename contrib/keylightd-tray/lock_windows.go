//go:build !bindings && windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// acquireLock attempts to create a PID lockfile. If another instance is
// already running, it prints a message and exits. The lockfile is removed
// automatically when the process exits.
//
// On Windows, os.FindProcess always succeeds, so we attempt to open the
// process handle to check if the PID is still alive.
func acquireLock() func() {
	lockPath := filepath.Join(os.TempDir(), "keylightd-tray.pid")

	// Check for an existing lockfile
	if data, err := os.ReadFile(lockPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				// On Windows, FindProcess always succeeds. Use Signal(nil)
				// which calls OpenProcess — if the process exists it returns
				// nil, otherwise an error.
				if proc.Signal(nil) == nil {
					fmt.Fprintf(os.Stderr, "keylightd-tray is already running (pid %d)\n", pid)
					os.Exit(1)
				}
			}
		}
		// Stale lockfile — remove it
		os.Remove(lockPath)
	}

	// Write our PID
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lockfile %s: %v\n", lockPath, err)
		os.Exit(1)
	}

	return func() { os.Remove(lockPath) }
}
