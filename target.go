package targ

import (
	"github.com/toejough/targ/internal/core"
)

// Exported constants.
const (
	// DepModeParallel executes all dependencies concurrently.
	DepModeParallel = core.DepModeParallel
	// DepModeSerial executes dependencies one at a time in order.
	DepModeSerial = core.DepModeSerial
)

type DepMode = core.DepMode

type Target = core.Target

// Targ creates a Target from a function or shell command string.
//
// Function targets:
//
//	var build = targ.Targ(Build)
//
// Shell command targets (run in user's shell):
//
//	var lint = targ.Targ("golangci-lint run ./...")
func Targ(fn any) *Target {
	return core.Targ(fn)
}
