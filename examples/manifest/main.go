package main

import (
	"commander"
	"fmt"
)

// --- The Manifest ---
// Define a single Root struct that contains the entire tree structure.
// This gives you a "Table of Contents" for your CLI.
type Root struct {
	// Top-level commands
	Build  *Build  `commander:"subcommand,desc=Build related commands"`
	Deploy *Deploy `commander:"subcommand,desc=Deployment commands"`

	// Global flags (optional)
	Verbose bool `commander:"flag,global"`
}

// --- Command Definitions ---

type Build struct {
	// Build subcommands
	Docker *BuildDocker `commander:"subcommand,desc=Build Docker image"`
	Linux  *BuildLinux  `commander:"subcommand,desc=Build Linux binary"`
}

// Building... This is the main build command.
func (b *Build) Run() { fmt.Println("Building...") }

type BuildDocker struct {
	Tag string `commander:"flag,desc=Docker image tag"`
}

// Build and tag a Docker image.
func (b *BuildDocker) Run() { fmt.Println("Building Docker") }

type BuildLinux struct{}

// Build the Linux binary.
func (b *BuildLinux) Run() { fmt.Println("Building Linux") }

type Deploy struct {
	Env string `commander:"flag,desc=Deployment environment"`
}

// Deploy the application to an environment.
func (d *Deploy) Run() { fmt.Println("Deploying...") }

func main() {
	// Run Commander with a single root; subcommands are optional.
	commander.Run(Root{})
}
