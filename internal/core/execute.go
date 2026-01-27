package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// DeregisterFrom queues a package path for deregistration.
// All targets from this package will be removed during registry resolution.
// Returns error if packagePath is empty.
func DeregisterFrom(packagePath string) error {
	if packagePath == "" {
		return errors.New("package path cannot be empty")
	}

	// Check if already queued (idempotent)
	if slices.Contains(deregistrations, packagePath) {
		return nil
	}

	deregistrations = append(deregistrations, packagePath)

	return nil
}

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

// GetDeregistrations returns the current deregistrations queue (for testing).
func GetDeregistrations() []string {
	return deregistrations
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
// Automatically sets sourcePkg on each *Target using runtime.Caller.
func RegisterTarget(targets ...any) {
	// Detect calling package once for all targets
	sourcePkg, _ := callerPackagePath(1)

	// Set source on each Target before appending to registry
	for _, item := range targets {
		if target, ok := item.(*Target); ok {
			// Only set if not already set (preserve explicit source)
			if target.sourcePkg == "" && sourcePkg != "" {
				target.sourcePkg = sourcePkg
			}
		}
	}

	registry = append(registry, targets...)
}

// ResetDeregistrations clears the deregistrations queue (for testing).
func ResetDeregistrations() {
	deregistrations = nil
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
	deregistrations []string //nolint:gochecknoglobals // Global deregistrations queue is intentional for DeregisterFrom() API
	registry        []any    //nolint:gochecknoglobals // Global registry is intentional for Register() API
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
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	return dir, nil
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
