package main

import (
	"fmt"
)

// --- Command Definitions ---

// Build is a top-level command
type Build struct {
	Docker *DockerCmd `targ:"subcommand"`
}

// Run is the method executed when Build command is called
// Description: Build the project
// Epilog: For more information, visit our docs.
func (b *Build) Run() {
	fmt.Println("Running Build")
}

// DockerCmd is a subcommand of Build, so it should NOT be a top-level command
type DockerCmd struct {
	Tag string `targ:"flag"`
}

// Build something
func (d *DockerCmd) Run() {
	fmt.Printf("Running Docker Build (tag=%s)\n", d.Tag)
}

// Deploy is a standalone top-level command
type Deploy struct {
	Env string `targ:"flag"`
}

func (d *Deploy) Run() {
	fmt.Printf("Running Deploy to %s\n", d.Env)
}
