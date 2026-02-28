// Package targ provides a declarative CLI framework using struct tags.
package targ

import (
	"context"
	"os"

	"github.com/toejough/targ/internal/core"
	internalfile "github.com/toejough/targ/internal/file"
	internalsh "github.com/toejough/targ/internal/sh"
)

// Exported constants.
const (
	// Cancelled indicates a target was cancelled during parallel execution.
	Cancelled = core.Cancelled
	// CollectAllErrors causes parallel deps to run all targets to completion
	// and collect all errors, rather than cancelling on first failure.
	CollectAllErrors = core.CollectAllErrors
	// DepModeMixed indicates a target has multiple dependency groups with different modes.
	DepModeMixed = core.DepModeMixed
	// DepModeParallel executes all dependencies concurrently.
	DepModeParallel = core.DepModeParallel
	// DepModeSerial executes dependencies one at a time in order.
	DepModeSerial = core.DepModeSerial
	// Disabled is a sentinel value for Target builder methods that indicates
	// the setting should be controlled by CLI flags rather than compile-time config.
	// Example: targ.Targ(Build).Watch(targ.Disabled) allows --watch flag to control watching.
	Disabled = "__targ_disabled__"
	// Errored indicates a target failed with an error.
	Errored = core.Errored
	// Fail indicates a target execution failed.
	Fail = core.Fail
	// Pass indicates a target executed successfully.
	Pass              = core.Pass
	TagKindFlag       = core.TagKindFlag
	TagKindPositional = core.TagKindPositional
	TagKindUnknown    = core.TagKindUnknown
)

// Exported variables.
var (
	ErrEmptyDest       = internalfile.ErrEmptyDest
	ErrNoInputPatterns = internalfile.ErrNoInputPatterns
	ErrNoPatterns      = internalfile.ErrNoPatterns
	ErrUnmatchedBrace  = internalfile.ErrUnmatchedBrace
)

// ChangeSet holds the files that changed between watch polls.
type ChangeSet = internalfile.ChangeSet

// DepGroup is the exported view of a dependency group.
type DepGroup = core.DepGroup

// DepMode controls how dependencies are executed (parallel or serial).
type DepMode = core.DepMode

// DepOption is an option that modifies dependency execution behavior.
type DepOption = core.DepOption

// Example represents a usage example shown in help text.
type Example = core.Example

// ExecuteResult contains the result of executing a command.
type ExecuteResult = core.ExecuteResult

// ExitError represents a non-zero exit code from command execution.
type ExitError = core.ExitError

// Interleaved wraps a value to be parsed from interleaved positional arguments.
type Interleaved[T any] = core.Interleaved[T]

// MultiError wraps multiple target failures from a collect-all-errors parallel run.
type MultiError = core.MultiError

// Result represents the outcome status of a parallel target execution.
type Result = core.Result

// RunOptions configures command execution behavior.
type RunOptions = core.RunOptions

// RuntimeOverrides are CLI flags that override compile-time Target settings.
type RuntimeOverrides = core.RuntimeOverrides

// TagKind represents the type of a struct tag (flag, positional, subcommand).
type TagKind = core.TagKind

// TagOptions holds parsed struct tag options for CLI argument handling.
type TagOptions = core.TagOptions

// Target represents a build target that can be invoked from the CLI.
type Target = core.Target

// TargetGroup represents a named collection of targets that can be run together.
type TargetGroup = core.TargetGroup

// WatchOptions configures file watching behavior.
type WatchOptions = internalfile.WatchOptions

// --- Examples ---

// AppendBuiltinExamples adds built-in examples after custom examples.
func AppendBuiltinExamples(custom ...Example) []Example {
	return core.AppendBuiltinExamples(custom...)
}

// BuiltinExamples returns the default targ examples (completion setup, chaining).
func BuiltinExamples() []Example { return core.BuiltinExamples() }

// Checksum reports whether the content hash of inputs differs from the stored hash at dest.
// When the hash changes, the new hash is written to dest.
func Checksum(inputs []string, dest string) (bool, error) {
	return internalfile.Checksum(inputs, dest, func(patterns []string) ([]string, error) {
		return Match(patterns...)
	}, nil)
}

// DeregisterFrom removes all targets registered by the named package.
// Must be called from init() before targ executes.
//
// Example:
//
//	import targets "github.com/alice/go-targets"
//
//	func init() {
//	    targ.DeregisterFrom("github.com/alice/go-targets")
//	    targ.Register(targets.Test) // Re-register just this one
//	}
//
// Returns error if packagePath is empty.
func DeregisterFrom(packagePath string) error {
	return core.DeregisterFrom(packagePath)
}

// EmptyExamples returns an empty slice to disable examples in help.
func EmptyExamples() []Example { return core.EmptyExamples() }

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
// Call this once at program startup to ensure Ctrl-C kills all spawned processes.
func EnableCleanup() {
	internalsh.EnableCleanup()
}

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	return internalsh.ExeSuffix(nil)
}

