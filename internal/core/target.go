package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"time"

	internalfile "github.com/toejough/targ/internal/file"
	internalsh "github.com/toejough/targ/internal/sh"
)

// DepMode controls how dependencies are executed (parallel or serial).
type DepMode int

// DepMode values.
const (
	// DepModeSerial executes dependencies one at a time in order.
	DepModeSerial DepMode = iota
	// DepModeParallel executes all dependencies concurrently.
	DepModeParallel
	// DepModeMixed indicates a target has multiple dependency groups with different modes.
	DepModeMixed
)

// String returns the string representation of the dependency mode.
func (m DepMode) String() string {
	switch m {
	case DepModeParallel:
		return depModeParallelStr
	case DepModeMixed:
		return depModeMixedStr
	default:
		return depModeSerialStr
	}
}

type depGroup struct {
	targets []*Target
	mode    DepMode
}

// DepGroup is the exported view of a dependency group.
type DepGroup struct {
	Targets []*Target
	Mode    DepMode
}

// Target represents a build target that can be invoked from the CLI.
type Target struct {
	fn              any           // func(...) or string (shell command)
	name            string        // CLI name override
	description     string        // help text
	depGroups       []depGroup    // dependency groups with execution modes
	timeout         time.Duration // execution timeout (0 = no timeout)
	cache           []string      // file patterns for cache invalidation
	cacheDir        string        // directory to store cache files
	watch           []string      // file patterns for watch mode
	times           int           // number of times to run (0 = once)
	whileFn         func() bool   // predicate to check before each run
	retry           bool          // continue despite failures
	backoffInitial  time.Duration // initial backoff delay after failure
	backoffMultiply float64       // backoff multiplier for exponential backoff

	// Disabled flags - when true, CLI flags control the setting
	watchDisabled bool
	cacheDisabled bool

	// Source attribution
	sourcePkg      string // package that registered this target
	nameOverridden bool   // true if Name() was called
}

// Backoff sets exponential backoff delay after failures.
// The delay starts at initial and multiplies by factor after each failure.
// Only applies when Retry() is enabled.
func (t *Target) Backoff(initial time.Duration, factor float64) *Target {
	t.backoffInitial = initial
	t.backoffMultiply = factor

	return t
}

// Cache sets file patterns for cache invalidation.
// If the files matching these patterns haven't changed since the last run,
// the target execution is skipped.
// Pass core.Disabled to allow CLI --cache flag to control caching.
func (t *Target) Cache(patterns ...string) *Target {
	if len(patterns) == 1 && patterns[0] == disabledSentinel {
		t.cacheDisabled = true
		t.cache = nil

		return t
	}

	t.cache = patterns

	return t
}

// CacheDir sets the directory where cache checksum files are stored.
// If not set, defaults to ".targ-cache" in the current directory.
func (t *Target) CacheDir(dir string) *Target {
	t.cacheDir = dir
	return t
}

