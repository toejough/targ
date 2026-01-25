package core_test

// core_properties_test.go contains property-based tests for execution properties.

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

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

		target := core.Targ(func(args Args) {
			received = args
		})

		err := target.Run(context.Background(), Args{Name: value})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(received.Name).To(Equal(value))
	})
}

// Property: Builder methods are chainable and idempotent on target
func TestProperty_Internal_BuilderChainableAndIdempotent(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := core.Targ(func() {})
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

// Property: Context cancellation stops execution chain
func TestProperty_Internal_ContextCancellationStopsChain(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var mainRan bool

	depA := core.Targ(func() {})
	depB := core.Targ(func(ctx context.Context) error {
		return ctx.Err() // Check if already cancelled
	})
	main := core.Targ(func() { mainRan = true }).Deps(depA, depB)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := main.Run(ctx)

	// With cancelled context, execution may be interrupted at various points
	g.Expect(err).To(HaveOccurred())
	g.Expect(mainRan).To(BeFalse()) // Main should not run with cancelled context
}

// Property: Internal dependencies track execution state correctly
func TestProperty_Internal_DependencyExecutionTracking(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		var depCount atomic.Int32

		dep := core.Targ(func() { depCount.Add(1) })

		// Create target with dependency
		target := core.Targ(func() {}).Deps(dep)

		err := target.Run(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depCount.Load()).To(Equal(int32(1)))
	})
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

			currentMax := maxConcurrent.Load()
			if current <= currentMax {
				break
			}

			if maxConcurrent.CompareAndSwap(currentMax, current) {
				break
			}
		}
	}

	a := core.Targ(func() {
		running.Add(1)
		updateMax()
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
	})

	b := core.Targ(func() {
		running.Add(1)
		updateMax()
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
	})

	target := core.Targ(func() {}).Deps(a, b, core.DepModeParallel)

	err := target.Run(context.Background())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(maxConcurrent.Load()).To(Equal(int32(2))) // Both ran concurrently
}

// Property: Timeout enforcement is accurate
func TestProperty_Internal_TimeoutEnforcement(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		timeoutMs := rapid.IntRange(10, 50).Draw(rt, "timeoutMs")
		timeout := time.Duration(timeoutMs) * time.Millisecond

		var wasTimedOut bool

		target := core.Targ(func(ctx context.Context) error {
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
