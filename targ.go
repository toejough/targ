package targ

import (
	"context"
	"os"
	"reflect"
	"strings"

	"github.com/toejough/targ/file"
)

// Interleaved wraps a value with its parse position for tracking flag ordering.
// Use []Interleaved[T] when you need to know the relative order of flags
// across multiple slice fields (e.g., interleaved --include and --exclude).
type Interleaved[T any] struct {
	Value    T
	Position int
}

// Run executes the CLI using os.Args and exits on error.
func Run(targets ...interface{}) {
	RunWithOptions(RunOptions{AllowDefault: true}, targets...)
}

// RunOptions controls runtime behavior for RunWithOptions.
type RunOptions struct {
	AllowDefault      bool
	DisableHelp       bool
	DisableTimeout    bool
	DisableCompletion bool
}

// RunWithOptions executes the CLI using os.Args and exits on error.
func RunWithOptions(opts RunOptions, targets ...interface{}) {
	err := runWithEnv(osRunEnv{}, opts, targets...)
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
	env := &executeEnv{args: args}
	err := runWithEnv(env, opts, targets...)
	return ExecuteResult{Output: env.output.String()}, err
}

// TagKind identifies the type of a struct field in command parsing.
type TagKind string

const (
	TagKindUnknown    TagKind = "unknown"
	TagKindFlag       TagKind = "flag"
	TagKindPositional TagKind = "positional"
	TagKindSubcommand TagKind = "subcommand"
)

// TagOptions contains parsed tag options for a struct field.
type TagOptions struct {
	Kind        TagKind
	Name        string
	Short       string
	Desc        string
	Env         string
	Default     *string
	Enum        string
	Placeholder string
	Required    bool
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
