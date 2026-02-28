package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"runtime"
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
	case DepModeSerial:
		return depModeSerialStr
	case DepModeParallel:
		return depModeParallelStr
	case DepModeMixed:
		return depModeMixedStr
	default:
		return depModeSerialStr
	}
}

// DepOption is an option that modifies dependency execution behavior.
type DepOption int

// DepOption values.
const (
	// CollectAllErrors causes parallel deps to run all targets to completion
	// and collect all errors, rather than cancelling on first failure.
	CollectAllErrors DepOption = iota + 1
)

// DepGroup is the exported view of a dependency group.
type DepGroup struct {
	Targets    []*Target
	Mode       DepMode
	CollectAll bool
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

	// Lifecycle hooks for parallel output
	onStart func(ctx context.Context, name string)
	onStop  func(ctx context.Context, name string, result Result, duration time.Duration)

	// Source attribution
	sourcePkg      string // package that registered this target
	sourceFile     string // file that called Targ() (for string and deps-only targets)
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
	collectAll := false

	var targets []*Target

	for _, arg := range args {
		switch v := arg.(type) {
		case *Target:
			targets = append(targets, v)
		case DepMode:
			mode = v
		case DepOption:
			if v == CollectAllErrors {
				collectAll = true
			}
		}
	}

	if len(targets) == 0 {
		return t
	}

	// Coalesce with last group if same mode and same collectAll setting
	if len(t.depGroups) > 0 &&
		t.depGroups[len(t.depGroups)-1].mode == mode &&
		t.depGroups[len(t.depGroups)-1].collectAll == collectAll {
		t.depGroups[len(t.depGroups)-1].targets = append(
			t.depGroups[len(t.depGroups)-1].targets, targets...)
	} else {
		t.depGroups = append(
			t.depGroups,
			depGroup{targets: targets, mode: mode, collectAll: collectAll},
		)
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

// GetDepGroups returns the dependency groups with their execution modes.
func (t *Target) GetDepGroups() []DepGroup {
	groups := make([]DepGroup, len(t.depGroups))
	for i, g := range t.depGroups {
		groups[i] = DepGroup{Targets: g.targets, Mode: g.mode, CollectAll: g.collectAll}
	}

	return groups
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
	total := 0
	for _, g := range t.depGroups {
		total += len(g.targets)
	}

	all := make([]*Target, 0, total)
	for _, g := range t.depGroups {
		all = append(all, g.targets...)
	}

	return all
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

// GetOnStart returns the configured OnStart hook, or nil if not set.
func (t *Target) GetOnStart() func(ctx context.Context, name string) {
	return t.onStart
}

// GetOnStop returns the configured OnStop hook, or nil if not set.
func (t *Target) GetOnStop() func(ctx context.Context, name string, result Result, duration time.Duration) {
	return t.onStop
}

// GetRetry returns whether retry is enabled.
func (t *Target) GetRetry() bool {
	return t.retry
}

// GetSource returns the package path that registered this target.
func (t *Target) GetSource() string {
	return t.sourcePkg
}

// GetSourceFile returns the file path that called Targ() for string and deps-only targets.
func (t *Target) GetSourceFile() string {
	return t.sourceFile
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

// OnStart sets a hook that fires when the target begins execution in parallel mode.
func (t *Target) OnStart(fn func(ctx context.Context, name string)) *Target {
	t.onStart = fn
	return t
}

// OnStop sets a hook that fires when the target completes execution in parallel mode.
func (t *Target) OnStop(
	fn func(ctx context.Context, name string, result Result, duration time.Duration),
) *Target {
	t.onStop = fn
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
	for _, group := range t.depGroups {
		var err error

		switch {
		case group.mode == DepModeParallel && group.collectAll:
			err = runGroupParallelAll(ctx, group.targets)
		case group.mode == DepModeParallel:
			err = runGroupParallel(ctx, group.targets)
		default:
			err = runGroupSerial(ctx, group.targets)
		}

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

// RunContext executes a command with context support, routing output through
// the parallel printer when running in parallel mode.
func RunContext(ctx context.Context, name string, args ...string) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextWithIO(ctx, env, name, args)

	if pw != nil {
		pw.Flush()
	}

	return err
}

// RunContextV executes a command, prints it first, with context support.
// Routes output through the parallel printer when in parallel mode.
func RunContextV(ctx context.Context, name string, args ...string) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextV(ctx, env, name, args)

	if pw != nil {
		pw.Flush()
	}

	return err
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
		_, file, _, _ := runtime.Caller(1)
		return &Target{sourceFile: file}
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

		_, file, _, _ := runtime.Caller(1)

		return &Target{fn: f, sourceFile: file}
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
	depModeMixedStr    = "mixed"
	depModeParallelStr = "parallel"
	depModeSerialStr   = "serial"
)

type depGroup struct {
	targets    []*Target
	mode       DepMode
	collectAll bool
}

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

// classifyCollectAllResult determines the Result for collect-all mode.
// Unlike ClassifyResult, there is no "first failure" concept.
func classifyCollectAllResult(err error) Result {
	if err == nil {
		return Pass
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return Errored
	}

	return Fail
}

// parallelShellEnv returns a ShellEnv with PrefixWriter-wrapped stdout/stderr
// if running in parallel mode, or nil for serial mode.
func parallelShellEnv(ctx context.Context) (*internalsh.ShellEnv, *PrefixWriter) {
	info, ok := GetExecInfo(ctx)
	if !ok || !info.Parallel || info.Printer == nil {
		return nil, nil
	}

	prefix := FormatPrefix(info.Name, info.MaxNameLen)
	prefixWriter := NewPrefixWriter(prefix, info.Printer)

	env := internalsh.DefaultShellEnv()
	env.Stdout = prefixWriter
	env.Stderr = prefixWriter

	return env, prefixWriter
}

//nolint:cyclop,funlen // sequential pipeline with error handling at each step
func runGroupParallel(ctx context.Context, targets []*Target) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := outputFromContext(ctx)

	// Compute max target name length for prefix alignment
	maxNameLen := 0
	for _, dep := range targets {
		if n := len(dep.GetName()); n > maxNameLen {
			maxNameLen = n
		}
	}

	// Set up printer for parallel output
	const printerBufferMultiplier = 10

	printer := NewPrinter(out, len(targets)*printerBufferMultiplier)

	type targetResult struct {
		index    int
		err      error
		duration time.Duration
	}

	resultCh := make(chan targetResult, len(targets))
	results := make([]TargetResult, len(targets))

	for i, dep := range targets {
		name := dep.GetName()
		results[i].Name = name

		go func(idx int, d *Target, targetName string) {
			tctx := WithExecInfo(ctx, ExecInfo{
				Parallel:   true,
				Name:       targetName,
				MaxNameLen: maxNameLen,
				Printer:    printer,
				Output:     out,
			})

			// Fire OnStart hook (or default)
			if d.onStart != nil {
				d.onStart(tctx, targetName)
			} else {
				Print(tctx, "starting...\n")
			}

			start := time.Now()
			err := d.Run(tctx)
			duration := time.Since(start)

			resultCh <- targetResult{index: idx, err: err, duration: duration}
		}(i, dep, name)
	}

	var firstErr error

	firstErrIdx := -1

	for range targets {
		resultMsg := <-resultCh
		results[resultMsg.index].Err = resultMsg.err
		results[resultMsg.index].Duration = resultMsg.duration

		if resultMsg.err != nil && firstErr == nil {
			firstErr = resultMsg.err
			firstErrIdx = resultMsg.index

			cancel()
		}
	}

	// Classify results and fire OnStop hooks
	for i := range results {
		isFirst := i == firstErrIdx
		results[i].Status = ClassifyResult(results[i].Err, isFirst)

		name := results[i].Name
		tctx := WithExecInfo(ctx, ExecInfo{
			Parallel:   true,
			Name:       name,
			MaxNameLen: maxNameLen,
			Printer:    printer,
			Output:     out,
		})
		target := targets[i]

		// Print error text with prefix before the stop message
		if results[i].Err != nil && !errors.Is(results[i].Err, context.Canceled) {
			Printf(tctx, "Error: %v\n", results[i].Err)
		}

		if target.onStop != nil {
			target.onStop(tctx, name, results[i].Status, results[i].Duration)
		} else {
			Printf(
				tctx,
				"%s (%s)\n",
				results[i].Status,
				results[i].Duration.Round(time.Millisecond),
			)
		}
	}

	// Drain printer and print summary
	printer.Close()

	summary := FormatSummary(results)
	if summary != "" {
		_, _ = fmt.Fprintln(out, "\n"+summary)
	}

	if firstErr != nil {
		return reportedError{err: firstErr}
	}

	return nil
}

//nolint:cyclop,funlen // sequential pipeline with error handling at each step
func runGroupParallelAll(ctx context.Context, targets []*Target) error {
	out := outputFromContext(ctx)

	// Compute max target name length for prefix alignment
	maxNameLen := 0
	for _, dep := range targets {
		if n := len(dep.GetName()); n > maxNameLen {
			maxNameLen = n
		}
	}

	// Set up printer for parallel output
	const printerBufferMultiplier = 10

	printer := NewPrinter(out, len(targets)*printerBufferMultiplier)

	type targetResult struct {
		index    int
		err      error
		duration time.Duration
	}

	resultCh := make(chan targetResult, len(targets))
	results := make([]TargetResult, len(targets))

	for i, dep := range targets {
		name := dep.GetName()
		results[i].Name = name

		go func(idx int, d *Target, targetName string) {
			tctx := WithExecInfo(ctx, ExecInfo{
				Parallel:   true,
				Name:       targetName,
				MaxNameLen: maxNameLen,
				Printer:    printer,
				Output:     out,
			})

			// Fire OnStart hook (or default)
			if d.onStart != nil {
				d.onStart(tctx, targetName)
			} else {
				Print(tctx, "starting...\n")
			}

			start := time.Now()
			err := d.Run(tctx)
			duration := time.Since(start)

			resultCh <- targetResult{index: idx, err: err, duration: duration}
		}(i, dep, name)
	}

	// Collect ALL results — no cancellation
	for range targets {
		resultMsg := <-resultCh
		results[resultMsg.index].Err = resultMsg.err
		results[resultMsg.index].Duration = resultMsg.duration
	}

	// Classify results — no "first failure" distinction in collect-all mode
	hasFailure := false

	for i := range results {
		results[i].Status = classifyCollectAllResult(results[i].Err)

		if results[i].Status != Pass {
			hasFailure = true
		}

		name := results[i].Name
		tctx := WithExecInfo(ctx, ExecInfo{
			Parallel:   true,
			Name:       name,
			MaxNameLen: maxNameLen,
			Printer:    printer,
			Output:     out,
		})
		target := targets[i]

		// Print error text with prefix before the stop message
		if results[i].Err != nil && !errors.Is(results[i].Err, context.Canceled) {
			Printf(tctx, "Error: %v\n", results[i].Err)
		}

		if target.onStop != nil {
			target.onStop(tctx, name, results[i].Status, results[i].Duration)
		} else {
			Printf(
				tctx,
				"%s (%s)\n",
				results[i].Status,
				results[i].Duration.Round(time.Millisecond),
			)
		}
	}

	// Drain printer and print detailed summary
	printer.Close()

	summary := FormatDetailedSummary(results)
	if summary != "" {
		_, _ = fmt.Fprintln(out, "\n"+summary)
	}

	if hasFailure {
		return reportedError{err: NewMultiError(results)}
	}

	return nil
}

func runGroupSerial(ctx context.Context, targets []*Target) error {
	for _, dep := range targets {
		err := dep.Run(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// runShellCommand executes a shell command string.
// The command is run via the user's shell (sh -c on Unix).
// In parallel mode, stdout/stderr are routed through a PrefixWriter.
func runShellCommand(ctx context.Context, cmd string) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextWithIO(ctx, env, "sh", []string{"-c", cmd})

	if pw != nil {
		pw.Flush()
	}

	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	return nil
}
