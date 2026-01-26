//nolint:maintidx // Test functions with many subtests have low maintainability index by design
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

	t.Run("TimeoutMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cmd")

		// --timeout without a value should error
		result, err := targ.Execute([]string{"app", "--timeout"}, target, dummy())
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("timeout"))
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

	t.Run("TimesFlagEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() { count++ }).Name("counter")

		_, err := targ.Execute(
			[]string{"app", "--times=3", "counter"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(3))
	})

	t.Run("TimesFlagMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--times"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--times requires"))
	})

	t.Run("TimesFlagInvalidValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		_, err := targ.Execute(
			[]string{"app", "--times", "notanumber", "target"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
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

	t.Run("BackoffFlagEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() error {
			count++

			return errors.New("fail")
		}).Name("failing")

		_, err := targ.Execute(
			[]string{"app", "--times", "2", "--retry", "--backoff=10ms,2.0", "failing"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(count).To(Equal(2))
	})

	t.Run("BackoffFlagMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--backoff"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--backoff requires"))
	})

	t.Run("BackoffFlagInvalidFormatReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		_, err := targ.Execute(
			[]string{"app", "--backoff", "invalid", "target"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
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

	t.Run("WhileFlagMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cmd")

		// --while without a command should error
		result, err := targ.Execute([]string{"app", "--while"}, target, dummy())
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--while requires"))
	})

	t.Run("WhileFlagEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() { count++ }).Name("loop")

		// --while=false with equals syntax
		_, err := targ.Execute(
			[]string{"app", "--times", "10", "--while=false", "loop"},
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

	t.Run("WatchFlagConflictsWithTargetWatch", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("watched").Watch("**/*.go")

		result, err := targ.Execute(
			[]string{"app", "--watch", "**/*.ts", "watched"},
			target, dummy(),
		)
		// Error message is printed to output, err is just ExitError{Code: 1}
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--watch conflicts"))
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

	t.Run("CacheFlagEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("flexible").Cache(targ.Disabled)

		_, err := targ.Execute(
			[]string{"app", "--cache=nonexistent/**", "flexible"},
			target, dummy(),
		)
		// May error due to cache check, but not conflict error
		if err != nil {
			g.Expect(err.Error()).NotTo(ContainSubstring("conflicts"))
		}
	})

	t.Run("CacheFlagMissingPatternReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--cache"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--cache requires"))
	})

	t.Run("WatchFlagEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		target := targ.Targ(func() {}).Name("cmd")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--watch=go.mod", "cmd"},
			targ.RunOptions{AllowDefault: false, Context: ctx},
			target,
		)
		// Will error due to watch being cancelled
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("WatchFlagMissingPatternReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--watch"},
			target, dummy(),
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--watch requires"))
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

	t.Run("DepModeSerialFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Track execution order
		var order []string

		dep1 := targ.Targ(func() { order = append(order, "dep1") }).Name("dep1")
		dep2 := targ.Targ(func() { order = append(order, "dep2") }).Name("dep2")
		main := targ.Targ(func() { order = append(order, "main") }).Name("main").Deps(dep1, dep2)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode", "serial", "main"},
			main, dep1, dep2, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Serial should maintain order
		g.Expect(order).To(HaveLen(3))
	})

	t.Run("DepModeSerialEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		dep := targ.Targ(func() {}).Name("dep")
		main := targ.Targ(func() { executed = true }).Name("main").Deps(dep)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode=serial", "main"},
			main, dep, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("DepModeParallelFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		dep := targ.Targ(func() {}).Name("dep")
		main := targ.Targ(func() { executed = true }).Name("main").Deps(dep)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode", "parallel", "main"},
			main, dep, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("DepModeParallelEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		dep := targ.Targ(func() {}).Name("dep")
		main := targ.Targ(func() { executed = true }).Name("main").Deps(dep)

		_, err := targ.Execute(
			[]string{"app", "--dep-mode=parallel", "main"},
			main, dep, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("DepModeInvalidValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--dep-mode", "invalid", "target"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("serial"))
		g.Expect(result.Output).To(ContainSubstring("parallel"))
	})

	t.Run("DepModeInvalidEqualsSyntaxReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		result, err := targ.Execute(
			[]string{"app", "--dep-mode=invalid", "target"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("serial"))
		g.Expect(result.Output).To(ContainSubstring("parallel"))
	})

	t.Run("DepModeMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		_, err := targ.Execute(
			[]string{"app", "--dep-mode"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("CacheDirFlagSetsDirectory", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		target := targ.Targ(func() { executed = true }).Name("build")

		// --cache-dir specifies where cache files are stored
		// Without --cache, the target just runs normally
		_, err := targ.Execute(
			[]string{"app", "--cache-dir", "/tmp/nonexistent", "build"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("CacheDirEqualsSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		target := targ.Targ(func() { executed = true }).Name("build")

		// --cache-dir= syntax, without --cache just runs target
		_, err := targ.Execute(
			[]string{"app", "--cache-dir=/tmp/nonexistent", "build"},
			target, dummy(),
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("CacheDirMissingValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		result, err := targ.Execute(
			[]string{"app", "--cache-dir"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--cache-dir"))
	})

	// Tests for --deps flag extraction
	// The --deps flag collects dependency names until a flag or --
	// Note: Runtime deps execution isn't implemented yet, tests verify parsing only

	t.Run("DepsFlagParsesSuccessfully", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")
		dep := targ.Targ(func() {}).Name("dep")

		// --deps followed by dependency names, then target name
		// The dep name is consumed by --deps, "target" becomes the command
		_, err := targ.Execute(
			[]string{"app", "--deps", "dep", "target"},
			target, dep,
		)
		// Should parse without error (deps aren't actually run, just parsed)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("DepsFlagWithMultipleValuesEndingAtAnotherFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")
		other := targ.Targ(func() {}).Name("other")

		// --deps with multiple values, ending when --times is hit
		// In multi-root mode, explicit target name is required
		result, err := targ.Execute(
			[]string{"app", "--deps", "dep1", "dep2", "--times", "1", "target"},
			target, other,
		)
		// Should parse without error
		g.Expect(err).NotTo(HaveOccurred(), "output: %s", result.Output)
	})

	t.Run("DepsFlagMissingTargetReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		// --deps at end with no value is an error
		result, err := targ.Execute(
			[]string{"app", "--deps"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--deps requires"))
	})

	t.Run("DepsFlagFollowedByPathResetWithoutValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("target")

		// --deps followed immediately by -- (path reset) with no deps is an error
		result, err := targ.Execute(
			[]string{"app", "--deps", "--", "target"},
			target, dummy(),
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--deps requires"))
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
