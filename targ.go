// Package targ provides a declarative CLI framework using struct tags.
package targ

import (
	"context"
	"errors"
	"fmt"
	"os"

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
	TagKindSubcommand = core.TagKindSubcommand
	TagKindUnknown    = core.TagKindUnknown
)

type DepsOption = core.DepsOption

type Example = core.Example

type ExecuteResult struct {
	Output string
}

type ExitError = core.ExitError

type Interleaved[T any] = core.Interleaved[T]

type RunOptions = core.RunOptions

type TagKind = core.TagKind

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

// DetectRootCommands filters a list of possible command objects to find those
// that are NOT subcommands of any other command in the list.
// It uses the `targ:"subcommand"` tag to identify relationships.

// 1. Find all types that are referenced as subcommands

// Handle pointer to struct

// This field type is a subcommand

// 2. Filter candidates

// EmptyExamples returns an empty slice to disable examples in help.
func EmptyExamples() []Example { return core.EmptyExamples() }

// Execute runs commands with the given args and returns results instead of exiting.
// This is useful for testing. Args should include the program name as the first element.
func Execute(args []string, targets ...any) (ExecuteResult, error) {
	return ExecuteWithOptions(args, RunOptions{AllowDefault: true}, targets...)
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

// ExecuteWithOptions runs commands with given args and options, returning results.
// This is useful for testing. Args should include the program name as the first element.
func ExecuteWithOptions(
	args []string,
	opts RunOptions,
	targets ...any,
) (ExecuteResult, error) {
	env := core.NewExecuteEnv(args)
	err := core.RunWithEnv(env, opts, targets...)

	return ExecuteResult{Output: env.Output()}, err
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
	registry = append(registry, targets...)
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
func RunWithOptions(opts RunOptions, targets ...any) {
	err := core.RunWithEnv(osRunEnv{}, opts, targets...)
	if err != nil {
		var exitErr ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}

		os.Exit(1)
	}
}

// WithContext passes a custom context to dependencies.
// Useful for cancellation in watch mode.
func WithContext(ctx context.Context) DepsOption { return core.WithContext(ctx) }

// unexported variables.
var (
	registry []any //nolint:gochecknoglobals // Global registry is intentional for Register() API
)

type osRunEnv struct{}

func (osRunEnv) Args() []string { return os.Args }

func (osRunEnv) Exit(code int) { os.Exit(code) }

func (osRunEnv) Printf(f string, a ...any) { fmt.Printf(f, a...) }

func (osRunEnv) Println(a ...any) { fmt.Println(a...) }

func (osRunEnv) SupportsSignals() bool { return true }
