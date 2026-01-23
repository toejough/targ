// Package main provides the targ CLI tool entry point.
// This is a thin wrapper that calls the runner implementation.
package main

import (
	"os"

	"github.com/toejough/targ/internal/runner"
)

func main() {
	os.Exit(runner.Run())
}
