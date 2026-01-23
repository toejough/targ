// Package sh provides utilities for running shell commands in build scripts.
package sh

import (
	internal "github.com/toejough/targ/internal/sh"
)

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
// Call this once at program startup to ensure Ctrl-C kills all spawned processes.
func EnableCleanup() {
	internal.EnableCleanup()
}
