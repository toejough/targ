// Package targ provides a declarative CLI framework using struct tags.
package targ

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/toejough/targ/internal/core"
)

// Exported constants.
const (
	TagKindFlag       = core.TagKindFlag
	TagKindPositional = core.TagKindPositional
	TagKindSubcommand = core.TagKindSubcommand
	TagKindUnknown    = core.TagKindUnknown
)

// DepsOption configures Deps behavior.
type DepsOption = core.DepsOption

// ExecuteResult contains the result of executing commands.
type ExecuteResult struct {
	Output string
}

// ExitError is returned when a command exits with a non-zero code.
type ExitError = core.ExitError

// --- Re-exported types from core ---

// Interleaved wraps a value with its parse position for tracking flag ordering.
type Interleaved[T any] = core.Interleaved[T]

// RunOptions controls runtime behavior for RunWithOptions.
type RunOptions = core.RunOptions

// TagKind identifies the type of a struct field in command parsing.
type TagKind = core.TagKind

// TagOptions contains parsed tag options for a struct field.
type TagOptions = core.TagOptions

// ContinueOnError runs all dependencies even if one fails.
// Without this option, Deps fails fast (cancels remaining on first error).
func ContinueOnError() DepsOption { return core.ContinueOnError() }

// DetectShell returns the current shell name (bash, zsh, fish) or empty string.

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
func DetectRootCommands(candidates ...any) []any {
	// 1. Find all types that are referenced as subcommands
	subcommandTypes := make(map[reflect.Type]bool)

	for _, c := range candidates {
		v := reflect.ValueOf(c)
		t := v.Type()
		// Handle pointer to struct
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if t.Kind() != reflect.Struct {
			continue
		}

		for i := range t.NumField() {
			field := t.Field(i)

			tag := field.Tag.Get("targ")
			if strings.Contains(tag, "subcommand") {
				// This field type is a subcommand
				subType := field.Type
				if subType.Kind() == reflect.Ptr {
					subType = subType.Elem()
				}

				subcommandTypes[subType] = true
			}
		}
	}

	// 2. Filter candidates
	var roots []any

	for _, candidate := range candidates {
		val := reflect.ValueOf(candidate)

		typ := val.Type()
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}

		if !subcommandTypes[typ] {
			roots = append(roots, candidate)
		}
	}

	return roots
}

// Execute runs commands with the given args and returns results instead of exiting.
// This is useful for testing. Args should include the program name as the first element.
func Execute(args []string, targets ...any) (ExecuteResult, error) {
	return ExecuteWithOptions(args, RunOptions{AllowDefault: true}, targets...)
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

// PrintCompletionScript outputs shell completion scripts for the given shell.

// ResetDeps clears the dependency execution cache, allowing all targets
// to run again on subsequent Deps() calls. This is useful for watch mode
// where the same targets need to re-run on each file change.
func ResetDeps() {
	core.ResetDeps()
}

// --- Public API ---

// Run executes the CLI using os.Args and exits on error.
func Run(targets ...any) {
	RunWithOptions(RunOptions{AllowDefault: true}, targets...)
}

// RunWithOptions executes the CLI using os.Args and exits on error.
func RunWithOptions(opts RunOptions, targets ...any) {
	err := core.RunWithEnv(core.NewOsRunEnv(), opts, targets...)
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

// RepoURL returns the repository URL detected from .git/config.
// This is useful when you want to use the auto-detected repo URL in your
// own MoreInfoText or other help customization.
// Returns empty string if no .git/config is found or origin URL cannot be parsed.
