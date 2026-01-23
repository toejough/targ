package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/toejough/targ/file"
)

// RuntimeOverrides holds CLI flags that override Target compile-time settings.
type RuntimeOverrides struct {
	Times             int           // Number of times to run (--times N)
	Retry             bool          // Continue on failure (--retry)
	Watch             []string      // File patterns to watch (--watch "pattern")
	Cache             []string      // File patterns for caching (--cache "pattern")
	CacheDir          string        // Directory for cache files (--cache-dir "path")
	BackoffInitial    time.Duration // Initial backoff delay (--backoff D,M)
	BackoffMultiplier float64       // Backoff multiplier
	DepMode           string        // Dependency mode: serial or parallel (--dep-mode)
	While             string        // Shell command to check (--while "cmd")
	Deps              []string      // Dependency target paths (--deps target1 target2)
	Parallel          bool          // Run multiple targets concurrently (--parallel or -p)
}

// hasAny returns true if any override is set.
func (o RuntimeOverrides) hasAny() bool {
	return o.Times > 0 ||
		o.Retry ||
		len(o.Watch) > 0 ||
		len(o.Cache) > 0 ||
		o.BackoffInitial > 0 ||
		o.While != "" ||
		len(o.Deps) > 0
}

// TargetConfig holds compile-time configuration from a Target definition.
type TargetConfig struct {
	WatchPatterns []string
	CachePatterns []string
	WatchDisabled bool // True if target explicitly allows CLI --watch
	CacheDisabled bool // True if target explicitly allows CLI --cache
	HasDeps       bool // True if target has .Deps() or .ParallelDeps() configured
}

// ExecuteWithOverrides runs a function with runtime overrides applied.
// The fn argument should be a function that executes the command once.
// The config parameter holds compile-time Target configuration for conflict detection.
func ExecuteWithOverrides(
	ctx context.Context,
	overrides RuntimeOverrides,
	config TargetConfig,
	fn func() error,
) error {
	// Check for conflicts between CLI overrides and Target config
	err := checkConflicts(overrides, config)
	if err != nil {
		return err
	}

	// If no overrides are active and no compile-time config, just run the function
	if !overrides.hasAny() && len(config.CachePatterns) == 0 {
		return fn()
	}

	// Merge cache patterns: CLI overrides take precedence if Target allows (disabled)
	// Otherwise use Target's patterns (conflict was checked above)
	allCachePatterns := overrides.Cache
	if len(config.CachePatterns) > 0 && len(overrides.Cache) == 0 {
		allCachePatterns = config.CachePatterns
	}

	// Merge watch patterns similarly
	allWatchPatterns := overrides.Watch
	if len(config.WatchPatterns) > 0 && len(overrides.Watch) == 0 {
		allWatchPatterns = config.WatchPatterns
	}

	// Create the execution function that handles cache, times, retry, etc.
	execFn := func() error {
		return executeOnce(ctx, overrides, allCachePatterns, fn)
	}

	// If watch mode is enabled (from CLI or Target), wrap in watch loop
	if len(allWatchPatterns) > 0 {
		return executeWithWatch(ctx, allWatchPatterns, execFn)
	}

	return execFn()
}

// ExtractOverrides parses runtime override flags from args.
// Returns the overrides, remaining args, and any error.
//
// Position-sensitive flags (like --parallel, -p) are only recognized when they
// appear BEFORE the first target name. After a target name is seen,
// these flags are passed through to targets.
func ExtractOverrides(args []string) (RuntimeOverrides, []string, error) {
	var overrides RuntimeOverrides

	remaining := make([]string, 0, len(args))
	skip := false
	seenTarget := false

	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		// Try flag handlers
		if handled, err := processOverrideFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// Position-sensitive flags (only before first target)
		if !seenTarget && isParallelFlag(arg) {
			overrides.Parallel = true
			continue
		}

		// Track when we see a target name (non-flag after first arg)
		if !strings.HasPrefix(arg, "-") && i > 0 {
			seenTarget = true
		}

		remaining = append(remaining, arg)
	}

	var err error

	remaining, overrides.Deps, err = extractDepsVariadic(remaining)
	if err != nil {
		return RuntimeOverrides{}, nil, err
	}

	return overrides, remaining, nil
}

