//go:build !windows

package sh

import (
	"os"
	"syscall"
)

// killProcess kills the process and its entire process group.
func killProcess(p *os.Process) {
	// Kill the entire process group (negative PID)
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
}
