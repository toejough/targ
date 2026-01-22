package core_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestExecuteWithOverrides_BackoffDelay(t *testing.T) {
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

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return errors.New("fail")
	})

	elapsed := time.Since(start)

	g.Expect(err).To(HaveOccurred())
	g.Expect(count).To(Equal(3))
	// Should have waited ~10ms + ~20ms = ~30ms between retries
	g.Expect(elapsed).To(BeNumerically(">=", 25*time.Millisecond))
}

func TestExecuteWithOverrides_CancelDuringBackoff(t *testing.T) {
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

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return errors.New("fail")
	})

	g.Expect(err).To(MatchError(ContainSubstring("cancelled during backoff")))
	g.Expect(count).To(Equal(1)) // Only ran once before cancel during backoff
}

func TestExecuteWithOverrides_CancelWithPreviousError(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	overrides := core.RuntimeOverrides{
		Times: 5,
		Retry: true,
	}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
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

func TestExecuteWithOverrides_ContextCancellation(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	count := 0
	overrides := core.RuntimeOverrides{Times: 5}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return nil
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("cancelled")))
	g.Expect(count).To(Equal(0)) // Doesn't run because context cancelled
}

func TestExecuteWithOverrides_NoOverrides(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	overrides := core.RuntimeOverrides{} // Empty overrides

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(1)) // Runs exactly once
}

func TestExecuteWithOverrides_RetryAllFails(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	overrides := core.RuntimeOverrides{Times: 3, Retry: true}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return errors.New("always fail")
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(ContainSubstring("always fail")))
	g.Expect(count).To(Equal(3)) // Runs all times even though all fail
}

// Integration tests for ExecuteWithOverrides

func TestExecuteWithOverrides_Times(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	overrides := core.RuntimeOverrides{Times: 3}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(3))
}

func TestExecuteWithOverrides_TimesProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)
		ctx := context.Background()
		times := rapid.IntRange(1, 20).Draw(rt, "times")

		count := 0
		overrides := core.RuntimeOverrides{Times: times}

		err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
			count++
			return nil
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(times))
	})
}

func TestExecuteWithOverrides_TimesWithError(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	overrides := core.RuntimeOverrides{Times: 5}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		if count == 2 {
			return errors.New("fail")
		}

		return nil
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(count).To(Equal(2)) // Stops on first error without retry
}

func TestExecuteWithOverrides_TimesWithRetry(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	overrides := core.RuntimeOverrides{Times: 5, Retry: true}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		if count < 3 {
			return errors.New("fail")
		}

		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(5)) // Continues all iterations with retry
}

func TestExecuteWithOverrides_WhileStopsOnFalse(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	// Use a while command that will fail (non-zero exit)
	overrides := core.RuntimeOverrides{Times: 10, While: "false"}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(0)) // While condition fails immediately
}

