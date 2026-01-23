//go:build !windows

package internal

import (
	"os"
	"syscall"
)

// PlatformKillProcess kills a process using the platform-specific method.
// On Unix, this kills the entire process group.
func PlatformKillProcess(p *os.Process) {
	// Kill the entire process group (negative PID)
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}
