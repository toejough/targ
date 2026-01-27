package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"strings"
)

// DeregisterFrom queues a package path for deregistration.
// All targets from this package will be removed during registry resolution.
// Returns error if packagePath is empty or if called after resolution.
func DeregisterFrom(packagePath string) error {
	if registryResolved {
		return errDeregisterAfterResolution
	}

	if packagePath == "" {
		return errEmptyPackagePath
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
	env := osRunEnv{}

	_ = ExecuteWithResolution(env, RunOptions{AllowDefault: true})
	// Exit is called by ExecuteWithResolution via env.Exit()
}

// ExecuteRegisteredWithOptions runs the registered targets with custom options.
// Used by generated bootstrap code.
func ExecuteRegisteredWithOptions(opts RunOptions) {
	env := osRunEnv{}
	opts.AllowDefault = true

	_ = ExecuteWithResolution(env, opts)
	// Exit is called by ExecuteWithResolution via env.Exit()
}

// ExecuteWithResolution resolves the global registry (applying
// deregistrations and detecting conflicts) and executes via RunWithEnv.
// On resolution error, prints to stderr and exits with code 1.
// Exported for testing - production code should use ExecuteRegistered.
func ExecuteWithResolution(env RunEnv, opts RunOptions) error {
	// Resolve registry (applies deregistrations, detects conflicts)
	resolved, err := resolveRegistry()
	if err != nil {
		// Print error to stderr
		fmt.Fprintf(os.Stderr, "%v\n", err)

		// Exit with error code
		env.Exit(1)

		return err
	}

	// Execute with resolved registry
	return RunWithEnv(env, opts, resolved...)
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
	RegisterTargetWithSkip(2, targets...)
}

// RegisterTargetWithSkip adds targets to the global registry with custom caller skip depth.
// The skip parameter controls which caller's package is used for source attribution:
// - skip=0: resolves to the core package itself (RegisterTargetWithSkip)
// - skip=1: resolves to the direct caller
// - skip=2: resolves to the caller's caller (useful for public API wrappers)
func RegisterTargetWithSkip(skip int, targets ...any) {
	// Detect calling package once for all targets
	sourcePkg, _ := callerPackagePath(skip)

	// Set source on each Target or TargetGroup before appending to registry
	for _, item := range targets {
		if target, ok := item.(*Target); ok {
			// Only set if not already set (preserve explicit source)
			if target.sourcePkg == "" && sourcePkg != "" {
				target.sourcePkg = sourcePkg
			}
		}

		if group, ok := item.(*TargetGroup); ok {
			// Only set if not already set (preserve explicit source)
			if group.sourcePkg == "" && sourcePkg != "" {
				group.sourcePkg = sourcePkg
			}
		}
	}

	registry = append(registry, targets...)
}

// ResetDeregistrations clears the deregistrations queue (for testing).
func ResetDeregistrations() {
	deregistrations = nil
}

// ResetResolved resets the resolved flag (for testing).
func ResetResolved() {
	registryResolved = false
}

// RunWithOptions executes the CLI using os.Args and exits on error.
// Exit is handled by RunWithEnv via the environment interface.
func RunWithOptions(opts RunOptions, targets ...any) {
	env := osRunEnv{}
	_ = RunWithEnv(env, opts, targets...)
	// Exit is called by RunWithEnv via env.Exit()
}

// SetMainModuleForTest injects a main module provider (for testing).
func SetMainModuleForTest(fn func() (string, bool)) {
	mainModuleProvider = fn
}

// SetRegistry replaces the global registry (for testing).
func SetRegistry(targets []any) {
	registry = targets
}

// getMainModule returns the main module path using runtime/debug.ReadBuildInfo.
// Returns (modulePath, true) on success, ("", false) on failure.
func getMainModule() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}

	return info.Main.Path, true
}

// unexported variables.
var (
	deregistrations              []string //nolint:gochecknoglobals // Intentional global for DeregisterFrom() API
	errDeregisterAfterResolution = errors.New(
		"targ: DeregisterFrom() must be called during init(), not after targ has started",
	)
	errEmptyPackagePath = errors.New("package path cannot be empty")
	mainModuleProvider  = getMainModule //nolint:gochecknoglobals // Default to runtime detection, injected for testing
	registry            []any           //nolint:gochecknoglobals // Global registry is intentional for Register() API
	registryResolved    bool            //nolint:gochecknoglobals // Global flag to track resolution state
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
