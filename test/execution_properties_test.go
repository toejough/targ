package targ_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestProperty_Execution(t *testing.T) {
	t.Parallel()

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

		var startedCount atomic.Int32
		var completedCount atomic.Int32
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
		g.Eventually(func() int32 { return startedCount.Load() }).Should(Equal(int32(2)))

		// Both should be waiting (neither completed yet)
		g.Expect(completedCount.Load()).To(Equal(int32(0)))

		// Release both
		close(parallelStart)

		err := <-errCh
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(completedCount.Load()).To(Equal(int32(2)))
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
}
