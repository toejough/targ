package targ

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Target wraps a function or shell command with configuration.
// Use Targ() to create a Target, then chain builder methods.
type Target struct {
	fn          any    // func(...) or string (shell command)
	name        string // CLI name override
	description string // help text
}

// Description sets the help text for this target.
func (t *Target) Description(s string) *Target {
	t.description = s
	return t
}

// Fn returns the underlying function or shell command string.
// This is used internally for discovery and execution.
func (t *Target) Fn() any {
	return t.fn
}

// GetDescription returns the configured description, or empty if not set.
func (t *Target) GetDescription() string {
	return t.description
}

// GetName returns the configured name, or empty if not set.
func (t *Target) GetName() string {
	return t.name
}

// Name sets the CLI name for this target.
// By default, the function name is used (converted to kebab-case).
func (t *Target) Name(s string) *Target {
	t.name = s
	return t
}

// Run executes the target with the full execution configuration.
// For now, this is a placeholder - full implementation comes in later phases.
func (t *Target) Run(ctx context.Context, args ...any) error {
	// TODO: implement full execution with deps, cache, retry, etc.
	// For now, just call the function directly
	switch fn := t.fn.(type) {
	case string:
		// Shell command - not implemented yet
		return fmt.Errorf("%w: %s", errShellNotImplemented, fn)
	default:
		// Function - call it
		return callFunc(ctx, fn, args)
	}
}

// Targ creates a Target from a function or shell command string.
//
// Function targets:
//
//	var build = targ.Targ(Build)
//
// Shell command targets (run in user's shell):
//
//	var lint = targ.Targ("golangci-lint run ./...")
func Targ(fn any) *Target {
	if fn == nil {
		panic("targ.Targ: fn cannot be nil")
	}

	// Validate fn is a function or string
	switch v := fn.(type) {
	case string:
		if v == "" {
			panic("targ.Targ: shell command cannot be empty")
		}
	default:
		fnValue := reflect.ValueOf(fn)
		if fnValue.Kind() != reflect.Func {
			panic(fmt.Sprintf("targ.Targ: expected func or string, got %T", fn))
		}
	}

	return &Target{fn: fn}
}

// unexported variables.
var (
	errShellNotImplemented = errors.New("shell command execution not yet implemented")
)

// callFunc calls a function with the appropriate signature.
func callFunc(ctx context.Context, fn any, args []any) error {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	// Build call arguments based on function signature
	numIn := fnType.NumIn()
	callArgs := make([]reflect.Value, 0, numIn)
	argIdx := 0

	for i := range numIn {
		paramType := fnType.In(i)

		// Check if this param is context.Context
		if paramType.Implements(reflect.TypeFor[context.Context]()) {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
			continue
		}

		// Use provided arg if available
		if argIdx < len(args) {
			callArgs = append(callArgs, reflect.ValueOf(args[argIdx]))
			argIdx++

			continue
		}

		// Create zero value for missing args
		callArgs = append(callArgs, reflect.Zero(paramType))
	}

	// Call the function
	results := fnValue.Call(callArgs)

	// Check for error return
	if len(results) > 0 {
		last := results[len(results)-1]
		if last.Type().Implements(reflect.TypeFor[error]()) {
			if !last.IsNil() {
				err, _ := last.Interface().(error)
				return err
			}
		}
	}

	return nil
}