// unexported variables.
var (
	errBackoffInvalidFormat = errors.New(
		"--backoff format must be duration,multiplier (e.g., 1s,2.0)",
	)
	errBackoffRequiresValue = errors.New("--backoff requires a value")
	errCacheConflict        = errors.New(
		"--cache conflicts with target's cache configuration; use .Cache(targ.Disabled) to allow CLI override",
	)
	errCacheDirRequiresPath = errors.New("--cache-dir requires a path")
	errCacheRequiresPattern = errors.New("--cache requires a pattern")
	errDepModeInvalid       = errors.New("--dep-mode must be 'serial' or 'parallel'")
	errDepModeRequiresValue = errors.New("--dep-mode requires a value")
	errDepsConflict         = errors.New(
		"--deps conflicts with target's dependency configuration; dependencies must be defined in one place (code or CLI)",
	)
	errDepsRequiresTarget = errors.New("--deps requires at least one target")
	errTimesRequiresValue = errors.New("--times requires a numeric value")
	errWatchConflict      = errors.New(
		"--watch conflicts with target's watch configuration; use .Watch(targ.Disabled) to allow CLI override",
	)
	errWatchRequiresPattern = errors.New("--watch requires a pattern")
	errWhileRequiresCommand = errors.New("--while requires a command")
	//nolint:gochecknoglobals // DI injection point for testing
	fileWatch = file.Watch
)

// overrideFlagHandler is a function that handles an override flag.
// Returns (handled, error). Sets skip to true if next arg was consumed.
type overrideFlagHandler func(arg string, args []string, i int, o *RuntimeOverrides, skip *bool) (bool, error)

// executeIteration runs a single iteration of the function with retry/backoff handling.
// Returns the updated backoff delay, whether to continue, and the error (if any).
// If prevErr is non-nil and context is cancelled, prevErr is returned to preserve the original error.
// applyBackoffSleep sleeps for the backoff delay if applicable, returning the new delay.
// Returns an error if context is cancelled during sleep.
func applyBackoffSleep(
	ctx context.Context,
	delay time.Duration,
	multiplier float64,
	iteration, total int,
) (time.Duration, error) {
	if delay <= 0 || iteration >= total-1 {
		return delay, nil
	}

	select {
	case <-ctx.Done():
		return delay, fmt.Errorf("execution cancelled during backoff: %w", ctx.Err())
	case <-time.After(delay):
	}

	return time.Duration(float64(delay) * multiplier), nil
}

// checkCacheHit returns true if cache is valid (skip execution).
func checkCacheHit(patterns []string, cacheDir string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}

	if cacheDir == "" {
		cacheDir = ".targ-cache"
	}

	changed, err := file.Checksum(patterns, cacheDir+"/override.sum")
	if err != nil {
		return false, fmt.Errorf("cache check failed: %w", err)
	}

	return !changed, nil
}

// checkConflicts verifies CLI overrides don't conflict with Target config.
func checkConflicts(overrides RuntimeOverrides, config TargetConfig) error {
	// Check watch conflict: CLI --watch vs Target.Watch()
	if len(overrides.Watch) > 0 && len(config.WatchPatterns) > 0 && !config.WatchDisabled {
		return errWatchConflict
	}

	// Check cache conflict: CLI --cache vs Target.Cache()
	if len(overrides.Cache) > 0 && len(config.CachePatterns) > 0 && !config.CacheDisabled {
		return errCacheConflict
	}

	// Check deps conflict: CLI --deps vs Target.Deps()/ParallelDeps()
	// Note: There's no "disabled" option for deps - they must be defined in one place only
	if len(overrides.Deps) > 0 && config.HasDeps {
		return errDepsConflict
	}

	return nil
}

// checkWhileCondition runs a shell command and returns true if it succeeds.
func checkWhileCondition(ctx context.Context, cmd string) bool {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)

	return c.Run() == nil
}

