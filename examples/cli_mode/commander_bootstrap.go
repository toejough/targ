
package main

import (
	"commander"
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
	roots := commander.DetectRootCommands(cmds...)
	
	// Run
	commander.Run(roots...)
}
