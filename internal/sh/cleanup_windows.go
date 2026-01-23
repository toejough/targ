//go:build windows

package internal

import "os"

func init() {
	// Inject OS-specific kill implementation.
	// This thin wrapper is the entry point for OS process killing.
	KillProcessFunc = func(p *os.Process) {
		_ = p.Kill()
	}
}
