package targ_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestProperty_Overrides(t *testing.T) {
	t.Parallel()

	// Helper to create a dummy second target for multi-root mode
	// In multi-root mode, target names must be specified explicitly
	dummy := func() *targ.Target { return targ.Targ(func() {}).Name("_dummy") }

	t.Run("TimeoutFlagEnforcesLimit", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		started := make(chan struct{})
		target := targ.Targ(func(ctx context.Context) error {
			close(started)
			<-ctx.Done() // Wait for timeout

			return ctx.Err()
		}).Name("slow")

		go func() {
			<-started // Ensure target started
		}()

		start := time.Now()
		_, err := targ.Execute([]string{"app", "--timeout", "50ms", "slow"}, target, dummy())
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(elapsed).To(BeNumerically("<", 200*time.Millisecond))
	})

	t.Run("TimeoutEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		}).Name("slow")

		start := time.Now()
		_, err := targ.Execute([]string{"app", "--timeout=50ms", "slow"}, target, dummy())
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(elapsed).To(BeNumerically("<", 200*time.Millisecond))
	})

	t.Run("TimesFlagControlsRepetition", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			times := rapid.IntRange(1, 5).Draw(t, "times")

			count := 0
			target := targ.Targ(func() { count++ }).Name("counter")

			_, err := targ.Execute(
				[]string{"app", "--times", itoa(times), "counter"},
				target, dummy(),
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(count).To(Equal(times))
		})
	})

	t.Run("RetryFlagRerunsOnFailure", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() error {
			count++
			if count < 3 {
				return errors.New("fail")
			}

			return nil
		}).Name("flaky")

		_, err := targ.Execute(
			[]string{"app", "--times", "5", "--retry", "flaky"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(3))
	})

	t.Run("BackoffFlagControlsRetryDelay", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		start := time.Now()
		target := targ.Targ(func() error {
			count++

			return errors.New("fail")
		}).Name("failing")

		_, err := targ.Execute(
			[]string{"app", "--times", "3", "--retry", "--backoff", "20ms,2.0", "failing"},
			target, dummy(),
		)
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(count).To(Equal(3))
		// Should have waited ~20ms + ~40ms = ~60ms between retries
		g.Expect(elapsed).To(BeNumerically(">=", 50*time.Millisecond))
	})

	t.Run("WhileFlagStopsOnFalse", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() { count++ }).Name("loop")

		// "false" command always returns non-zero, stopping loop immediately
		_, err := targ.Execute(
			[]string{"app", "--times", "10", "--while", "false", "loop"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(0))
	})

	t.Run("CacheFlagConflictsWithTargetCache", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cached").Cache("**/*.go")

		result, err := targ.Execute(
			[]string{"app", "--cache", "**/*.ts", "cached"},
			target, dummy(),
		)
		// Error message is printed to output, err is just ExitError{Code: 1}
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--cache conflicts"))
	})

	t.Run("CacheAllowedWhenDisabled", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("flexible").Cache(targ.Disabled)

		// With Disabled, --cache flag is allowed (patterns won't match but no conflict)
		_, err := targ.Execute(
			[]string{"app", "--cache", "nonexistent/**", "flexible"},
			target, dummy(),
		)
		// May error due to cache check, but not conflict error
		if err != nil {
			g.Expect(err.Error()).NotTo(ContainSubstring("conflicts"))
		}
	})

	// NOTE: DepsFlagConflictsWithTargetDeps is NOT tested here because
	// the public API doesn't currently populate HasDeps in TargetConfig.
	// The internal ExecuteWithOverrides tests verify this conflict detection,
	// but it requires fixing the command.go implementation to work via public API.

	t.Run("OverridesWorkWhenTargetHasNoConfig", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() { count++ }).Name("plain")

		_, err := targ.Execute(
			[]string{"app", "--times", "2", "plain"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(2))
	})

	t.Run("DisabledTimeoutRejectsFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("notimeout")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--timeout", "5m", "notimeout"},
			targ.RunOptions{DisableTimeout: true},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
	})
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}