func TestExecuteWithOverrides_WhileSucceeds(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	count := 0
	// Use a while command that succeeds (exit 0)
	overrides := core.RuntimeOverrides{Times: 3, While: "true"}

	err := core.ExecuteWithOverrides(ctx, overrides, nil, func() error {
		count++
		return nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(count).To(Equal(3)) // Runs all 3 times because while succeeds
}

func TestExtractOverrides_Backoff(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff", "1s,2.0"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.BackoffInitial).To(Equal(time.Second))
	g.Expect(overrides.BackoffMultiplier).To(Equal(2.0))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_BackoffEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff=500ms,1.5"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.BackoffInitial).To(Equal(500 * time.Millisecond))
	g.Expect(overrides.BackoffMultiplier).To(Equal(1.5))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_BackoffEqualsInvalidDuration(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff=bad,2.0"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("duration"))
}

func TestExtractOverrides_BackoffEqualsInvalidMultiplier(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff=1s,bad"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("multiplier"))
}

func TestExtractOverrides_BackoffEqualsMissingComma(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff=1s"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("format"))
}

func TestExtractOverrides_BackoffInvalidDuration(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff", "bad,2.0"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("duration"))
}

func TestExtractOverrides_BackoffInvalidMultiplier(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff", "1s,bad"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("multiplier"))
}

func TestExtractOverrides_BackoffMissingComma(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff", "1s"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("format"))
}

func TestExtractOverrides_BackoffMissingValue(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--backoff"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

func TestExtractOverrides_Cache(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--cache", "**/*.go"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Cache).To(Equal([]string{"**/*.go"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_CacheEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--cache=lib/**/*.js"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Cache).To(Equal([]string{"lib/**/*.js"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_CacheMissing(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--cache"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

func TestExtractOverrides_CacheMultiple(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--cache", "src/**", "--cache", "pkg/**"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Cache).To(Equal([]string{"src/**", "pkg/**"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_Combined(t *testing.T) {
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
	g := NewWithT(t)

	args := []string{"build", "--dep-mode", "parallel"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.DepMode).To(Equal("parallel"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_DepModeEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--dep-mode=parallel"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.DepMode).To(Equal("parallel"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_DepModeEqualsInvalid(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--dep-mode=bad"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("parallel"))
}

func TestExtractOverrides_DepModeInvalid(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--dep-mode", "invalid"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("parallel"))
}

func TestExtractOverrides_DepModeMissing(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--dep-mode"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

func TestExtractOverrides_DepModeSerial(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--dep-mode", "serial"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.DepMode).To(Equal("serial"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_NoOverrides(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--verbose", "arg1"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Times).To(Equal(0))
	g.Expect(overrides.Retry).To(BeFalse())
	g.Expect(overrides.Watch).To(BeNil())
	g.Expect(remaining).To(Equal([]string{"build", "--verbose", "arg1"}))
}

func TestExtractOverrides_Retry(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--retry", "arg1"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Retry).To(BeTrue())
	g.Expect(remaining).To(Equal([]string{"build", "arg1"}))
}

func TestExtractOverrides_Times(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--times", "5", "arg1"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Times).To(Equal(5))
	g.Expect(remaining).To(Equal([]string{"build", "arg1"}))
}

func TestExtractOverrides_TimesEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--times=10"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Times).To(Equal(10))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_TimesInvalid(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--times", "abc"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid"))
}

func TestExtractOverrides_TimesMissing(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--times"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

// Property-based tests
func TestExtractOverrides_TimesProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)
		times := rapid.IntRange(1, 100).Draw(rt, "times")

		args := []string{"build", "--times", strconv.Itoa(times), "arg1"}
		overrides, _, err := core.ExtractOverrides(args)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(overrides.Times).To(Equal(times))
	})
}

func TestExtractOverrides_Watch(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--watch", "**/*.go", "arg1"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Watch).To(Equal([]string{"**/*.go"}))
	g.Expect(remaining).To(Equal([]string{"build", "arg1"}))
}

func TestExtractOverrides_WatchEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--watch=src/**/*.ts"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Watch).To(Equal([]string{"src/**/*.ts"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WatchMissing(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--watch"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}

func TestExtractOverrides_WatchMultiple(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--watch", "**/*.go", "--watch", "**/*.mod"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.Watch).To(Equal([]string{"**/*.go", "**/*.mod"}))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WatchPatternsPreserved(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)
		// Generate valid glob patterns
		pattern := rapid.StringMatching(`[a-z]+/\*\*\.[a-z]+`).Draw(rt, "pattern")

		args := []string{"build", "--watch", pattern}
		overrides, _, err := core.ExtractOverrides(args)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(overrides.Watch).To(ContainElement(pattern))
	})
}

func TestExtractOverrides_While(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--while", "test -f lockfile"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.While).To(Equal("test -f lockfile"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WhileEquals(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--while=pgrep -x myapp"}
	overrides, remaining, err := core.ExtractOverrides(args)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(overrides.While).To(Equal("pgrep -x myapp"))
	g.Expect(remaining).To(Equal([]string{"build"}))
}

func TestExtractOverrides_WhileMissing(t *testing.T) {
	g := NewWithT(t)

	args := []string{"build", "--while"}
	_, _, err := core.ExtractOverrides(args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires"))
}