func executeIteration(
	ctx context.Context,
	fn func() error,
	overrides RuntimeOverrides,
	iteration, totalIterations int,
	backoffDelay time.Duration,
	prevErr error,
) (newBackoff time.Duration, shouldContinue bool, lastErr error) {
	// Check while condition
	if overrides.While != "" && !checkWhileCondition(ctx, overrides.While) {
		return backoffDelay, false, nil
	}

	// Check context cancellation - return previous error if any
	select {
	case <-ctx.Done():
		if prevErr != nil {
			return backoffDelay, false, prevErr
		}

		return backoffDelay, false, fmt.Errorf("execution cancelled: %w", ctx.Err())
	default:
	}

	err := fn()
	if err == nil {
		return backoffDelay, true, nil
	}

	if !overrides.Retry {
		return backoffDelay, false, err
	}

	// Apply backoff if configured
	newDelay, backoffErr := applyBackoffSleep(
		ctx, backoffDelay, overrides.BackoffMultiplier, iteration, totalIterations,
	)
	if backoffErr != nil {
		return backoffDelay, false, backoffErr
	}

	return newDelay, true, err
}

// executeOnce handles a single execution with cache, times, retry, while, and backoff.
func executeOnce(
	ctx context.Context,
	overrides RuntimeOverrides,
	cachePatterns []string,
	fn func() error,
) error {
	cacheHit, err := checkCacheHit(cachePatterns, overrides.CacheDir)
	if err != nil {
		return err
	}

	if cacheHit {
		return nil
	}

	iterations := 1
	if overrides.Times > 0 {
		iterations = overrides.Times
	}

	var lastErr error

	backoffDelay := overrides.BackoffInitial

	for i := range iterations {
		var shouldContinue bool

		backoffDelay, shouldContinue, lastErr = executeIteration(
			ctx, fn, overrides, i, iterations, backoffDelay, lastErr,
		)

		if !shouldContinue {
			break
		}
	}

	return lastErr
}

// executeWithWatch runs the function and re-runs on file changes.
func executeWithWatch(
	ctx context.Context,
	patterns []string,
	fn func() error,
) error {
	// Run once initially
	err := fn()
	if err != nil {
		return err
	}

	// Watch for changes and re-run. Watch runs until error (including context cancel).
	return fmt.Errorf(
		"watching files: %w",
		fileWatch(ctx, patterns, file.WatchOptions{}, func(_ file.ChangeSet) error {
			return fn()
		}),
	)
}

// extractDepsVariadic extracts --deps and its variadic arguments.
// Deps values continue until the next flag (--something) or path reset (--).
func extractDepsVariadic(args []string) ([]string, []string, error) {
	var deps []string

	remaining := make([]string, 0, len(args))

	inDeps := false

	for _, arg := range args {
		if inDeps {
			// Check if this ends the variadic sequence
			if arg == "--" || strings.HasPrefix(arg, "-") {
				// If we hit a flag/-- but have no deps collected, that's an error
				if len(deps) == 0 {
					return nil, nil, errDepsRequiresTarget
				}

				// End of deps sequence
				inDeps = false

				// If it's --, it's a path reset - add to remaining for the path parser
				// Other flags also go to remaining
				remaining = append(remaining, arg)

				continue
			}
			// Add to deps
			deps = append(deps, arg)

			continue
		}

		if arg == "--deps" {
			inDeps = true

			continue
		}

		remaining = append(remaining, arg)
	}

	// If we ended while in deps mode without any deps, that's an error
	if inDeps && len(deps) == 0 {
		return nil, nil, errDepsRequiresTarget
	}

	return remaining, deps, nil
}

func handleBackoffFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--backoff" {
		if index+1 >= len(args) {
			return true, errBackoffRequiresValue
		}

		initial, multiplier, err := parseBackoffValue(args[index+1])
		if err != nil {
			return true, err
		}

		overrides.BackoffInitial = initial
		overrides.BackoffMultiplier = multiplier
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--backoff="); ok {
		initial, multiplier, err := parseBackoffValue(after)
		if err != nil {
			return true, err
		}

		overrides.BackoffInitial = initial
		overrides.BackoffMultiplier = multiplier

		return true, nil
	}

	return false, nil
}

func handleCacheDirFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--cache-dir" {
		if index+1 >= len(args) {
			return true, errCacheDirRequiresPath
		}

		overrides.CacheDir = args[index+1]
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--cache-dir="); ok {
		overrides.CacheDir = after
		return true, nil
	}

	return false, nil
}

func handleCacheFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--cache" {
		if index+1 >= len(args) {
			return true, errCacheRequiresPattern
		}

		overrides.Cache = append(overrides.Cache, args[index+1])
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--cache="); ok {
		overrides.Cache = append(overrides.Cache, after)
		return true, nil
	}

	return false, nil
}

func handleDepModeFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--dep-mode" {
		if index+1 >= len(args) {
			return true, errDepModeRequiresValue
		}

		mode := args[index+1]
		if mode != "serial" && mode != "parallel" {
			return true, errDepModeInvalid
		}

		overrides.DepMode = mode
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--dep-mode="); ok {
		if after != "serial" && after != "parallel" {
			return true, errDepModeInvalid
		}

		overrides.DepMode = after

		return true, nil
	}

	return false, nil
}

func handleRetryFlag(
	arg string,
	_ []string,
	_ int,
	overrides *RuntimeOverrides,
	_ *bool,
) (bool, error) {
	if arg == "--retry" {
		overrides.Retry = true
		return true, nil
	}

	return false, nil
}

func handleTimesFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--times" {
		if index+1 >= len(args) {
			return true, errTimesRequiresValue
		}

		n, err := strconv.Atoi(args[index+1])
		if err != nil {
			return true, fmt.Errorf("invalid --times value %q: %w", args[index+1], err)
		}

		overrides.Times = n
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--times="); ok {
		n, err := strconv.Atoi(after)
		if err != nil {
			return true, fmt.Errorf("invalid --times value %q: %w", after, err)
		}

		overrides.Times = n

		return true, nil
	}

	return false, nil
}

func handleWatchFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--watch" {
		if index+1 >= len(args) {
			return true, errWatchRequiresPattern
		}

		overrides.Watch = append(overrides.Watch, args[index+1])
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--watch="); ok {
		overrides.Watch = append(overrides.Watch, after)
		return true, nil
	}

	return false, nil
}

func handleWhileFlag(
	arg string,
	args []string,
	index int,
	overrides *RuntimeOverrides,
	skip *bool,
) (bool, error) {
	if arg == "--while" {
		if index+1 >= len(args) {
			return true, errWhileRequiresCommand
		}

		overrides.While = args[index+1]
		*skip = true

		return true, nil
	}

	if after, ok := strings.CutPrefix(arg, "--while="); ok {
		overrides.While = after
		return true, nil
	}

	return false, nil
}

func isParallelFlag(arg string) bool {
	return arg == "--parallel" || arg == "-p"
}

// overrideFlagHandlers returns the list of flag handlers for ExtractOverrides.
func overrideFlagHandlers() []overrideFlagHandler {
	return []overrideFlagHandler{
		handleTimesFlag,
		handleWatchFlag,
		handleCacheFlag,
		handleCacheDirFlag,
		handleRetryFlag,
		handleBackoffFlag,
		handleDepModeFlag,
		handleWhileFlag,
	}
}

func parseBackoffValue(val string) (time.Duration, float64, error) {
	parts := strings.SplitN(val, ",", 2) //nolint:mnd // backoff has exactly 2 parts
	if len(parts) != 2 {                 //nolint:mnd // backoff has exactly 2 parts
		return 0, 0, errBackoffInvalidFormat
	}

	duration, err := time.ParseDuration(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid backoff duration %q: %w", parts[0], err)
	}

	multiplier, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid backoff multiplier %q: %w", parts[1], err)
	}

	return duration, multiplier, nil
}

// processOverrideFlag tries each handler until one succeeds.
func processOverrideFlag(
	arg string,
	args []string,
	i int,
	o *RuntimeOverrides,
	skip *bool,
) (handled bool, err error) {
	for _, handler := range overrideFlagHandlers() {
		handled, err = handler(arg, args, i, o, skip)
		if handled {
			return handled, err
		}
	}

	return false, nil
}
