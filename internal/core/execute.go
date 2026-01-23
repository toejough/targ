package core

import (
	"errors"
	"fmt"
	"os"
)

// ExecuteRegistered runs the registered targets using os.Args and exits on error.
// This is used by the targ buildtool for packages that use explicit registration.
func ExecuteRegistered() {
	RunWithOptions(RunOptions{AllowDefault: true}, registry...)
}

// ExecuteRegisteredWithOptions runs the registered targets with custom options.
// Used by generated bootstrap code.
func ExecuteRegisteredWithOptions(opts RunOptions) {
	opts.AllowDefault = true
	RunWithOptions(opts, registry...)
}

// GetRegistry returns the current global registry (for testing).
func GetRegistry() []any {
	return registry
}

// Main runs the given targets as a CLI application.
func Main(targets ...any) {
	RegisterTarget(targets...)
	ExecuteRegistered()
}

// RegisterTarget adds targets to the global registry for later execution.
// Typically called from init() in packages with //go:build targ.
// Use ExecuteRegistered() in main() to run the registered targets.
func RegisterTarget(targets ...any) {
	registry = append(registry, targets...)
}

// RunWithOptions executes the CLI using os.Args and exits on error.
func RunWithOptions(opts RunOptions, targets ...any) {
	env := osRunEnv{}

	err := RunWithEnv(env, opts, targets...)
	if err != nil {
		var exitErr ExitError
		if errors.As(err, &exitErr) {
			env.Exit(exitErr.Code)
		} else {
			env.Exit(1)
		}
	}
}

// SetRegistry replaces the global registry (for testing).
func SetRegistry(targets []any) {
	registry = targets
}

// unexported variables.
var (
	registry []any //nolint:gochecknoglobals // Global registry is intentional for Register() API
)

type osRunEnv struct{}

func (osRunEnv) Args() []string {
	return os.Args
}

func (osRunEnv) Exit(code int) {
	os.Exit(code)
}

func (osRunEnv) Printf(f string, a ...any) {
	fmt.Printf(f, a...)
}

func (osRunEnv) Println(a ...any) {
	fmt.Println(a...)
}

func (osRunEnv) SupportsSignals() bool {
	return true
}
