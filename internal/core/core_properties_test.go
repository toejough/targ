package core

// core_properties_test.go contains property-based tests for internal execution
// properties that require access to unexported symbols.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

// Property: Internal dependencies track execution state correctly
func TestProperty_Internal_DependencyExecutionTracking(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		var depCount atomic.Int32

		dep := Targ(func() { depCount.Add(1) })

		// Create target with dependency
		target := Targ(func() {}).Deps(dep)

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depCount.Load()).To(Equal(int32(1)))
	})
}

// Property: Execution context is properly propagated
func TestProperty_Internal_ContextPropagation(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type ctxKey string

	key := ctxKey("test-key")
	expectedValue := "test-value"

	var receivedValue any

	target := Targ(func(ctx context.Context) {
		receivedValue = ctx.Value(key)
	})

	ctx := context.WithValue(context.Background(), key, expectedValue)
	err := target.Run(ctx)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(receivedValue).To(Equal(expectedValue))
}

// Property: Parallel dependencies actually run in parallel
func TestProperty_Internal_ParallelDependencyExecution(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// Track concurrent execution
	var (
		running       atomic.Int32
		maxConcurrent atomic.Int32
	)

	updateMax := func() {
		for {
			current := running.Load()

			max := maxConcurrent.Load()
			if current <= max {
				break
			}

			if maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}
	}

	a := Targ(func() {
		running.Add(1)
		updateMax()
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
	})

	b := Targ(func() {
		running.Add(1)
		updateMax()
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
	})

	target := Targ(func() {}).Deps(a, b, DepModeParallel)

	err := target.Run(context.Background())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(maxConcurrent.Load()).To(Equal(int32(2))) // Both ran concurrently
}

// Property: Serial dependencies maintain order
func TestProperty_Internal_SerialDependencyOrder(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		numDeps := rapid.IntRange(2, 5).Draw(rt, "numDeps")

		order := make([]int, 0)

		var mu sync.Mutex

		deps := make([]any, 0, numDeps)
		for i := range numDeps {
			idx := i // Capture

			deps = append(deps, Targ(func() {
				mu.Lock()

				order = append(order, idx)

				mu.Unlock()
			}))
		}

		target := Targ(func() {}).Deps(deps...)

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())

		// Verify order is maintained
		expected := make([]int, numDeps)
		for i := range numDeps {
			expected[i] = i
		}

		g.Expect(order).To(Equal(expected))
	})
}

// Property: Error propagation from dependencies
func TestProperty_Internal_DependencyErrorPropagation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		errMsg := rapid.StringMatching(`[a-z]{5,15}`).Draw(rt, "errMsg")
		expectedErr := errors.New(errMsg)

		failingDep := Targ(func() error { return expectedErr })
		target := Targ(func() {}).Deps(failingDep)

		err := target.Run(context.Background())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring(errMsg))
	})
}

// Property: Target state is properly reset between runs
func TestProperty_Internal_StateResetBetweenRuns(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		runCount := 0
		target := Targ(func() { runCount++ })

		// Run multiple times
		for range 3 {
			err := target.Run(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
		}

		g.Expect(runCount).To(Equal(3))
	})
}

// Property: Builder methods are chainable and idempotent on target
func TestProperty_Internal_BuilderChainableAndIdempotent(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := Targ(func() {})
	original := target

	target = target.Name("test").
		Description("desc").
		Timeout(time.Second).
		Times(3).
		Retry().
		Cache("*.go").
		Watch("*.go")

	g.Expect(target).To(BeIdenticalTo(original))
}

// Property: Timeout enforcement is accurate
func TestProperty_Internal_TimeoutEnforcement(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		timeoutMs := rapid.IntRange(10, 50).Draw(rt, "timeoutMs")
		timeout := time.Duration(timeoutMs) * time.Millisecond

		var wasTimedOut bool

		target := Targ(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				wasTimedOut = true
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return nil
			}
		}).Timeout(timeout)

		start := time.Now()
		err := target.Run(context.Background())
		elapsed := time.Since(start)

		g.Expect(err).To(HaveOccurred())
		g.Expect(wasTimedOut).To(BeTrue())
		// Should complete close to the timeout
		g.Expect(elapsed).To(BeNumerically("<", timeout+50*time.Millisecond))
	})
}

// Property: Times counter respects limit
func TestProperty_Internal_TimesCounterRespectsLimit(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		times := rapid.IntRange(1, 10).Draw(rt, "times")

		count := 0
		target := Targ(func() { count++ }).Times(times)

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(times))
	})
}

// Property: Retry mechanism continues after failure
func TestProperty_Internal_RetryMechanism(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		successAt := rapid.IntRange(1, 4).Draw(rt, "successAt")

		count := 0
		target := Targ(func() error {
			count++
			if count < successAt {
				return errors.New("not yet")
			}

			return nil
		}).Times(5).Retry()

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(count).To(Equal(successAt))
	})
}

// Property: Args struct is properly populated
func TestProperty_Internal_ArgsStructPopulation(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.StringMatching(`[a-z]{5,10}`).Draw(rt, "value")

		type Args struct {
			Name string
		}

		var received Args

		target := Targ(func(args Args) {
			received = args
		})

		err := target.Run(context.Background(), Args{Name: value})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(received.Name).To(Equal(value))
	})
}

// Property: Context cancellation stops execution chain
func TestProperty_Internal_ContextCancellationStopsChain(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var mainRan bool

	depA := Targ(func() {})
	depB := Targ(func(ctx context.Context) error {
		return ctx.Err() // Check if already cancelled
	})
	main := Targ(func() { mainRan = true }).Deps(depA, depB)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := main.Run(ctx)

	// With cancelled context, execution may be interrupted at various points
	g.Expect(err).To(HaveOccurred())
	g.Expect(mainRan).To(BeFalse()) // Main should not run with cancelled context
}
