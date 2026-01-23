package targ_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestTarg_AcceptsFunction(t *testing.T) {
	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		// Create a no-op function
		fn := func() {}
		target := targ.Targ(fn)

		g.Expect(target).NotTo(BeNil())
		g.Expect(target.Fn()).NotTo(BeNil())
	})
}

func TestTarg_AcceptsString(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate a non-empty command string
		cmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(rt, "cmd")
		target := targ.Targ(cmd)

		g.Expect(target).NotTo(BeNil())
		g.Expect(target.Fn()).To(Equal(cmd))
	})
}

func TestTarg_PanicsOnEmptyString(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ("")
	}).To(Panic())
}

func TestTarg_PanicsOnNil(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(nil)
	}).To(Panic())
}

func TestTarg_PanicsOnNonFuncNonString(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(42) // int is not func or string
	}).To(Panic())
}

func TestTarget_BackoffBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterBackoff := original.Backoff(time.Second, 2.0)

	g.Expect(afterBackoff).To(BeIdenticalTo(original))
}

func TestTarget_BackoffDelaysAfterFailure(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	start := time.Now()
	target := targ.Targ(func() error {
		execCount++

		return errors.New("fail")
	}).Times(3).Retry().Backoff(50*time.Millisecond, 2.0)

	err := target.Run(context.Background())
	elapsed := time.Since(start)

	g.Expect(err).To(HaveOccurred())
	g.Expect(execCount).To(Equal(3))
	// Should have delays: 50ms after first, 100ms after second = 150ms total
	g.Expect(elapsed).To(BeNumerically(">=", 100*time.Millisecond))
}

func TestTarget_BuilderChainWithDepsAndTimeout(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	dep := targ.Targ(func() { order = append(order, "dep") })
	main := targ.Targ(func() { order = append(order, "main") }).
		Name("test").
		Description("test target").
		Deps(dep).
		Timeout(time.Second)

	err := main.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(order).To(Equal([]string{"dep", "main"}))
}

