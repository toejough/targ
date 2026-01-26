package core

import (
	"fmt"
	"io"
	"os"
	"strings"
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
// Exit is handled by RunWithEnv via the environment interface.
func RunWithOptions(opts RunOptions, targets ...any) {
	env := osRunEnv{}
	_ = RunWithEnv(env, opts, targets...)
	// Exit is called by RunWithEnv via env.Exit()
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

func (osRunEnv) BinaryName() string {
	if name := os.Getenv("TARG_BIN_NAME"); name != "" {
		return name
	}

	if len(os.Args) == 0 {
		return "targ"
	}

	name := os.Args[0]
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	if idx := strings.LastIndex(name, "\\"); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

func (osRunEnv) Exit(code int) {
	os.Exit(code)
}

func (osRunEnv) Getenv(key string) string {
	return os.Getenv(key)
}

func (osRunEnv) Getwd() (string, error) {
	return os.Getwd()
}

func (osRunEnv) Printf(f string, a ...any) {
	fmt.Printf(f, a...)
}

func (osRunEnv) Println(a ...any) {
	fmt.Println(a...)
}

func (osRunEnv) Stdout() io.Writer {
	return os.Stdout
}

func (osRunEnv) SupportsSignals() bool {
	return true
}
