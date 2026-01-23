package core_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/toejough/targ/internal/core"
)

// Test args struct for dependency execution.

type DepRootArgs struct {
	Err bool `targ:"flag"`
}

func TestDepsErrorCached(t *testing.T) {
	depCount = 0

	// Run through Execute which sets up the dep tracker
	target := core.Targ(func(args DepRootArgs) error {
		if args.Err {
			return core.Deps(depErr)
		}
		return core.Deps(depOnce, depOnce)
	})
	_, err := core.Execute([]string{"cmd", "--err"}, target)
	if err == nil {
		t.Fatal("expected error")
	}

	if depCount != 1 {
		t.Fatalf("expected dep error to run once, got %d", depCount)
	}
}

func TestDepsRunsOnce(t *testing.T) {
	depCount = 0

	target := core.Targ(func(_ DepRootArgs) error {
		return core.Deps(depOnce, depOnce)
	})
	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if depCount != 1 {
		t.Fatalf("expected dep to run once, got %d", depCount)
	}
}

func TestDepsTargetRunsOnce(t *testing.T) {
	var depCalled int
	depTarget := core.Targ(func() { depCalled++ }).Name("dep")
	target := core.Targ(func() error {
		return core.Deps(depTarget, depTarget)
	})

	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if depCalled != 1 {
		t.Fatalf("expected target dep to run once, got %d", depCalled)
	}
}

func TestDeps_ConcurrentSameDepWaitsForFirst(t *testing.T) {
	// Test that when two goroutines request the same dep,
	// the second one waits for the first to complete (inFlight path)
	running := make(chan struct{})
	finish := make(chan struct{})

	var count int32

	slowDep := func() {
		atomic.AddInt32(&count, 1)

		running <- struct{}{} // Signal we're running

		<-finish // Wait for signal to finish
	}

	target := func() error {
		done := make(chan error, 2)

		// Start first call - it will block in slowDep
		go func() {
			done <- core.Deps(slowDep)
		}()

		// Wait for slowDep to start
		<-running

		// Start second call - should wait on inFlight channel
		go func() {
			done <- core.Deps(slowDep)
		}()

		// Give second goroutine time to hit the inFlight path
		time.Sleep(10 * time.Millisecond)

		// Let the first call complete
		close(finish)

		// Both should complete
		for range 2 {
			if err := <-done; err != nil {
				return err
			}
		}

		return nil
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected dep to run once, got %d", count)
	}
}

func TestDeps_InvalidFunctionSignature(t *testing.T) {
	// A function with multiple non-error returns fails parseTarget
	invalidFunc := func() (int, int) { return 1, 2 }

	target := func() error {
		return core.Deps(invalidFunc)
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error for invalid function signature")
	}
}