// Deps sets dependencies that run before this target.
// Each dependency runs exactly once even if referenced multiple times.
// Pass targ.Parallel as the last argument to run dependencies concurrently.
//
//	targ.Targ(build).Deps(generate, compile)           // serial (default)
//	targ.Targ(build).Deps(lint, test, targ.Parallel)   // parallel
func (t *Target) Deps(args ...any) *Target {
	mode := DepModeSerial
	var targets []*Target

	for _, arg := range args {
		switch v := arg.(type) {
		case *Target:
			targets = append(targets, v)
		case DepMode:
			mode = v
		}
	}

	if len(targets) == 0 {
		return t
	}

	// Coalesce with last group if same mode
	if len(t.depGroups) > 0 && t.depGroups[len(t.depGroups)-1].mode == mode {
		t.depGroups[len(t.depGroups)-1].targets = append(
			t.depGroups[len(t.depGroups)-1].targets, targets...)
	} else {
		t.depGroups = append(t.depGroups, depGroup{targets: targets, mode: mode})
	}

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

// GetBackoff returns the backoff configuration (initial delay, multiplier).
func (t *Target) GetBackoff() (time.Duration, float64) {
	return t.backoffInitial, t.backoffMultiply
}

// GetConfig returns the target's configuration for conflict detection.
// Returns (watchPatterns, cachePatterns, watchDisabled, cacheDisabled).
func (t *Target) GetConfig() ([]string, []string, bool, bool) {
	return t.watch, t.cache, t.watchDisabled, t.cacheDisabled
}

// GetDepMode returns the dependency execution mode.
func (t *Target) GetDepMode() DepMode {
	if len(t.depGroups) == 0 {
		return DepModeSerial
	}

	mode := t.depGroups[0].mode
	for _, g := range t.depGroups[1:] {
		if g.mode != mode {
			return DepModeMixed
		}
	}
	return mode
}

// GetDeps returns the target's dependencies.
func (t *Target) GetDeps() []*Target {
	var all []*Target
	for _, g := range t.depGroups {
		all = append(all, g.targets...)
	}
	return all
}

// GetDepGroups returns the dependency groups with their execution modes.
func (t *Target) GetDepGroups() []DepGroup {
	groups := make([]DepGroup, len(t.depGroups))
	for i, g := range t.depGroups {
		groups[i] = DepGroup{Targets: g.targets, Mode: g.mode}
	}
	return groups
}

// GetDescription returns the configured description, or empty if not set.
func (t *Target) GetDescription() string {
	return t.description
}

// GetName returns the configured name, or derives it from the function name.
func (t *Target) GetName() string {
	if t.name != "" {
		return t.name
	}

	// Derive name from function
	v := reflect.ValueOf(t.fn)
	if v.Kind() != reflect.Func || v.IsNil() {
		return ""
	}

	name := functionName(v)
	if name == "" {
		return ""
	}

	return camelToKebab(name)
}

// GetRetry returns whether retry is enabled.
func (t *Target) GetRetry() bool {
	return t.retry
}

// GetSource returns the package path that registered this target.
func (t *Target) GetSource() string {
	return t.sourcePkg
}

// GetTimeout returns the target's timeout duration.
func (t *Target) GetTimeout() time.Duration {
	return t.timeout
}

// GetTimes returns the number of times to run.
func (t *Target) GetTimes() int {
	return t.times
}

// IsRenamed returns true if Name() was called to override the default name.
func (t *Target) IsRenamed() bool {
	return t.nameOverridden
}

// Name sets the CLI name for this target.
// By default, the function name is used (converted to kebab-case).
func (t *Target) Name(s string) *Target {
	t.name = s
	// Only mark as overridden if target has been registered (sourcePkg is set)
	// This distinguishes between package author setting name vs consumer renaming
	if t.sourcePkg != "" {
		t.nameOverridden = true
	}

	return t
}

// Retry makes the target continue to the next iteration even if execution fails.
// Without Retry, the target stops on the first error.
// Use with Times() or While() to retry multiple times.
func (t *Target) Retry() *Target {
	t.retry = true
	return t
}

// Run executes the target with the full execution configuration.
// If Watch() patterns are set, Run() will re-run on file changes until context is cancelled.
func (t *Target) Run(ctx context.Context, args ...any) error {
	// Run once initially
	err := t.runOnce(ctx, args)
	if err != nil {
		return err
	}

	// If watch patterns set, watch for changes and re-run
	if len(t.watch) > 0 {
		err := internalfile.Watch(
			ctx,
			t.watch,
			internalfile.WatchOptions{},
			func(_ internalfile.ChangeSet) error {
				return t.runOnce(ctx, args)
			},
			func(p []string) ([]string, error) { return internalfile.Match(p...) },
			nil,
		)
		if err != nil {
			return fmt.Errorf("watching files: %w", err)
		}
	}

	return nil
}

// SetSourceForTest sets the source package path (for testing only).
func (t *Target) SetSourceForTest(pkg string) {
	t.sourcePkg = pkg
}

// Timeout sets the maximum execution time for this target.
// If the timeout is exceeded, the context is cancelled.
func (t *Target) Timeout(d time.Duration) *Target {
	t.timeout = d
	return t
}

// Times sets how many times to run the target.
// Without Retry(), stops on first failure.
// With Retry(), continues to run all iterations.
func (t *Target) Times(n int) *Target {
	t.times = n
	return t
}

// Watch sets file patterns to watch for changes.
// When set, Run() will re-run the target when matching files change.
// Pass core.Disabled to allow CLI --watch flag to control watching.
func (t *Target) Watch(patterns ...string) *Target {
	if len(patterns) == 1 && patterns[0] == disabledSentinel {
		t.watchDisabled = true
		t.watch = nil

		return t
	}

	t.watch = patterns

	return t
}

// While sets a predicate that's checked before each run.
// The target runs as long as the predicate returns true.
// Can be combined with Times() - the earliest stopping condition wins.
func (t *Target) While(fn func() bool) *Target {
	t.whileFn = fn
	return t
}

// applyBackoff applies backoff delay if configured.
func (t *Target) applyBackoff(
	ctx context.Context,
	state *repetitionState,
	i, iterations int,
) error {
	if state.backoffDelay <= 0 || i >= iterations-1 {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("execution cancelled during backoff: %w", ctx.Err())
	case <-time.After(state.backoffDelay):
	}

	state.backoffDelay = time.Duration(float64(state.backoffDelay) * t.backoffMultiply)

	return nil
}

// cacheFilePath returns the path to the cache checksum internalfile.
func (t *Target) cacheFilePath() string {
	dir := t.cacheDir
	if dir == "" {
		dir = ".targ-cache"
	}

	// Generate a deterministic filename from the patterns
	hash := sha256.New()

	// Sort patterns for deterministic hashing
	patterns := make([]string, len(t.cache))
	copy(patterns, t.cache)
	sort.Strings(patterns)

	for _, p := range patterns {
		hash.Write([]byte(p))
		hash.Write([]byte{0})
	}

	filename := hex.EncodeToString(hash.Sum(nil))[:16] + ".sum"

	return dir + "/" + filename
}

// checkCache checks if cached files have changed.
// Returns true if files changed (cache miss), false if unchanged (cache hit).
func (t *Target) checkCache() (bool, error) {
	cacheFile := t.cacheFilePath()

	changed, err := internalfile.Checksum(
		t.cache,
		cacheFile,
		func(p []string) ([]string, error) { return internalfile.Match(p...) },
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("computing checksum: %w", err)
	}

	return changed, nil
}

// execute runs the target's function or shell command.
func (t *Target) execute(ctx context.Context, args []any) error {
	if t.fn == nil {
		// Deps-only target, nothing to execute
		return nil
	}

	switch fn := t.fn.(type) {
	case string:
		return runShellCommand(ctx, fn)
	default:
		return callFunc(ctx, fn, args)
	}
}

// iterationCount returns the number of iterations to run.
func (t *Target) iterationCount() int {
	if t.times > 0 {
		return t.times
	}

	return 1
}

// runDeps executes dependencies according to the configured mode.
func (t *Target) runDeps(ctx context.Context) error {
	if t.GetDepMode() == DepModeParallel {
		return t.runDepsParallel(ctx)
	}

	return t.runDepsSerial(ctx)
}

// runDepsParallel executes all dependencies concurrently.
// On first error, cancels remaining deps and returns immediately.
func (t *Target) runDepsParallel(ctx context.Context) error {
	deps := t.GetDeps()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(deps))

	for _, dep := range deps {
		go func(d *Target) {
			errs <- d.Run(ctx)
		}(dep)
	}

	var firstErr error

	for range deps {
		err := <-errs
		if err != nil && firstErr == nil {
			firstErr = err

			cancel() // Cancel remaining deps on first error
		}
	}

	return firstErr
}

