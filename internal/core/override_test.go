package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestExecuteWithOverrides_BackoffDelay(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	start := time.Now()
	overrides := core.RuntimeOverrides{
		Times:             3,
		Retry:             true,
		BackoffInitial:    10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	err := core.ExecuteWithOverrides(ctx, overrides, core.TargetConfig{}, func() error {
		count++
		return errors.New("fail")
	})

	elapsed := time.Since(start)

	g.Expect(err).To(HaveOccurred())
	g.Expect(count).To(Equal(3))
	// Should have waited ~10ms + ~20ms = ~30ms between retries
	g.Expect(elapsed).To(BeNumerically(">=", 25*time.Millisecond))
}

func TestExecuteWithOverrides_CacheConflict(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	// Target has cache patterns configured
	config := core.TargetConfig{
		CachePatterns: []string{"**/*.go"},
		CacheDisabled: false,
	}

	// CLI also specifies --cache, which should conflict
	overrides := core.RuntimeOverrides{
		Cache: []string{"**/*.ts"},
	}

	err := core.ExecuteWithOverrides(ctx, overrides, config, func() error {
		return nil
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("--cache conflicts")))
	g.Expect(err).To(MatchError(ContainSubstring("targ.Disabled")))
}

func TestExecuteWithOverrides_CacheDisabledAllowsOverride(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	// Target has cache disabled (allows CLI control)
	config := core.TargetConfig{
		CachePatterns: []string{"**/*.go"}, // These are ignored when disabled
		CacheDisabled: true,
	}

	// CLI specifies --cache, which should be allowed
	overrides := core.RuntimeOverrides{
		Cache: []string{"nonexistent/**"}, // Patterns that won't match anything
	}

	err := core.ExecuteWithOverrides(ctx, overrides, config, func() error {
		return nil
	})

	// No conflict error - function ran (cache check may error but not conflict)
	// The cache check will fail because files don't exist, but that's different
	// from a conflict error
	g.Expect(err).To(Or(Not(HaveOccurred()), MatchError(ContainSubstring("cache check"))))
}

func TestExecuteWithOverrides_CancelDuringBackoff(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	overrides := core.RuntimeOverrides{
		Times:             3,
		Retry:             true,
		BackoffInitial:    500 * time.Millisecond, // Long delay so we can cancel during it
		BackoffMultiplier: 1.0,
	}

	// Cancel after first execution
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := core.ExecuteWithOverrides(ctx, overrides, core.TargetConfig{}, func() error {
		count++
		return errors.New("fail")
	})

	g.Expect(err).To(MatchError(ContainSubstring("cancelled during backoff")))
	g.Expect(count).To(Equal(1)) // Only ran once before cancel during backoff
}

func TestExecuteWithOverrides_CancelWithPreviousError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	overrides := core.RuntimeOverrides{
		Times: 5,
		Retry: true,
	}

	err := core.ExecuteWithOverrides(ctx, overrides, core.TargetConfig{}, func() error {
		count++
		if count == 2 {
			cancel() // Cancel after second run
		}

		return errors.New("fail")
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("fail"))) // Returns lastErr, not cancelled
	g.Expect(count).To(Equal(2))
}

func TestExecuteWithOverrides_DepsConflict(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	// Target has deps configured
	config := core.TargetConfig{
		HasDeps: true,
	}

	// CLI also specifies --deps, which should conflict
	overrides := core.RuntimeOverrides{
		Deps: []string{"lint", "test"},
	}

	err := core.ExecuteWithOverrides(ctx, overrides, config, func() error {
		return nil
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("--deps conflicts")))
}

func TestExecuteWithOverrides_NoConflictWhenTargetHasNoConfig(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	// Target has no configuration
	config := core.TargetConfig{}

	// CLI specifies --watch (should work because target has no watch config)
	overrides := core.RuntimeOverrides{
		Watch: []string{"**/*.go"},
	}

	err := core.ExecuteWithOverrides(ctx, overrides, config, func() error {
		count++

		cancel() // Cancel after first run to exit watch loop

		return nil
	})

	// No conflict error - watch was cancelled (returns context error)
	g.Expect(err).To(Or(
		Not(HaveOccurred()),
		MatchError(ContainSubstring("context canceled")),
	))
	g.Expect(count).To(Equal(1))
}

// Ownership model tests - conflict detection

func TestExecuteWithOverrides_WatchInitialError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	testErr := errors.New("initial execution failed")
	config := core.TargetConfig{}
	overrides := core.RuntimeOverrides{
		Watch: []string{"**/*.go"},
	}

	// Function returns error on initial execution - should not start watch
	err := core.ExecuteWithOverrides(ctx, overrides, config, func() error {
		return testErr
	})

	g.Expect(err).To(MatchError(ContainSubstring("initial execution failed")))
}

func TestExecuteWithOverrides_WhileStopsOnFalse(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	// Use a while command that will fail (non-zero exit)
	overrides := core.RuntimeOverrides{Times: 10, While: "false"}

	err := core.ExecuteWithOverrides(ctx, overrides, core.TargetConfig{}, func() error {
		count++
		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(0)) // While condition fails immediately
}

func TestExtractOverrides_Backoff(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--backoff", "1s,2.0"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.BackoffInitial).To(Equal(time.Second))
	g.Expect(overrides.BackoffMultiplier).To(Equal(2.0))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_BackoffEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--backoff=500ms,1.5"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.BackoffInitial).To(Equal(500 * time.Millisecond))
	g.Expect(overrides.BackoffMultiplier).To(Equal(1.5))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_BackoffEqualsInvalidDuration(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--backoff=bad,2.0"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("duration"))
}

func TestExtractOverrides_Cache(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--cache", "**/*.go"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Cache).To(Equal([]string{"**/*.go"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_CacheDir(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--cache-dir", "/tmp/cache"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.CacheDir).To(Equal("/tmp/cache"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_CacheDirEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--cache-dir=.my-cache"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.CacheDir).To(Equal(".my-cache"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_CacheEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--cache=lib/**/*.js"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Cache).To(Equal([]string{"lib/**/*.js"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_Combined(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--times", "3", "--retry", "--watch", "**/*.go"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Times).To(Equal(3))
	g.Expect(overrides.Retry).To(BeTrue())
	g.Expect(overrides.Watch).To(Equal([]string{"**/*.go"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_DepMode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--dep-mode", "parallel"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.DepMode).To(Equal("parallel"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_DepModeEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--dep-mode=parallel"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.DepMode).To(Equal("parallel"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_Deps(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--deps", "lint", "test"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Deps).To(Equal([]string{"lint", "test"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_DepsEmptyBeforeFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--deps", "--timeout", "5m"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

func TestExtractOverrides_TimesEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--times=10"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Times).To(Equal(10))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WatchEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--watch=src/**/*.ts"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Watch).To(Equal([]string{"src/**/*.ts"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_While(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--while", "test -f lockfile"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.While).To(Equal("test -f lockfile"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WhileEquals(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	args := []string{"build", "--while=pgrep -x myapp"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.While).To(Equal("pgrep -x myapp"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}
