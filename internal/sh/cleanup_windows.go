//go:build windows

package internal

import "os"

// PlatformKillProcess kills a process using the platform-specific method.
// On Windows, this uses the standard Process.Kill().
func PlatformKillProcess(p *os.Process) {
	_ = p.Kill()
}
