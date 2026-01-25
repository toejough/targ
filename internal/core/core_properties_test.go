package core_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestProperty_Internal(t *testing.T) {
	t.Parallel()

	t.Run("ArgsStructPopulation", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-z]{5,10}`).Draw(t, "value")

			type Args struct {
				Name string
			}

			var received Args

			target := core.Targ(func(args Args) { received = args })

			err := target.Run(context.Background(), Args{Name: value})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(received.Name).To(Equal(value))
		})
	})

	t.Run("BuilderChainableAndIdempotent", func(t *testing.T) {
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
	})

	t.Run("ContextCancellationStopsChain", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var mainRan bool

		depA := core.Targ(func() {})
		depB := core.Targ(func(ctx context.Context) error { return ctx.Err() })
		main := core.Targ(func() { mainRan = true }).Deps(depA, depB)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := main.Run(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(mainRan).To(BeFalse())
	})

	t.Run("DependencyExecutionTracking", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			var depCount atomic.Int32

			dep := core.Targ(func() { depCount.Add(1) })
			target := core.Targ(func() {}).Deps(dep)

			err := target.Run(context.Background())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(depCount.Load()).To(Equal(int32(1)))
		})
	})

	t.Run("ParallelDependencyExecution", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var running, maxConcurrent atomic.Int32

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
		g.Expect(maxConcurrent.Load()).To(Equal(int32(2)))
	})

	t.Run("TimeoutEnforcement", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			timeoutMs := rapid.IntRange(10, 50).Draw(t, "timeoutMs")
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
			g.Expect(elapsed).To(BeNumerically("<", timeout+50*time.Millisecond))
		})
	})
}
