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

// Property: Dependencies run exactly once per execution context
func TestProperty_Dependencies_RunExactlyOncePerExecutionContext(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		var depCount atomic.Int32

		dep := targ.Targ(func() { depCount.Add(1) })

		// Two targets sharing same dependency
		a := targ.Targ(func() {}).Deps(dep)
		b := targ.Targ(func() {}).Deps(dep)

		// Run both in sequence
		err := a.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depCount.Load()).To(Equal(int32(1)))

		// Running b should run dep again (new execution context)
		err = b.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depCount.Load()).To(Equal(int32(2)))
	})
}

// Property: Serial mode runs dependencies sequentially
func TestProperty_Dependencies_SerialModeRunsSequentially(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	order := make([]string, 0)

	var mu sync.Mutex

	a := targ.Targ(func() {
		mu.Lock()

		order = append(order, "a")

		mu.Unlock()
	})
	b := targ.Targ(func() {
		mu.Lock()

		order = append(order, "b")

		mu.Unlock()
	})
	c := targ.Targ(func() {
		mu.Lock()

		order = append(order, "c")

		mu.Unlock()
	}).Deps(a, b) // Default is serial

	err := c.Run(context.Background())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(order).To(Equal([]string{"a", "b", "c"}))
}

// Property: Parallel mode runs dependencies concurrently
func TestProperty_Dependencies_ParallelModeRunsConcurrently(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Use channels to verify concurrent execution
	aStarted := make(chan struct{})
	bStarted := make(chan struct{})
	done := make(chan struct{})

	a := targ.Targ(func() {
		close(aStarted)
		<-done
	})
	b := targ.Targ(func() {
		close(bStarted)
		<-done
	})
	c := targ.Targ(func() {}).Deps(a, b, targ.DepModeParallel)

	go func() {
		_ = c.Run(context.Background())
	}()

	// Both should start before either completes
	select {
	case <-aStarted:
	case <-time.After(time.Second):
		t.Fatal("a did not start in time")
	}

	select {
	case <-bStarted:
	case <-time.After(time.Second):
		t.Fatal("b did not start in time")
	}

	close(done)

	g.Eventually(func() bool { return true }).Should(BeTrue())
}

// Property: Timeout enforces time bound
func TestProperty_Timeout_CancelsExecutionAfterDuration(t *testing.T) {
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
	g.Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
	g.Expect(elapsed).To(BeNumerically("<", 500*time.Millisecond))
}

// Property: Retry continues on failure until success
func TestProperty_Retry_ContinuesOnFailure(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	execCount := 0
	target := targ.Targ(func() error {
		execCount++
		if execCount < 3 {
			return errors.New("fail")
		}

		return nil
	}).Times(5).Retry()

	err := target.Run(context.Background())
	// Retry should stop on first success and return no error
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(execCount).To(Equal(3)) // Stopped at first success
}

// Property: Retry stops after max attempts
func TestProperty_Retry_StopsAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		maxTimes := rapid.IntRange(2, 5).Draw(rt, "maxTimes")

		execCount := 0
		target := targ.Targ(func() error {
			execCount++
			return errors.New("always fail")
		}).Times(maxTimes).Retry()

		err := target.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(execCount).To(Equal(maxTimes))
	})
}

// Property: Times runs exactly N times when successful
func TestProperty_Times_RunsUpToNTimes(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		times := rapid.IntRange(1, 10).Draw(rt, "times")

		execCount := 0
		target := targ.Targ(func() { execCount++ }).Times(times)

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(execCount).To(Equal(times))
	})
}

// Property: Times stops on failure without retry
func TestProperty_Times_StopsOnFailureWithoutRetry(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		failAt := rapid.IntRange(1, 4).Draw(rt, "failAt")

		execCount := 0
		target := targ.Targ(func() error {
			execCount++
			if execCount == failAt {
				return errors.New("fail")
			}

			return nil
		}).Times(10) // No retry

		err := target.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(execCount).To(Equal(failAt))
	})
}

// Property: Context cancellation stops execution
func TestProperty_Context_RespectsCancellation(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var wasContextDone bool

	target := targ.Targ(func(ctx context.Context) error {
		<-ctx.Done()

		wasContextDone = true

		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- target.Run(ctx)
	}()

	// Give the target time to start
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	g.Expect(err).To(HaveOccurred())
	g.Expect(wasContextDone).To(BeTrue())
}

// Property: Error return indicates failure
func TestProperty_Execution_ErrorReturnIndicatesFailure(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		errMsg := rapid.StringMatching(`[a-z]{5,20}`).Draw(rt, "errMsg")
		expectedErr := errors.New(errMsg)

		target := targ.Targ(func() error {
			return expectedErr
		})

		err := target.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(errMsg))
	})
}

// Property: Basic execution completes successfully
func TestProperty_Execution_BasicExecutionCompletesSuccessfully(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		called := false
		target := targ.Targ(func() { called = true })

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(called).To(BeTrue())
	})
}

// Property: Backoff increases delay between retries
func TestProperty_Backoff_IncreasesDelayBetweenRetries(t *testing.T) {
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

	// First run has no delay, second ~20ms, third ~40ms
	// Just verify delays exist and are increasing
	g.Expect(delays).To(HaveLen(3))
	g.Expect(delays[2]).To(BeNumerically(">", delays[1]))
}

// Property: While condition stops execution when false
func TestProperty_While_StopsWhenConditionFalse(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		stopAt := rapid.IntRange(1, 5).Draw(rt, "stopAt")

		execCount := 0
		target := targ.Targ(func() { execCount++ }).
			Times(10).
			While(func() bool { return execCount < stopAt })

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(execCount).To(Equal(stopAt))
	})
}
