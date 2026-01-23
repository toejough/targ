//go:build !windows

package internal

import (
	"os"
	"syscall"
)

//nolint:gochecknoinits // init required to inject platform-specific process kill implementation
func init() {
	// Inject OS-specific kill implementation.
	// This thin wrapper is the entry point for OS process killing.
	KillProcessFunc = func(p *os.Process) {
		// Kill the entire process group (negative PID)
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	}
}
