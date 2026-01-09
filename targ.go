package targ

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/toejough/targ/file"
	"github.com/toejough/targ/internal/core"
)

// --- Re-exported types from core ---

// Interleaved wraps a value with its parse position for tracking flag ordering.
type Interleaved[T any] = core.Interleaved[T]

// RunOptions controls runtime behavior for RunWithOptions.
type RunOptions = core.RunOptions

// TagKind identifies the type of a struct field in command parsing.
type TagKind = core.TagKind

// TagOptions contains parsed tag options for a struct field.
type TagOptions = core.TagOptions

// ExitError is returned when a command exits with a non-zero code.
type ExitError = core.ExitError

// Re-export TagKind constants
const (
	TagKindUnknown    = core.TagKindUnknown
	TagKindFlag       = core.TagKindFlag
	TagKindPositional = core.TagKindPositional
	TagKindSubcommand = core.TagKindSubcommand
)

// --- Public API ---

// Run executes the CLI using os.Args and exits on error.
func Run(targets ...interface{}) {
	RunWithOptions(RunOptions{AllowDefault: true}, targets...)
}

// RunWithOptions executes the CLI using os.Args and exits on error.
func RunWithOptions(opts RunOptions, targets ...interface{}) {
	err := core.RunWithEnv(core.NewOsEnv(), opts, targets...)
	if err != nil {
		if exitErr, ok := err.(ExitError); ok {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}

// ExecuteResult contains the result of executing commands.
type ExecuteResult struct {
	Output string
}

// Execute runs commands with the given args and returns results instead of exiting.
// This is useful for testing. Args should include the program name as the first element.
func Execute(args []string, targets ...interface{}) (ExecuteResult, error) {
	return ExecuteWithOptions(args, RunOptions{AllowDefault: true}, targets...)
}

// ExecuteWithOptions runs commands with given args and options, returning results.
// This is useful for testing. Args should include the program name as the first element.
func ExecuteWithOptions(args []string, opts RunOptions, targets ...interface{}) (ExecuteResult, error) {
	env := core.NewExecuteEnv(args)
	err := core.RunWithEnv(env, opts, targets...)
	return ExecuteResult{Output: env.Output()}, err
}

// DetectRootCommands filters a list of possible command objects to find those
// that are NOT subcommands of any other command in the list.
// It uses the `targ:"subcommand"` tag to identify relationships.
func DetectRootCommands(candidates ...interface{}) []interface{} {
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

		for i := 0; i < t.NumField(); i++ {
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
	var roots []interface{}
	for _, c := range candidates {
		v := reflect.ValueOf(c)
		t := v.Type()
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if !subcommandTypes[t] {
			roots = append(roots, c)
		}
	}

	return roots
}

// PrintCompletionScript outputs shell completion scripts for the given shell.
func PrintCompletionScript(shell string, binName string) error {
	return core.PrintCompletionScript(shell, binName)
}

// Deps executes each dependency exactly once per CLI run.
// Dependencies are tracked by their reflect.Value, so the same function or
// struct pointer will only run once even if Deps is called multiple times.
func Deps(targets ...interface{}) error {
	return core.Deps(targets...)
}

// ParallelDeps executes dependencies in parallel, ensuring each target runs once.
// Like Deps, each dependency only executes once per CLI run.
func ParallelDeps(targets ...interface{}) error {
	return core.ParallelDeps(targets...)
}

// --- Backwards-compatible file utility re-exports ---

// Match expands one or more patterns using fish-style globs.
// Deprecated: Use file.Match instead.
func Match(patterns ...string) ([]string, error) {
	return file.Match(patterns...)
}

// Newer reports whether inputs are newer than outputs.
// Deprecated: Use file.Newer instead.
func Newer(inputs []string, outputs []string) (bool, error) {
	return file.Newer(inputs, outputs)
}

// ChangeSet contains the files that changed between snapshots.
// Deprecated: Use file.ChangeSet instead.
type ChangeSet = file.ChangeSet

// WatchOptions configures the Watch function.
// Deprecated: Use file.WatchOptions instead.
type WatchOptions = file.WatchOptions

// Watch polls patterns for changes and invokes fn with any detected changes.
// Deprecated: Use file.Watch instead.
func Watch(ctx context.Context, patterns []string, opts WatchOptions, fn func(ChangeSet) error) error {
	return file.Watch(ctx, patterns, opts, fn)
}
