// Package targ provides a declarative CLI framework using struct tags.
package targ

import (
	"context"

	"github.com/toejough/targ/internal/core"
)

// Exported constants.
const (
	// Disabled is a sentinel value for Target builder methods that indicates
	// the setting should be controlled by CLI flags rather than compile-time config.
	// Example: targ.Targ(Build).Watch(targ.Disabled) allows --watch flag to control watching.
	Disabled          = "__targ_disabled__"
	TagKindFlag       = core.TagKindFlag
	TagKindPositional = core.TagKindPositional
	TagKindUnknown    = core.TagKindUnknown
)

// DepsOption configures dependency execution behavior.
type DepsOption = core.DepsOption

// Example represents a usage example shown in help text.
type Example = core.Example

// ExecuteResult contains the result of executing a command.
type ExecuteResult = core.ExecuteResult

// ExitError represents a non-zero exit code from command execution.
type ExitError = core.ExitError

// Interleaved wraps a value to be parsed from interleaved positional arguments.
type Interleaved[T any] = core.Interleaved[T]

// RunOptions configures command execution behavior.
type RunOptions = core.RunOptions

// TagKind represents the type of a struct tag (flag, positional, subcommand).
type TagKind = core.TagKind

// TagOptions holds parsed struct tag options for CLI argument handling.
type TagOptions = core.TagOptions

// AppendBuiltinExamples adds built-in examples after custom examples.
func AppendBuiltinExamples(
	custom ...Example,
) []Example {
	return core.AppendBuiltinExamples(custom...)
}

// BuiltinExamples returns the default targ examples (completion setup, chaining).
func BuiltinExamples() []Example { return core.BuiltinExamples() }

// ContinueOnError runs all dependencies even if one fails.
// Without this option, Deps fails fast (cancels remaining on first error).
func ContinueOnError() DepsOption { return core.ContinueOnError() }

// Deps executes dependencies, each exactly once per CLI run.
// Options can be mixed with targets in any order:
//
//	targ.Deps(A, B, C)                              // serial, fail-fast
//	targ.Deps(A, B, C, targ.Parallel())             // parallel, fail-fast
//	targ.Deps(A, B, C, targ.ContinueOnError())      // serial, run all
//	targ.Deps(A, B, targ.Parallel(), targ.WithContext(ctx))
func Deps(args ...any) error {
	return core.Deps(args...)
}

// EmptyExamples returns an empty slice to disable examples in help.
func EmptyExamples() []Example { return core.EmptyExamples() }

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

// Parallel runs dependencies concurrently instead of sequentially.
func Parallel() DepsOption { return core.Parallel() }

// PrependBuiltinExamples adds built-in examples before custom examples.
func PrependBuiltinExamples(custom ...Example) []Example {
	return core.PrependBuiltinExamples(custom...)
}

// Register adds targets to the global registry for later execution.
// Typically called from init() in packages with //go:build targ.
// Use ExecuteRegistered() in main() to run the registered targets.
func Register(targets ...any) {
	core.RegisterTarget(targets...)
}

// PrintCompletionScript outputs shell completion scripts for the given shell.

// ResetDeps clears the dependency execution cache, allowing all targets
// to run again on subsequent Deps() calls. This is useful for watch mode
// where the same targets need to re-run on each file change.
func ResetDeps() {
	core.ResetDeps()
}

// --- Public API ---

// Run executes the CLI using os.Args and exits on error.

// RunWithOptions executes the CLI using os.Args and exits on error.

// WithContext passes a custom context to dependencies.
// Useful for cancellation in watch mode.
func WithContext(ctx context.Context) DepsOption { return core.WithContext(ctx) }
