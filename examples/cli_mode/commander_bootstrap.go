package main

import (
	"github.com/toejough/targ"
)

func main() {
	// Auto-detected commands
	cmds := []any{
		&Build{},
		&Clean{},
		&Deploy{},
		&StagingCmd{},
		&ProdCmd{},
	}

	// Filter roots
	roots := targ.DetectRootCommands(cmds...)

	// Run
	targ.Run(roots...)
}
