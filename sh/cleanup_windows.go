//go:build windows

package sh

import "os"

func init() {
	// Inject OS-specific kill implementation.
	// This thin wrapper is the entry point for OS process killing.
	killProcessFunc = func(p *os.Process) {
		_ = p.Kill()
	}
}
