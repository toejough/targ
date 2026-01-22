package targ

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/toejough/targ/sh"
)

// DepMode specifies how dependencies are executed.
type DepMode int

// DepMode values.
const (
	// DepModeSerial executes dependencies one at a time in order.
	DepModeSerial DepMode = iota
	// DepModeParallel executes all dependencies concurrently.
	DepModeParallel
)

// Target wraps a function or shell command with configuration.
// Use Targ() to create a Target, then chain builder methods.
type Target struct {
	fn          any           // func(...) or string (shell command)
	name        string        // CLI name override
	description string        // help text
	deps        []*Target     // dependencies to run before this target
	depMode     DepMode       // serial or parallel dependency execution
	timeout     time.Duration // execution timeout (0 = no timeout)
}

// Deps sets dependencies that run serially before this target.
// Each dependency runs exactly once even if referenced multiple times.
func (t *Target) Deps(targets ...*Target) *Target {
	t.deps = targets
	t.depMode = DepModeSerial

	return t
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

// ParallelDeps sets dependencies that run concurrently before this target.
// Each dependency runs exactly once even if referenced multiple times.
func (t *Target) ParallelDeps(targets ...*Target) *Target {
	t.deps = targets
	t.depMode = DepModeParallel

	return t
}

// Run executes the target with the full execution configuration.
func (t *Target) Run(ctx context.Context, args ...any) error {
	// Apply timeout if configured
	if t.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	// Run dependencies first
	if len(t.deps) > 0 {
		err := t.runDeps(ctx)
		if err != nil {
			return err
		}
	}

	// Execute the target itself
	return t.execute(ctx, args)
}

// Timeout sets the maximum execution time for this target.
// If the timeout is exceeded, the context is cancelled.
func (t *Target) Timeout(d time.Duration) *Target {
	t.timeout = d
	return t
}

// execute runs the target's function or shell command.
func (t *Target) execute(ctx context.Context, args []any) error {
	switch fn := t.fn.(type) {
	case string:
		return runShellCommand(ctx, fn)
	default:
		return callFunc(ctx, fn, args)
	}
}

// runDeps executes dependencies according to the configured mode.
func (t *Target) runDeps(ctx context.Context) error {
	if t.depMode == DepModeParallel {
		return t.runDepsParallel(ctx)
	}

	return t.runDepsSerial(ctx)
}

// runDepsParallel executes all dependencies concurrently.
func (t *Target) runDepsParallel(ctx context.Context) error {
	errs := make(chan error, len(t.deps))

	for _, dep := range t.deps {
		go func(d *Target) {
			errs <- d.Run(ctx)
		}(dep)
	}

	var firstErr error

	for range t.deps {
		err := <-errs
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// runDepsSerial executes dependencies one at a time in order.
func (t *Target) runDepsSerial(ctx context.Context) error {
	for _, dep := range t.deps {
		err := dep.Run(ctx)
		if err != nil {
			return err
		}
	}

	return nil
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

// runShellCommand executes a shell command string.
// The command is run via the user's shell (sh -c on Unix).
func runShellCommand(ctx context.Context, cmd string) error {
	err := sh.RunContext(ctx, "sh", "-c", cmd)
	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	return nil
}
