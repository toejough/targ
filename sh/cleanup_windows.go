//go:build windows

package sh

import "os"

// killProcess kills the process on Windows.
func killProcess(p *os.Process) {
	_ = p.Kill()
}
