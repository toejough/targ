package main

import (
	"fmt"
	"targs"
)

// --- The Manifest ---
// Define a single Root struct that contains the entire tree structure.
// This gives you a "Table of Contents" for your CLI.
type Root struct {
	// Top-level commands
	Build  *Build  `targs:"subcommand,desc=Build related commands"`
	Deploy *Deploy `targs:"subcommand,desc=Deployment commands"`

	// Global flags (optional)
	Verbose bool `targs:"flag,global"`
}

// --- Command Definitions ---

type Build struct {
	// Build subcommands
	Docker *BuildDocker `targs:"subcommand,desc=Build Docker image"`
	Linux  *BuildLinux  `targs:"subcommand,desc=Build Linux binary"`
}

// Building... This is the main build command.
func (b *Build) Run() { fmt.Println("Building...") }

type BuildDocker struct {
	Tag string `targs:"flag,desc=Docker image tag"`
}

// Build and tag a Docker image.
func (b *BuildDocker) Run() { fmt.Println("Building Docker") }

type BuildLinux struct{}

// Build the Linux binary.
func (b *BuildLinux) Run() { fmt.Println("Building Linux") }

type Deploy struct {
	Env string `targs:"flag,desc=Deployment environment"`
}

// Deploy the application to an environment.
func (d *Deploy) Run() { fmt.Println("Deploying...") }

func main() {
	// Run Targs with a single root; subcommands are optional.
	targs.Run(Root{})
}
