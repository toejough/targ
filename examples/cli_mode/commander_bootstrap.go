package main

import (
	"targs"
)

func main() {
	// Auto-detected commands
	cmds := []interface{}{
		&Build{},
		&Clean{},
		&Deploy{},
		&StagingCmd{},
		&ProdCmd{},
	}

	// Filter roots
	roots := targs.DetectRootCommands(cmds...)

	// Run
	targs.Run(roots...)
}
