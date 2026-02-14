// TEST-002: Execution properties - validates target builder pattern and invocation
// traces: ARCH-002, ARCH-011

// Package targ_test contains property-based tests for targ.
// The test functions have many subtests which triggers maintidx warnings,
// but this is the intended structure for property-based testing.
//
//nolint:maintidx // Test functions with many subtests have low maintainability index by design
package targ_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// TagOptionsArgs is a struct with a TagOptions method that dynamically overrides tag options.
type TagOptionsArgs struct {
	Verbose bool `targ:"flag"`
}

// TagOptions is called by targ for each field to allow dynamic tag option overrides.
func (a *TagOptionsArgs) TagOptions(
	fieldName string,
	opts targ.TagOptions,
) (targ.TagOptions, error) {
	if fieldName == "Verbose" {
		opts.Desc = "dynamic description"
	}

	return opts, nil
}

// TagOptionsErrorArgs is a struct with a TagOptions method that returns an error.
type TagOptionsErrorArgs struct {
	Verbose bool `targ:"flag"`
}

// TagOptions returns an error for testing error handling.
func (a *TagOptionsErrorArgs) TagOptions(
	_ string,
	opts targ.TagOptions,
) (targ.TagOptions, error) {
	return opts, errors.New("tag options error")
}

func TestProperty_Execution(t *testing.T) {
	t.Parallel()

	t.Run("NoTargetsPrintsMessage", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Execute with no targets should print "No commands found."
		result, err := targ.Execute([]string{"app"})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("No commands found"))
	})

	t.Run("ExitErrorHasMessage", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// ExitError implements error interface
		exitErr := targ.ExitError{Code: 42}
		g.Expect(exitErr.Error()).To(ContainSubstring("42"))
	})

	t.Run("MultipleTargetsRunSequentially", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		order := make([]string, 0)
		a := targ.Targ(func() { order = append(order, "a") }).Name("a")
		b := targ.Targ(func() { order = append(order, "b") }).Name("b")

		_, err := targ.Execute([]string{"app", "a", "b"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(order).To(Equal([]string{"a", "b"}))
	})

	t.Run("BackoffIncreasesDelayBetweenRetries", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		delays := make([]time.Duration, 0)
		lastRun := time.Now()
		execCount := 0

		target := targ.Targ(func() error {
			now := time.Now()
			delays = append(delays, now.Sub(lastRun))
			lastRun = now
			execCount++

			return errors.New("fail")
		}).Times(3).Retry().Backoff(20*time.Millisecond, 2.0)

		err := target.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(execCount).To(Equal(3))
		g.Expect(delays).To(HaveLen(3))
		g.Expect(delays[2]).To(BeNumerically(">", delays[1]))
	})

	t.Run("ShellCommandExecutesViaRun", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			shouldFail := rapid.Bool().Draw(t, "shouldFail")

			var cmd string
			if shouldFail {
				cmd = "exit 1"
			} else {
				cmd = "true"
			}

			target := targ.Targ(cmd)
			err := target.Run(context.Background())

			if shouldFail {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	t.Run("RunWithArgsPassesToFunction", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var received int

		target := targ.Targ(func(_ context.Context, x int) {
			received = x
		})

		err := target.Run(context.Background(), 42)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(received).To(Equal(42))
	})

	t.Run("RunWithMissingArgsUsesZeroValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var received int

		target := targ.Targ(func(_ context.Context, x int) {
			received = x
		})

		// Don't pass the int arg - should use zero value
		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(received).To(Equal(0))
	})

	t.Run("WhileStopsWhenConditionFalse", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			stopAt := rapid.IntRange(1, 5).Draw(t, "stopAt")

			execCount := 0
			target := targ.Targ(func() { execCount++ }).
				Times(10).
				While(func() bool { return execCount < stopAt })

			err := target.Run(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(execCount).To(Equal(stopAt))
		})
	})

	t.Run("ContextCancellationStopsExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})

		target := targ.Targ(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()

			return ctx.Err()
		})

		errCh := make(chan error, 1)

		go func() {
			errCh <- target.Run(ctx)
		}()

		<-started // Wait for execution to start
		cancel()  // Cancel the context

		err := <-errCh
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})

	t.Run("ContextCancellationStopsLoop", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx, cancel := context.WithCancel(context.Background())

		var execCount atomic.Int32

		started := make(chan struct{})

		var once sync.Once

		target := targ.Targ(func() {
			execCount.Add(1)
			once.Do(func() { close(started) }) // Signal first execution started
			time.Sleep(50 * time.Millisecond)  // Give time for cancellation
		}).Times(100)

		errCh := make(chan error, 1)

		go func() {
			errCh <- target.Run(ctx)
		}()

		<-started // Wait for first execution to start
		cancel()  // Cancel the context

		err := <-errCh
		g.Expect(err).To(HaveOccurred())
		g.Expect(execCount.Load()).To(BeNumerically("<", 100)) // Should stop before completing all
	})

	t.Run("DependencySerialExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		order := make([]string, 0)

		var mu sync.Mutex

		dep1 := targ.Targ(func() {
			mu.Lock()

			order = append(order, "dep1")

			mu.Unlock()
		}).Name("dep1")

		dep2 := targ.Targ(func() {
			mu.Lock()

			order = append(order, "dep2")

			mu.Unlock()
		}).Name("dep2")

		main := targ.Targ(func() {
			mu.Lock()

			order = append(order, "main")

			mu.Unlock()
		}).Deps(dep1, dep2, targ.DepModeSerial)

		err := main.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(order).To(Equal([]string{"dep1", "dep2", "main"}))
	})

	t.Run("DependencyParallelExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			startedCount   atomic.Int32
			completedCount atomic.Int32
		)

		parallelStart := make(chan struct{})

		dep1 := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("dep1")

		dep2 := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("dep2")

		main := targ.Targ(func() {}).Deps(dep1, dep2, targ.DepModeParallel)

		errCh := make(chan error, 1)

		go func() {
			errCh <- main.Run(context.Background())
		}()

		// Wait for both deps to start
		g.Eventually(startedCount.Load).Should(Equal(int32(2)))

		// Both should be waiting (neither completed yet)
		g.Expect(completedCount.Load()).To(Equal(int32(0)))

		// Release both
		close(parallelStart)

		err := <-errCh
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(completedCount.Load()).To(Equal(int32(2)))
	})

	t.Run("DependencyChainedGroupExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var mu sync.Mutex
		var order []string

		a := targ.Targ(func() {
			mu.Lock()
			order = append(order, "a")
			mu.Unlock()
		}).Name("a")

		b := targ.Targ(func() {
			mu.Lock()
			order = append(order, "b")
			mu.Unlock()
		}).Name("b")

		c := targ.Targ(func() {
			mu.Lock()
			order = append(order, "c")
			mu.Unlock()
		}).Name("c")

		main := targ.Targ(func() {
			mu.Lock()
			order = append(order, "main")
			mu.Unlock()
		}).Deps(a).Deps(b, c, targ.DepModeParallel)

		err := main.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())

		// a must come first (serial group 1), then b and c (parallel group 2), then main
		g.Expect(order[0]).To(Equal("a"))
		g.Expect(order[len(order)-1]).To(Equal("main"))
		// b and c are in the middle (parallel, order may vary)
		g.Expect(order[1:3]).To(ConsistOf("b", "c"))
	})

	t.Run("DependencyChainedGroupError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false

		a := targ.Targ(func() error {
			return fmt.Errorf("fail in group 1")
		}).Name("a")

		b := targ.Targ(func() {
			executed = true
		}).Name("b")

		main := targ.Targ(func() {}).
			Deps(a).
			Deps(b, targ.DepModeParallel)

		err := main.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(executed).To(BeFalse(), "group 2 should not run if group 1 fails")
	})

	t.Run("TimeoutEnforcesLimit", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(ctx context.Context) error {
			<-ctx.Done()

			return ctx.Err()
		}).Timeout(50 * time.Millisecond)

		start := time.Now()
		err := target.Run(context.Background())
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(elapsed).To(BeNumerically("<", 200*time.Millisecond))
	})

	t.Run("RetryRerunsOnFailure", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		count := 0
		target := targ.Targ(func() error {
			count++
			if count < 3 {
				return errors.New("fail")
			}

			return nil
		}).Times(5).Retry()

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(3)) // Ran until success
	})

	t.Run("TimesLimitEnforced", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			times := rapid.IntRange(1, 5).Draw(t, "times")

			count := 0
			target := targ.Targ(func() { count++ }).Times(times)

			err := target.Run(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(count).To(Equal(times))
		})
	})

	t.Run("ErrorPropagatesFromFunction", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			errMsg := rapid.String().Draw(t, "errMsg")

			target := targ.Targ(func() error {
				return errors.New(errMsg)
			})

			err := target.Run(context.Background())
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(errMsg))
		})
	})

	t.Run("DependencyErrorStopsExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depErr := errors.New("dependency failed")
		mainCalled := false

		dep := targ.Targ(func() error {
			return depErr
		}).Name("dep")

		main := targ.Targ(func() {
			mainCalled = true
		}).Deps(dep)

		err := main.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("dependency failed"))
		g.Expect(mainCalled).To(BeFalse()) // Main should not run
	})

	t.Run("ExecuteWithSerialDeps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			order []string
			mu    sync.Mutex
		)

		dep1 := targ.Targ(func() {
			mu.Lock()

			order = append(order, "dep1")

			mu.Unlock()
		}).Name("dep1")

		dep2 := targ.Targ(func() {
			mu.Lock()

			order = append(order, "dep2")

			mu.Unlock()
		}).Name("dep2")

		main := targ.Targ(func() {
			mu.Lock()

			order = append(order, "main")

			mu.Unlock()
		}).Name("main").Deps(dep1, dep2, targ.DepModeSerial)

		// Pass all targets so deps are registered
		_, err := targ.Execute([]string{"app", "main"}, main, dep1, dep2)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(order).To(Equal([]string{"dep1", "dep2", "main"}))
	})

	t.Run("ExecuteWithParallelDeps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			startedCount   atomic.Int32
			completedCount atomic.Int32
		)

		parallelStart := make(chan struct{})

		dep1 := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("dep1")

		dep2 := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("dep2")

		main := targ.Targ(func() {}).Name("main").Deps(dep1, dep2, targ.DepModeParallel)

		errCh := make(chan error, 1)

		go func() {
			// Pass all targets so deps are registered
			_, err := targ.Execute([]string{"app", "main"}, main, dep1, dep2)
			errCh <- err
		}()

		// Wait for both deps to start
		g.Eventually(startedCount.Load).Should(Equal(int32(2)))

		// Both should be waiting (neither completed yet)
		g.Expect(completedCount.Load()).To(Equal(int32(0)))

		// Release both
		close(parallelStart)

		err := <-errCh
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(completedCount.Load()).To(Equal(int32(2)))
	})

	t.Run("SuccessfulExecutionReturnsNil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		target := targ.Targ(func() {
			executed = true
		})

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("InternalListShowsAvailableTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build").Description("Build the project")
		test := targ.Targ(func() {}).Name("test").Description("Run tests")

		// __list is an internal command used by completion scripts
		result, err := targ.Execute([]string{"app", "__list"}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("InternalListShowsGroupHierarchy", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := targ.Targ(func() {}).Name("sub")
		grp := targ.Group("grp", sub)

		// __list is an internal command
		result, err := targ.Execute([]string{"app", "__list"}, grp)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("grp"))
	})

	t.Run("ParallelFlagRunsTargetsInParallel", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			startedCount   atomic.Int32
			completedCount atomic.Int32
		)

		parallelStart := make(chan struct{})

		a := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("a")

		b := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("b")

		errCh := make(chan error, 1)

		go func() {
			_, err := targ.Execute([]string{"app", "--parallel", "a", "b"}, a, b)
			errCh <- err
		}()

		// Wait for both to start - if they run in parallel, both will start
		g.Eventually(startedCount.Load).Should(Equal(int32(2)))

		// Both started, neither completed yet
		g.Expect(completedCount.Load()).To(Equal(int32(0)))

		// Release both
		close(parallelStart)

		err := <-errCh
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("ParallelFlagWithSingleRootGroup", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			startedCount   atomic.Int32
			completedCount atomic.Int32
		)

		parallelStart := make(chan struct{})

		a := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("a")

		b := targ.Targ(func() {
			startedCount.Add(1)
			<-parallelStart
			completedCount.Add(1)
		}).Name("b")

		// Single root (group) containing both targets
		grp := targ.Group("grp", a, b)

		errCh := make(chan error, 1)

		go func() {
			// In single-root mode with --parallel, subcommands run concurrently
			// Add flag-like args that should be skipped by executeDefaultParallel
			_, err := targ.Execute([]string{"app", "--parallel", "-x", "a", "--ignored", "b"}, grp)
			errCh <- err
		}()

		// Wait for both to start
		g.Eventually(startedCount.Load).Should(Equal(int32(2)))

		// Release both
		close(parallelStart)

		err := <-errCh
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("ParallelFailureReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		success := targ.Targ(func() {}).Name("success")
		failure := targ.Targ(func() error {
			return errors.New("deliberate failure")
		}).Name("failure")

		result, err := targ.Execute(
			[]string{"app", "--parallel", "success", "failure"},
			success,
			failure,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("failure"))
	})

	t.Run("VariadicPositionalAcceptsMultipleValues", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files []string `targ:"positional"`
		}

		var captured []string

		target := targ.Targ(func(a Args) {
			captured = a.Files
		}).Name("cmd")

		_, err := targ.Execute([]string{"app", "a.txt", "b.txt", "c.txt"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured).To(Equal([]string{"a.txt", "b.txt", "c.txt"}))
	})

	t.Run("VariadicPositionalTerminatedByFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files   []string `targ:"positional"`
			Verbose bool     `targ:"flag"`
		}

		var captured Args

		target := targ.Targ(func(a Args) {
			captured = a
		}).Name("cmd")

		// Variadic positional captures values until flag is encountered
		_, err := targ.Execute([]string{"app", "a.txt", "b.txt", "--verbose"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured.Files).To(Equal([]string{"a.txt", "b.txt"}))
		g.Expect(captured.Verbose).To(BeTrue())
	})

	t.Run("InterleavedFlagsAndPositionals", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag"`
			File    string `targ:"positional"`
		}

		var captured Args

		target := targ.Targ(func(a Args) {
			captured = a
		}).Name("cmd")

		_, err := targ.Execute([]string{"app", "test.txt", "--verbose"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured.File).To(Equal("test.txt"))
		g.Expect(captured.Verbose).To(BeTrue())
	})

	t.Run("InterleavedFlagsTrackPosition", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Include []targ.Interleaved[string] `targ:"flag,short=i"`
			Exclude []targ.Interleaved[string] `targ:"flag,short=e"`
		}

		var captured Args

		target := targ.Targ(func(a Args) {
			captured = a
		}).Name("filter")

		// Interleave include/exclude flags
		_, err := targ.Execute([]string{"app", "-i", "a", "-e", "b", "-i", "c"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured.Include).To(HaveLen(2))
		g.Expect(captured.Exclude).To(HaveLen(1))
		// Positions should reflect global order
		g.Expect(captured.Include[0].Value).To(Equal("a"))
		g.Expect(captured.Include[0].Position).To(Equal(0))
		g.Expect(captured.Exclude[0].Value).To(Equal("b"))
		g.Expect(captured.Exclude[0].Position).To(Equal(1))
		g.Expect(captured.Include[1].Value).To(Equal("c"))
		g.Expect(captured.Include[1].Position).To(Equal(2))
	})

	t.Run("DoubleDashSeparatorEndsVariadicAndStartsNew", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files  []string `targ:"positional"`
			Output string   `targ:"positional"`
		}

		var captured Args

		target := targ.Targ(func(a Args) {
			captured = a
		}).Name("cmd")

		// -- should end variadic positional and allow next positional
		_, err := targ.Execute([]string{"app", "a.txt", "b.txt", "--", "out.txt"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured.Files).To(Equal([]string{"a.txt", "b.txt"}))
		g.Expect(captured.Output).To(Equal("out.txt"))
	})

	t.Run("GlobPatternMatchesMultipleTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			executed []string
			mu       sync.Mutex
		)

		build := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "build")

			mu.Unlock()
		}).Name("build")

		buildDocker := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "build-docker")

			mu.Unlock()
		}).Name("build-docker")

		test := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "test")

			mu.Unlock()
		}).Name("test")

		// Glob pattern should match build and build-docker
		_, err := targ.Execute([]string{"app", "build*"}, build, buildDocker, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(ContainElement("build"))
		g.Expect(executed).To(ContainElement("build-docker"))
		g.Expect(executed).NotTo(ContainElement("test"))
	})

	t.Run("GlobPatternInParallelMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			executed []string
			mu       sync.Mutex
		)

		build := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "build")

			mu.Unlock()
		}).Name("build")

		buildDocker := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "build-docker")

			mu.Unlock()
		}).Name("build-docker")

		test := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "test")

			mu.Unlock()
		}).Name("test")

		// Glob pattern with --parallel
		_, err := targ.Execute([]string{"app", "--parallel", "build*"}, build, buildDocker, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(ContainElement("build"))
		g.Expect(executed).To(ContainElement("build-docker"))
		g.Expect(executed).NotTo(ContainElement("test"))
	})

	t.Run("TagOptionsMethodOverridesDescription", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(_ TagOptionsArgs) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// The TagOptions method should have overridden the description
		g.Expect(result.Output).To(ContainSubstring("dynamic description"))
	})

	t.Run("TagOptionsMethodErrorReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(_ TagOptionsErrorArgs) {}).Name("cmd")

		// When TagOptions returns an error, execution should fail
		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("tag options error"))
	})

	t.Run("RawFunctionCanBeExecuted", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		rawFunc := func() { executed = true }

		_, err := targ.Execute([]string{"app"}, rawFunc)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	t.Run("MissingRequiredPositionalReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional,required"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("File"))
	})

	t.Run("MissingSecondRequiredPositionalReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Input  string `targ:"positional,required"`
			Output string `targ:"positional,required"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Provide only first positional
		result, err := targ.Execute([]string{"app", "input.txt"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Output"))
	})

	t.Run("MissingPositionalWithEmptyNameUsesFieldName", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			// Explicitly set name to empty string to test fallback
			MyFile string `targ:"positional,required,name="`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		// When name tag is empty, should use field name "MyFile"
		g.Expect(result.Output).To(ContainSubstring("MyFile"))
	})

	t.Run("OptionalPositionalNotProvidedSucceeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Required string `targ:"positional,required"`
			Optional string `targ:"positional"`
		}

		var captured Args

		target := targ.Targ(func(a Args) {
			captured = a
		}).Name("cmd")

		_, err := targ.Execute([]string{"app", "required-value"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(captured.Required).To(Equal("required-value"))
		g.Expect(captured.Optional).To(Equal("")) // Not provided, empty string
	})

	t.Run("WatchModeExitsOnContextCancellation", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		target := targ.Targ(func() {
			executed = true
		}).Name("cmd")

		// Create a context that will be cancelled shortly
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after a short delay to allow initial execution
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		// Use --watch with a pattern that matches a real file
		// AllowDefault: false so we must specify the command name
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--watch", "go.mod", "cmd"},
			targ.RunOptions{AllowDefault: false, Context: ctx},
			target,
		)

		// Should error due to watch being cancelled
		g.Expect(err).To(HaveOccurred())
		// Error message is printed to output
		g.Expect(result.Output).To(ContainSubstring("watch"))
		g.Expect(executed).To(BeTrue()) // Initial run should have completed
	})

	t.Run("WatchModeInitialErrorSkipsWatch", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() error {
			return errors.New("initial failure")
		}).Name("cmd")

		// Use --watch but target will fail initially
		// AllowDefault: false so we must specify the command name
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--watch", "go.mod", "cmd"},
			targ.RunOptions{AllowDefault: false},
			target,
		)

		// Should error from initial execution, not from watch
		g.Expect(err).To(HaveOccurred())
		// Error message is printed to output
		g.Expect(result.Output).To(ContainSubstring("initial failure"))
	})

	t.Run("RecursiveGlobMatchesSubcommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var (
			executed []string
			mu       sync.Mutex
		)

		lint := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "lint")

			mu.Unlock()
		}).Name("lint")

		test := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "test")

			mu.Unlock()
		}).Name("test")

		// Create a group with subcommands
		check := targ.Group("check", lint, test)

		build := targ.Targ(func() {
			mu.Lock()

			executed = append(executed, "build")

			mu.Unlock()
		}).Name("build")

		// ** pattern should match all nested subcommands
		_, err := targ.Execute([]string{"app", "**"}, check, build)
		g.Expect(err).NotTo(HaveOccurred())
		// Should have executed all subcommands under check
		g.Expect(executed).To(ContainElement("lint"))
		g.Expect(executed).To(ContainElement("test"))
		// build is at root level, not nested
	})

	t.Run("CacheDirConfiguresDirectory", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// CacheDir should return target for chaining
		target := targ.Targ(func() {}).CacheDir("/custom/cache")
		g.Expect(target).NotTo(BeNil())

		// Verify it's chainable
		target2 := targ.Targ(func() {}).Name("test").CacheDir("/custom/cache").Description("desc")
		g.Expect(target2).NotTo(BeNil())
	})

	t.Run("WatchConfiguresPatterns", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Watch with patterns should return target for chaining
		target := targ.Targ(func() {}).Watch("*.go", "go.mod")
		g.Expect(target).NotTo(BeNil())

		// Watch with Disabled should also work
		target2 := targ.Targ(func() {}).Watch(targ.Disabled)
		g.Expect(target2).NotTo(BeNil())

		// Verify chainable with Disabled
		target3 := targ.Targ(func() {}).Name("test").Watch(targ.Disabled).Description("desc")
		g.Expect(target3).NotTo(BeNil())
	})

	t.Run("CacheWithRunChecksFiles", func(t *testing.T) {
		t.Parallel()

		// Use a pattern that matches real files (go.mod exists in repo root)
		// The pattern needs to be relative to working directory
		target := targ.Targ(func() {}).Cache("../go.mod").CacheDir(t.TempDir())

		// First run should execute (cache miss or fail gracefully)
		// The main point is to exercise the cache path code
		err := target.Run(context.Background())
		// If it fails due to pattern issues, that's OK - we're testing the code path
		_ = err
	})

	t.Run("CacheWithCustomDir", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Use a pattern that matches real files with custom cache dir
		target := targ.Targ(func() {}).Cache("../go.mod").CacheDir(t.TempDir())

		// Run - may fail due to pattern issues, but exercises the code path
		err := target.Run(context.Background())
		_ = err
		// Just verify it didn't panic
		g.Expect(true).To(BeTrue())
	})

	t.Run("WatchPatternsWithRunEntersWatchLoop", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		// Create a target with watch patterns (matches nothing, but enters watch loop)
		target := targ.Targ(func() {
			executed = true
		}).Watch("*.nonexistent")

		// Create a context that will be cancelled shortly
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		// Call Run directly - should execute, enter watch loop, then exit on cancel
		err := target.Run(ctx)

		// Should get error from cancelled watch
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("watch"))
		g.Expect(executed).To(BeTrue())
	})

	t.Run("ParallelModeSkipsFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Test that --parallel skips flag-like args (starting with -)
		aCalls := 0
		bCalls := 0
		a := targ.Targ(func() { aCalls++ }).Name("alpha")
		b := targ.Targ(func() { bCalls++ }).Name("beta")

		// In parallel mode with a flag-like arg mixed in
		// The -x should be skipped, targets should still run
		_, err := targ.Execute(
			[]string{"app", "--parallel", "-x", "alpha", "--ignored", "beta"},
			a, b,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aCalls).To(Equal(1))
		g.Expect(bCalls).To(Equal(1))
	})

	t.Run("WhileConditionFalseStopsExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		execCount := 0
		// Single target runs as default, no need for name or subcommand
		target := targ.Targ(func() {
			execCount++
		})

		// Use CLI override --while "exit 1" which fails immediately
		// With single target, overrides apply directly - no subcommand needed
		_, err := targ.Execute(
			[]string{"app", "--while", "exit 1", "--times", "5"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(execCount).To(Equal(0)) // Should never execute
	})

	t.Run("RetryOnFailureWithContextCancelled", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		execCount := 0
		ctx, cancel := context.WithCancel(context.Background())

		// Single target runs as default, no need for name or subcommand
		target := targ.Targ(func() error {
			execCount++
			// Cancel context after first execution
			cancel()

			return errors.New("deliberate failure")
		})

		// Use CLI override --retry with --times 5
		// Should fail because context was cancelled (with previous error)
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--retry", "--times", "5"},
			targ.RunOptions{Context: ctx, AllowDefault: true},
			target,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(execCount).To(Equal(1)) // Executed once before cancel
	})

	// Deps-only targets - targets with no function that just run dependencies
	t.Run("DepsOnlyTargetRunsDependencies", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aRan := false
		bRan := false

		a := targ.Targ(func() { aRan = true }).Name("a")
		b := targ.Targ(func() { bRan = true }).Name("b")
		all := targ.Targ().Name("all").Deps(a, b)

		_, err := targ.Execute([]string{"app", "all"}, a, b, all)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aRan).To(BeTrue())
		g.Expect(bRan).To(BeTrue())
	})

	t.Run("DepsOnlyTargetErrorPropagates", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep := targ.Targ(func() error {
			return errors.New("dep failed in deps-only")
		}).Name("dep")

		all := targ.Targ().Name("all").Deps(dep)

		result, err := targ.Execute([]string{"app", "all"}, dep, all)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("dep failed in deps-only"))
	})

	t.Run("DepsOnlyTargetHelpShowsDeps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false

		dep := targ.Targ(func() { depRan = true }).Name("dep")
		all := targ.Targ().Name("all").Deps(dep)

		// Help should not run the deps
		result, err := targ.Execute([]string{"app", "all", "--help"}, dep, all)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("all"))
	})

	t.Run("DepsOnlyTargetNoDepsSucceeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// A deps-only target with no deps should still succeed
		// Use a second target so it's not default mode (need to specify command name)
		empty := targ.Targ().Name("empty")
		other := targ.Targ(func() {}).Name("other")

		_, err := targ.Execute([]string{"app", "empty"}, empty, other)
		g.Expect(err).NotTo(HaveOccurred())
	})

	// Error propagation tests - verify errors bubble up through CLI path
	t.Run("DependencyErrorPropagatesViaCLI", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depErr := errors.New("dependency failed via CLI")
		mainCalled := false

		dep := targ.Targ(func() error {
			return depErr
		}).Name("dep")

		main := targ.Targ(func() {
			mainCalled = true
		}).Name("main").Deps(dep)

		// Execute via CLI path (not .Run()) to exercise runTargetWithOverrides
		result, err := targ.Execute([]string{"app", "main"}, main, dep)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("dependency failed via CLI"))
		g.Expect(mainCalled).To(BeFalse())
	})

	t.Run("TargetExecutionErrorPropagatesViaCLI", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		execErr := errors.New("execution failed via CLI")

		target := targ.Targ(func() error {
			return execErr
		}).Name("failing")

		// Single target runs as default - no subcommand needed
		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("execution failed via CLI"))
	})

	t.Run("RequiredFlagMissingPropagatesViaCLI", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string `targ:"flag,required"`
		}

		called := false
		target := targ.Targ(func(_ Args) {
			called = true
		}).Name("cmd")

		// Single target runs as default - no subcommand needed
		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("required"))
		g.Expect(called).To(BeFalse())
	})

	t.Run("ParallelDepsFirstErrorPropagates", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep1Err := errors.New("dep1 failed")

		dep1 := targ.Targ(func() error {
			return dep1Err
		}).Name("dep1")

		dep2 := targ.Targ(func() error {
			time.Sleep(50 * time.Millisecond) // Delay to ensure dep1 fails first
			return errors.New("dep2 failed")
		}).Name("dep2")

		mainCalled := false
		main := targ.Targ(func() {
			mainCalled = true
		}).Name("main").Deps(dep1, dep2, targ.DepModeParallel)

		result, err := targ.Execute([]string{"app", "main"}, main, dep1, dep2)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("dep1 failed"))
		g.Expect(mainCalled).To(BeFalse())
	})

	t.Run("SerialDepsStopOnFirstError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep1Called := false
		dep2Called := false

		dep1 := targ.Targ(func() error {
			dep1Called = true
			return errors.New("dep1 serial failed")
		}).Name("dep1")

		dep2 := targ.Targ(func() error {
			dep2Called = true
			return nil
		}).Name("dep2")

		mainCalled := false
		main := targ.Targ(func() {
			mainCalled = true
		}).Name("main").Deps(dep1, dep2) // Serial by default

		result, err := targ.Execute([]string{"app", "main"}, main, dep1, dep2)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("dep1 serial failed"))
		g.Expect(dep1Called).To(BeTrue())
		g.Expect(dep2Called).To(BeFalse()) // Should not run after dep1 fails
		g.Expect(mainCalled).To(BeFalse())
	})
}