// --- Execution ---

// Execute runs commands with the given args and returns results instead of exiting.
// This is useful for testing. Args should include the program name as the first element.
func Execute(args []string, targets ...any) (ExecuteResult, error) {
	return core.Execute(args, targets...)
}

// ExecuteRegistered runs the registered targets using os.Args and exits on error.
// This is used by the targ buildtool for packages that use explicit registration.
func ExecuteRegistered() {
	core.ExecuteRegistered()
}

// ExecuteRegisteredWithOptions runs the registered targets with custom options.
// Used by generated bootstrap code.
func ExecuteRegisteredWithOptions(opts RunOptions) {
	core.ExecuteRegisteredWithOptions(opts)
}

// ExecuteWithOptions runs commands with given args and options, returning results.
// This is useful for testing. Args should include the program name as the first element.
func ExecuteWithOptions(
	args []string,
	opts RunOptions,
	targets ...any,
) (ExecuteResult, error) {
	return core.ExecuteWithOptions(args, opts, targets...)
}

// Group creates a named group containing the given members.
// Members can be *Target or *Group (for nested hierarchies).
//
//	var lint = targ.Group("lint", lintFast, lintFull)
//	var dev = targ.Group("dev", build, lint, test)
func Group(name string, members ...any) *TargetGroup {
	return core.Group(name, members...)
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return internalsh.IsWindowsOS()
}

// Main runs the given targets as a CLI application.
// Call this from main() for standalone binaries:
//
//	func main() {
//	    targ.Main(
//	        targ.Targ(build),
//	        targ.Targ(test),
//	    )
//	}
func Main(targets ...any) {
	core.Main(targets...)
}

// --- File Utilities ---

// Match expands one or more patterns using fish-style globs (including ** and {a,b}).
func Match(patterns ...string) ([]string, error) {
	return internalfile.Match(patterns...)
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	return internalsh.Output(nil, name, args...)
}

// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return internalsh.OutputContext(ctx, name, args, os.Stdin)
}

// PrependBuiltinExamples adds built-in examples before custom examples.
func PrependBuiltinExamples(custom ...Example) []Example {
	return core.PrependBuiltinExamples(custom...)
}

// Print writes output that is automatically prefixed with the target name in parallel mode.
// In serial mode, it writes directly to stdout.
func Print(ctx context.Context, args ...any) {
	core.Print(ctx, args...)
}

// Printf writes formatted output that is automatically prefixed with the target name in parallel mode.
// In serial mode, it writes directly to stdout.
func Printf(ctx context.Context, format string, args ...any) {
	core.Printf(ctx, format, args...)
}

// Register adds targets to the global registry for later execution.
// Typically called from init() in packages with //go:build targ.
// Use ExecuteRegistered() in main() to run the registered targets.
func Register(targets ...any) {
	core.RegisterTargetWithSkip(core.CallerSkipPublicAPI, targets...)
}

// Run executes a command streaming stdout/stderr.
func Run(name string, args ...string) error {
	return internalsh.Run(nil, name, args...)
}

// RunContext executes a command with context support.
// When ctx is cancelled, the process and all its children are killed.
// In parallel mode, stdout/stderr are routed through the parallel printer.
func RunContext(ctx context.Context, name string, args ...string) error {
	return core.RunContext(ctx, name, args...)
}

// RunContextV executes a command, prints it first, with context support.
// When ctx is cancelled, the process and all its children are killed.
// In parallel mode, stdout/stderr are routed through the parallel printer.
func RunContextV(ctx context.Context, name string, args ...string) error {
	return core.RunContextV(ctx, name, args...)
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	return internalsh.RunV(nil, name, args...)
}

// --- Shell Execution ---

// --- Target and Group Creation ---

// Targ creates a Target from a function or shell command string.
//
// Function targets:
//
//	var build = targ.Targ(Build)
//
// Shell command targets (run in user's shell):
//
//	var lint = targ.Targ("golangci-lint run ./...")
//
// Deps-only targets (no function, just runs dependencies):
//
//	var all = targ.Targ().Name("all").Deps(build, test, lint)
func Targ(fn ...any) *Target {
	return core.Targ(fn...)
}

// Watch polls patterns for changes and invokes callback with any detected changes.
func Watch(
	ctx context.Context,
	patterns []string,
	opts WatchOptions,
	callback func(ChangeSet) error,
) error {
	return internalfile.Watch(ctx, patterns, opts, callback, func(p []string) ([]string, error) {
		return Match(p...)
	}, nil)
}

// WithExeSuffix appends the OS-specific executable suffix if missing.
func WithExeSuffix(name string) string {
	return internalsh.WithExeSuffix(nil, name)
}