func TestDeps_InvalidTypeViaDeps(t *testing.T) {
	// Pass an invalid type through Deps to exercise the error path
	target := func() error {
		return core.Deps(42) // int is not a valid target type
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestDeps_NilPointerTarget(t *testing.T) {
	// Pass a typed nil pointer through Deps
	var nilPtr *core.Target

	target := func() error {
		return core.Deps(nilPtr)
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error for nil pointer target")
	}
}

func TestDeps_NilTarget(t *testing.T) {
	// Pass nil directly through Deps
	target := func() error {
		return core.Deps(nil)
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestParallelDepsReturnsError(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	bad := func() error { return errors.New("boom") }
	waiter := func() {
		started <- struct{}{}

		<-release
	}

	target := func() error {
		done := make(chan error, 1)

		go func() {
			done <- core.Deps(bad, waiter, core.Parallel(), core.ContinueOnError())
		}()

		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			close(release)

			return errors.New("expected waiter to start")
		}

		close(release)

		return <-done
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParallelDepsRunsConcurrently(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	worker := func() {
		started <- struct{}{}

		<-release
	}

	target := func() error {
		done := make(chan error, 1)

		go func() {
			done <- core.Deps(worker, func() { worker() }, core.Parallel(), core.ContinueOnError())
		}()

		timeout := time.After(200 * time.Millisecond)

		for range 2 {
			select {
			case <-started:
			case <-timeout:
				close(release)

				return errors.New("expected both tasks to start concurrently")
			}
		}

		close(release)

		return <-done
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParallelDepsSharesDependencies(t *testing.T) {
	depCount = 0

	var runCount int32

	dep := func() { depCount++ }
	inner := func() error {
		err := core.Deps(dep)
		if err != nil {
			return err
		}

		atomic.AddInt32(&runCount, 1)

		return nil
	}

	target := func() error {
		return core.Deps(
			inner,
			func() error { return inner() },
			core.Parallel(),
			core.ContinueOnError(),
		)
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if depCount != 1 {
		t.Fatalf("expected shared dep to run once, got %d", depCount)
	}

	if runCount != 2 {
		t.Fatalf("expected both targets to run, got %d", runCount)
	}
}

func TestResetDeps(t *testing.T) {
	callCount := 0
	inner := func() { callCount++ }

	target := func() error {
		// First call runs the target
		err := core.Deps(inner)
		if err != nil {
			return err
		}

		if callCount != 1 {
			return fmt.Errorf("expected 1 call, got %d", callCount)
		}

		// Second call skips (already ran)
		err = core.Deps(inner)
		if err != nil {
			return err
		}

		if callCount != 1 {
			return fmt.Errorf("expected still 1 call, got %d", callCount)
		}

		// After reset, target runs again
		core.ResetDeps()

		err = core.Deps(inner)
		if err != nil {
			return err
		}

		if callCount != 2 {
			return fmt.Errorf("expected 2 calls after reset, got %d", callCount)
		}

		return nil
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSerialDeps_ContextCanceledNoFirstError(t *testing.T) {
	// Test that when context is canceled and no prior errors,
	// the context canceled error is returned
	secondRan := false

	target := func() error {
		ctx, cancel := context.WithCancel(context.Background())

		return core.Deps(
			func() {
				// First target succeeds but cancels context
				cancel()
			},
			func() {
				// Second target won't run due to cancellation
				secondRan = true
			},
			core.WithContext(ctx),
		)
	}

	result, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error")
	}

	// Second target should not run since context was canceled
	if secondRan {
		t.Fatal("second target should not have run")
	}

	// Output should contain context canceled error
	if !strings.Contains(result.Output, "context canceled") {
		t.Fatalf("expected output to contain context canceled, got: %v", result.Output)
	}
}

func TestSerialDeps_ContextCanceledWithFirstError(t *testing.T) {
	// Test that when context is canceled AND there's a firstErr from ContinueOnError,
	// the firstErr is returned (not the context error)
	thirdRan := false

	target := func() error {
		ctx, cancel := context.WithCancel(context.Background())

		return core.Deps(
			func() error {
				return errors.New("first error")
			},
			func() {
				// Cancel context during second target
				cancel()
			},
			func() {
				// Third target won't run due to cancellation check
				thirdRan = true
			},
			core.ContinueOnError(),
			core.WithContext(ctx),
		)
	}

	result, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error")
	}

	// Third target should not run since context was canceled before it
	if thirdRan {
		t.Fatal("third target should not have run")
	}

	// Output should contain the first error
	if !strings.Contains(result.Output, "first error") {
		t.Fatalf("expected output to contain first error, got: %v", result.Output)
	}
}

func TestSerialDeps_ContinueOnErrorAccumulates(t *testing.T) {
	firstCalled := false
	secondCalled := false

	target := func() error {
		return core.Deps(
			func() error {
				firstCalled = true

				return errors.New("first error")
			},
			func() error {
				secondCalled = true

				return errors.New("second error")
			},
			core.ContinueOnError(),
		)
	}

	_, err := core.Execute([]string{"cmd"}, target)
	if err == nil {
		t.Fatal("expected error")
	}

	if !firstCalled || !secondCalled {
		t.Fatalf("expected both to be called: first=%v, second=%v", firstCalled, secondCalled)
	}
}

// unexported variables.
var (
	depCount int
)

func depErr() error {
	depCount++

	return errors.New("boom")
}

func depOnce() {
	depCount++
}
