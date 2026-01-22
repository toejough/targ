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

// RuntimeOverrides holds CLI flags that override Target's compile-time configuration.
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

// TargetConfig holds compile-time configuration from a Target for conflict detection.
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
//nolint:gocognit,cyclop,funlen // Parsing multiple flag types requires branching for each
func ExtractOverrides(args []string) (RuntimeOverrides, []string, error) {
	var overrides RuntimeOverrides

	remaining := make([]string, 0, len(args))
	skip := false

	for i, arg := range args {
		if skip {
			skip = false

			continue
		}

		// --times N or --times=N
		if handled, err := handleTimesFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --retry
		if arg == "--retry" {
			overrides.Retry = true
			continue
		}

		// --watch PATTERN
		if handled, err := handleWatchFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --cache PATTERN
		if handled, err := handleCacheFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --cache-dir PATH
		if handled, err := handleCacheDirFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --backoff D,M
		if handled, err := handleBackoffFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --dep-mode MODE
		if handled, err := handleDepModeFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		// --while CMD
		if handled, err := handleWhileFlag(arg, args, i, &overrides, &skip); handled {
			if err != nil {
				return RuntimeOverrides{}, nil, err
			}

			continue
		}

		remaining = append(remaining, arg)
	}

	// Extract variadic --deps (must be done as a second pass because it consumes multiple args)
	var err error

	remaining, overrides.Deps, err = extractDepsVariadic(remaining)
	if err != nil {
		return RuntimeOverrides{}, nil, err
	}

	return overrides, remaining, nil
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

// unexported variables.
var (
	errBackoffInvalidFormat = errors.New(
		"--backoff format must be duration,multiplier (e.g., 1s,2.0)",
	)
	errBackoffRequiresValue = errors.New("--backoff requires a value")
	errCacheConflict        = errors.New(
		"--cache conflicts with target's cache configuration; use .Cache(targ.Disabled) to allow CLI override",
	)
	errCacheRequiresPattern = errors.New("--cache requires a pattern")
	errDepModeInvalid       = errors.New("--dep-mode must be 'serial' or 'parallel'")
	errDepModeRequiresValue = errors.New("--dep-mode requires a value")
	errTimesRequiresValue   = errors.New("--times requires a numeric value")
	errWatchConflict        = errors.New(
		"--watch conflicts with target's watch configuration; use .Watch(targ.Disabled) to allow CLI override",
	)
	errWatchRequiresPattern = errors.New("--watch requires a pattern")
	errWhileRequiresCommand = errors.New("--while requires a command")
	errDepsConflict         = errors.New(
		"--deps conflicts with target's dependency configuration; dependencies must be defined in one place (code or CLI)",
	)
	errDepsRequiresTarget   = errors.New("--deps requires at least one target")
	errCacheDirRequiresPath = errors.New("--cache-dir requires a path")
)

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

// executeOnce handles a single execution with cache, times, retry, while, and backoff.
//
//nolint:cyclop,funlen // Handles multiple execution modifiers (cache, times, retry, while, backoff)
func executeOnce(
	ctx context.Context,
	overrides RuntimeOverrides,
	cachePatterns []string,
	fn func() error,
) error {
	// Check cache first
	if len(cachePatterns) > 0 {
		cacheDir := overrides.CacheDir
		if cacheDir == "" {
			cacheDir = ".targ-cache"
		}

		cacheFile := cacheDir + "/override.sum"

		changed, err := file.Checksum(cachePatterns, cacheFile)
		if err != nil {
			return fmt.Errorf("cache check failed: %w", err)
		}

		if !changed {
			// Cache hit - skip execution
			return nil
		}
	}

	// Determine iteration count
	iterations := 1
	if overrides.Times > 0 {
		iterations = overrides.Times
	}

	var lastErr error

	backoffDelay := overrides.BackoffInitial

	for i := range iterations {
		// Check while condition
		if overrides.While != "" && !checkWhileCondition(ctx, overrides.While) {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			if lastErr == nil {
				return fmt.Errorf("execution cancelled: %w", ctx.Err())
			}

			return lastErr
		default:
		}

		// Execute the function
		err := fn()
		if err != nil {
			lastErr = err

			if !overrides.Retry {
				return err
			}

			// Apply backoff if configured
			if backoffDelay > 0 && i < iterations-1 {
				select {
				case <-ctx.Done():
					return fmt.Errorf("execution cancelled during backoff: %w", ctx.Err())
				case <-time.After(backoffDelay):
				}

				backoffDelay = time.Duration(float64(backoffDelay) * overrides.BackoffMultiplier)
			}
		} else {
			// Success - clear any previous error
			lastErr = nil
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

	// Watch for changes and re-run
	err = file.Watch(ctx, patterns, file.WatchOptions{}, func(_ file.ChangeSet) error {
		return fn()
	})
	if err != nil {
		return fmt.Errorf("watching files: %w", err)
	}

	return nil
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
