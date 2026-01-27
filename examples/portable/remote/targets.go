//go:build targ

// Package remote demonstrates portable target package authoring.
// This package exports targets that can be imported and registered by other projects.
package remote

import (
	"context"
	"fmt"
	"os"

	"github.com/toejough/targ"
)

// Exported target variables - consumers can import these.
var (
	// Lint runs linting with configurable linter via LINTER env var
	Lint = targ.Targ(lint).
		Name("lint").
		Description("Run linter (set LINTER env var to choose linter)")

	// Test runs tests with coverage threshold via COVERAGE env var
	Test = targ.Targ(runTests).
		Name("test").
		Description("Run tests (set COVERAGE env var for threshold)")

	// Build compiles the project with optional output via OUTPUT env var
	Build = targ.Targ(build).
		Name("build").
		Description("Build project (set OUTPUT env var for binary name)")
)

//nolint:gochecknoinits // init required for targ.Register - targets auto-register on import
func init() {
	// Register all targets when package is imported
	targ.Register(Lint, Test, Build)
}

// lint runs the linter specified by LINTER env var
func lint(ctx context.Context) error {
	linter := os.Getenv("LINTER")
	if linter == "" {
		linter = "golangci-lint"
	}

	fmt.Printf("Running %s...\n", linter)
	return targ.RunContext(ctx, linter, "run", "./...")
}

// runTests runs tests with coverage threshold from COVERAGE env var
func runTests(ctx context.Context) error {
	threshold := os.Getenv("COVERAGE")
	if threshold == "" {
		threshold = "80"
	}

	fmt.Printf("Running tests with %s%% coverage threshold...\n", threshold)
	return targ.RunContext(ctx, "go", "test", "-cover", "./...")
}

// build compiles the project with output name from OUTPUT env var
func build(ctx context.Context) error {
	output := os.Getenv("OUTPUT")
	if output == "" {
		output = "app"
	}

	fmt.Printf("Building %s...\n", output)
	return targ.RunContext(ctx, "go", "build", "-o", output, ".")
}