func TestTarget_BuilderChainsPreserveSettings(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[a-z]+`).Draw(rt, "name")
		desc := rapid.StringMatching(`[a-zA-Z ]+`).Draw(rt, "desc")

		target := targ.Targ(func() {}).Name(name).Description(desc)

		g.Expect(target.GetName()).To(Equal(name))
		g.Expect(target.GetDescription()).To(Equal(desc))
	})
}

func TestTarget_BuilderMethodsReturnSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterName := original.Name("test")
	afterDesc := afterName.Description("test desc")

	// All should be the same pointer
	g.Expect(afterName).To(BeIdenticalTo(original))
	g.Expect(afterDesc).To(BeIdenticalTo(original))
}

func TestTarget_CacheBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterCache := original.Cache("*.go")

	g.Expect(afterCache).To(BeIdenticalTo(original))
}

func TestTarget_CacheDisabledSetsFlag(t *testing.T) {
	g := NewWithT(t)

	target := targ.Targ(func() {}).Cache(targ.Disabled)

	// GetConfig returns (watch, cache, watchDisabled, cacheDisabled)
	_, cache, _, cacheDisabled := target.GetConfig()

	g.Expect(cacheDisabled).To(BeTrue())
	g.Expect(cache).To(BeNil()) // Patterns cleared when disabled
}

func TestTarget_CacheHitSkipsExecution(t *testing.T) {
	g := NewWithT(t)

	// Create temp dir and file for cache testing
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/input.txt"
	cacheDir := tmpDir + "/cache"

	err := os.WriteFile(inputFile, []byte("content"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	execCount := 0
	target := targ.Targ(func() { execCount++ }).
		Cache(inputFile).
		CacheDir(cacheDir)

	// First run - cache miss, should execute
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))

	// Second run - cache hit, should skip
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))
}

func TestTarget_CacheMissRunsExecution(t *testing.T) {
	g := NewWithT(t)

	// Create temp dir and file for cache testing
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/input.txt"
	cacheDir := tmpDir + "/cache"

	err := os.WriteFile(inputFile, []byte("content1"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	execCount := 0
	target := targ.Targ(func() { execCount++ }).
		Cache(inputFile).
		CacheDir(cacheDir)

	// First run
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))

	// Change file content
	err = os.WriteFile(inputFile, []byte("content2"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	// Second run - cache miss due to content change
	err = target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(2))
}

func TestTarget_DepsRunSerially(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	a := targ.Targ(func() { order = append(order, "a") })
	b := targ.Targ(func() { order = append(order, "b") })
	c := targ.Targ(func() { order = append(order, "c") }).Deps(a, b)

	err := c.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(order).To(Equal([]string{"a", "b", "c"}))
}

func TestTarget_DepsStopOnError(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	a := targ.Targ(func() { order = append(order, "a") })
	b := targ.Targ(func() error {
		order = append(order, "b")
		return errors.New("b failed")
	})
	c := targ.Targ(func() { order = append(order, "c") }).Deps(a, b)

	err := c.Run(context.Background())
	g.Expect(err).To(MatchError(ContainSubstring("b failed")))
	g.Expect(order).To(Equal([]string{"a", "b"}))
}

func TestTarget_ParallelDepsRunConcurrently(t *testing.T) {
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
	c := targ.Targ(func() {}).ParallelDeps(a, b)

	go func() {
		_ = c.Run(context.Background())
	}()

	// Both should start before either completes
	<-aStarted
	<-bStarted
	close(done)

	// Give the main goroutine time to complete
	g.Eventually(func() bool { return true }).Should(BeTrue())
}

func TestTarget_RetryBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterRetry := original.Retry()

	g.Expect(afterRetry).To(BeIdenticalTo(original))
}

func TestTarget_RunCallsFunction(t *testing.T) {
	g := NewWithT(t)

	called := false
	target := targ.Targ(func() {
		called = true
	})

	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

func TestTarget_RunPassesContext(t *testing.T) {
	g := NewWithT(t)

	var receivedValue any

	target := targ.Targ(func(ctx context.Context) {
		receivedValue = ctx.Value(testContextKey)
	})

	ctx := context.WithValue(context.Background(), testContextKey, "value")
	err := target.Run(ctx)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(receivedValue).To(Equal("value"))
}

func TestTarget_RunReturnsError(t *testing.T) {
	g := NewWithT(t)

	expectedErr := errors.New("test error")
	target := targ.Targ(func() error {
		return expectedErr
	})

	err := target.Run(context.Background())
	g.Expect(err).To(MatchError(expectedErr))
}

func TestTarget_RunWithArgs(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Value string
	}

	var received Args

	target := targ.Targ(func(args Args) {
		received = args
	})

	err := target.Run(context.Background(), Args{Value: "test"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(received.Value).To(Equal("test"))
}

func TestTarget_RunWithContextAndArgs(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Value string
	}

	var receivedCtxValue any

	var receivedArgs Args

	target := targ.Targ(func(ctx context.Context, args Args) error {
		receivedCtxValue = ctx.Value(testContextKey)
		receivedArgs = args

		return nil
	})

	ctx := context.WithValue(context.Background(), testContextKey, "value")
	err := target.Run(ctx, Args{Value: "test"})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(receivedCtxValue).To(Equal("value"))
	g.Expect(receivedArgs.Value).To(Equal("test"))
}

func TestTarget_RunWithWatchRerunsOnFileChange(t *testing.T) {
	g := NewWithT(t)

	// Create temp dir and file for watch testing
	tmpDir := t.TempDir()
	inputFile := tmpDir + "/input.txt"

	err := os.WriteFile(inputFile, []byte("content"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	var execCount atomic.Int32

	target := targ.Targ(func() { execCount.Add(1) }).
		Watch(inputFile)

	// Run in a goroutine with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- target.Run(ctx)
	}()

	// Wait for initial run
	g.Eventually(execCount.Load).Should(Equal(int32(1)))

	// Modify the file to trigger re-run
	err = os.WriteFile(inputFile, []byte("content2"), 0o644)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for re-run
	g.Eventually(execCount.Load, "2s").Should(Equal(int32(2)))

	// Cancel and verify it stops
	cancel()

	err = <-done
	g.Expect(err).To(HaveOccurred()) // Should error on context cancellation
}

func TestTarget_RunWithoutWatchRunsOnce(t *testing.T) {
	g := NewWithT(t)

	// Without watch patterns, Run should just run once and return
	execCount := 0
	target := targ.Targ(func() { execCount++ })

	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(1))
}

func TestTarget_ShellCommandExecution(t *testing.T) {
	g := NewWithT(t)

	// Simple shell command that should succeed
	target := targ.Targ("echo hello")
	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestTarget_ShellCommandFails(t *testing.T) {
	g := NewWithT(t)

	// Command that should fail
	target := targ.Targ("exit 1")
	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
}

func TestTarget_TimeoutCancelsExecution(t *testing.T) {
	g := NewWithT(t)

	target := targ.Targ(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}).Timeout(10 * time.Millisecond)

	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
}

func TestTarget_TimesBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterTimes := original.Times(5)

	g.Expect(afterTimes).To(BeIdenticalTo(original))
}

func TestTarget_TimesCompletesAllWithRetry(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	target := targ.Targ(func() error {
		execCount++

		return errors.New("always fail")
	}).Times(5).Retry()

	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred()) // Returns last error
	g.Expect(execCount).To(Equal(5)) // All iterations ran
}

func TestTarget_TimesRunsNTimes(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	target := targ.Targ(func() { execCount++ }).Times(5)

	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(5))
}

func TestTarget_TimesStopsOnContextCancellation(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	target := targ.Targ(func() {
		execCount++
		if execCount == 2 {
			cancel()
		}
	}).Times(10)

	err := target.Run(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(execCount).To(Equal(2)) // Stopped after cancellation
}

func TestTarget_TimesStopsOnFailureWithoutRetry(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	target := targ.Targ(func() error {
		execCount++
		if execCount == 3 {
			return errors.New("fail at 3")
		}

		return nil
	}).Times(5)

	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(execCount).To(Equal(3)) // Stopped at failure
}

func TestTarget_WatchBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterWatch := original.Watch("*.go")

	g.Expect(afterWatch).To(BeIdenticalTo(original))
}

func TestTarget_WatchDisabledSetsFlag(t *testing.T) {
	g := NewWithT(t)

	target := targ.Targ(func() {}).Watch(targ.Disabled)

	// GetConfig returns (watch, cache, watchDisabled, cacheDisabled)
	watch, _, watchDisabled, _ := target.GetConfig()

	g.Expect(watchDisabled).To(BeTrue())
	g.Expect(watch).To(BeNil()) // Patterns cleared when disabled
}

func TestTarget_WhileBuilderReturnsSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterWhile := original.While(func() bool { return true })

	g.Expect(afterWhile).To(BeIdenticalTo(original))
}

func TestTarget_WhileStopsWhenPredicateFalse(t *testing.T) {
	g := NewWithT(t)

	execCount := 0
	target := targ.Targ(func() { execCount++ }).
		Times(10).
		While(func() bool { return execCount < 3 })

	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(execCount).To(Equal(3)) // While stopped it at 3
}

// unexported constants.
const (
	testContextKey contextKey = "key"
)

type contextKey string