// runDepsSerial executes dependencies one at a time in order.
func (t *Target) runDepsSerial(ctx context.Context) error {
	for _, dep := range t.GetDeps() {
		err := dep.Run(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// runOnce executes the target a single time with all configuration applied.
func (t *Target) runOnce(ctx context.Context, args []any) error {
	// Apply timeout if configured
	if t.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	// Run dependencies first
	if len(t.depGroups) > 0 {
		err := t.runDeps(ctx)
		if err != nil {
			return err
		}
	}

	// Check cache - if hit, skip execution
	if len(t.cache) > 0 {
		changed, err := t.checkCache()
		if err != nil {
			return fmt.Errorf("cache check failed: %w", err)
		}

		if !changed {
			// Cache hit - skip execution
			return nil
		}
	}

	// Execute the target with repetition handling
	return t.runWithRepetition(ctx, args)
}

// runWithRepetition handles Times, While, Retry, and Backoff logic.
func (t *Target) runWithRepetition(ctx context.Context, args []any) error {
	iterations := t.iterationCount()
	state := &repetitionState{backoffDelay: t.backoffInitial}

	for i := range iterations {
		if !t.shouldContinueLoop(ctx, state) {
			break
		}

		err := t.execute(ctx, args)
		if err != nil {
			state.lastErr = err

			if !t.retry {
				return err
			}

			err := t.applyBackoff(ctx, state, i, iterations)
			if err != nil {
				return err
			}
		} else {
			// Success - clear any previous error
			state.lastErr = nil

			// If retry is enabled, stop on first success
			if t.retry {
				break
			}
		}
	}

	return state.lastErr
}

// shouldContinueLoop checks if the loop should continue.
func (t *Target) shouldContinueLoop(ctx context.Context, state *repetitionState) bool {
	if t.whileFn != nil && !t.whileFn() {
		return false
	}

	select {
	case <-ctx.Done():
		if state.lastErr == nil {
			state.lastErr = fmt.Errorf("execution cancelled: %w", ctx.Err())
		}

		return false
	default:
		return true
	}
}

// Targ creates a Target from a function or shell command string.
//
// Function targets:
//
//	var build = core.Targ(Build)
//
// Shell command targets (run in user's shell):
//
//	var lint = core.Targ("golangci-lint run ./...")
//
// Deps-only targets (no function, just runs dependencies):
//
//	var all = core.Targ().Name("all").Deps(build, test, lint)
func Targ(fn ...any) *Target {
	if len(fn) == 0 {
		// Deps-only target with no function
		return &Target{}
	}

	if len(fn) > 1 {
		panic("targ.Targ: expected at most one argument")
	}

	f := fn[0]
	if f == nil {
		panic("targ.Targ: fn cannot be nil")
	}

	// Validate fn is a function or string
	switch v := f.(type) {
	case string:
		if v == "" {
			panic("targ.Targ: shell command cannot be empty")
		}
	default:
		fnValue := reflect.ValueOf(f)
		if fnValue.Kind() != reflect.Func {
			panic(fmt.Sprintf("targ.Targ: expected func or string, got %T", f))
		}
	}

	return &Target{fn: f}
}

// unexported constants.
const (
	depModeParallelStr = "parallel"
	depModeSerialStr   = "serial"
	depModeMixedStr    = "mixed"
)

type repetitionState struct {
	lastErr      error
	backoffDelay time.Duration
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
	err := internalsh.RunContextWithIO(ctx, nil, "sh", []string{"-c", cmd})
	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	return nil
}
