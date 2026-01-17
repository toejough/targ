package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type DepRoot struct {
	Err bool
}

func (d *DepRoot) Run() error {
	if d.Err {
		return Deps(depErr)
	}

	return Deps(depOnce, depOnce)
}

type DepStruct struct {
	Called int
}

func (d *DepStruct) Run() {
	d.Called++
}

func TestDepKeyFor_InvalidType(t *testing.T) {
	// Pass a non-func, non-pointer type (e.g., int)
	_, err := depKeyFor(42)
	if err == nil || !strings.Contains(err.Error(), "must be func or pointer") {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}

func TestDepKeyFor_NilPointer(t *testing.T) {
	var ptr *DepStruct

	_, err := depKeyFor(ptr)
	if err == nil || err.Error() != "dependency target cannot be nil" {
		t.Fatalf("expected nil pointer error, got %v", err)
	}
}

// --- depKeyFor tests ---

func TestDepKeyFor_NilTarget(t *testing.T) {
	_, err := depKeyFor(nil)
	if err == nil || err.Error() != "dependency target cannot be nil" {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDepsErrorCached(t *testing.T) {
	depCount = 0

	err := withDepTracker(context.Background(), func() error {
		node, parseErr := parseTarget(&DepRoot{})
		if parseErr != nil {
			return parseErr
		}

		runErr := node.execute(context.Background(), []string{"--err"}, RunOptions{})
		if runErr == nil {
			return errors.New("expected error")
		}

		runErr = Deps(depErr)
		if runErr == nil {
			return errors.New("expected error on second call")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if depCount != 1 {
		t.Fatalf("expected dep error to run once, got %d", depCount)
	}
}

func TestDepsRunsOnce(t *testing.T) {
	depCount = 0

	err := withDepTracker(context.Background(), func() error {
		node, parseErr := parseTarget(&DepRoot{})
		if parseErr != nil {
			return parseErr
		}

		return node.execute(context.Background(), nil, RunOptions{})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if depCount != 1 {
		t.Fatalf("expected dep to run once, got %d", depCount)
	}
}

func TestDepsStructRunsOnce(t *testing.T) {
	dep := &DepStruct{}

	err := withDepTracker(context.Background(), func() error {
		runErr := Deps(dep, dep)
		if runErr != nil {
			return runErr
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dep.Called != 1 {
		t.Fatalf("expected struct dep to run once, got %d", dep.Called)
	}
}

func TestDeps_InvalidFunctionSignature(t *testing.T) {
	// A function with multiple non-error returns fails parseTarget but passes depKeyFor
	invalidFunc := func() (int, int) { return 1, 2 }

	err := withDepTracker(context.Background(), func() error {
		return Deps(invalidFunc)
	})
	if err == nil {
		t.Fatal("expected error for invalid function signature")
	}

	if !strings.Contains(err.Error(), "return") {
		t.Fatalf("expected return type error, got %v", err)
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

	err := withDepTracker(context.Background(), func() error {
		done := make(chan error, 1)

		go func() {
			done <- Deps(bad, waiter, Parallel(), ContinueOnError())
		}()

		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			close(release)
			return errors.New("expected waiter to start")
		}

		close(release)

		return <-done
	})
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

	err := withDepTracker(context.Background(), func() error {
		done := make(chan error, 1)

		go func() {
			done <- Deps(worker, func() { worker() }, Parallel(), ContinueOnError())
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
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParallelDepsSharesDependencies(t *testing.T) {
	depCount = 0

	var runCount int32

	dep := func() { depCount++ }
	target := func() error {
		err := Deps(dep)
		if err != nil {
			return err
		}

		atomic.AddInt32(&runCount, 1)

		return nil
	}

	err := withDepTracker(context.Background(), func() error {
		return Deps(target, func() error { return target() }, Parallel(), ContinueOnError())
	})
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
	target := func() { callCount++ }

	err := withDepTracker(context.Background(), func() error {
		// First call runs the target
		err := Deps(target)
		if err != nil {
			return err
		}

		if callCount != 1 {
			return fmt.Errorf("expected 1 call, got %d", callCount)
		}

		// Second call skips (already ran)
		err = Deps(target)
		if err != nil {
			return err
		}

		if callCount != 1 {
			return fmt.Errorf("expected still 1 call, got %d", callCount)
		}

		// After reset, target runs again
		ResetDeps()

		err = Deps(target)
		if err != nil {
			return err
		}

		if callCount != 2 {
			return fmt.Errorf("expected 2 calls after reset, got %d", callCount)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSerialRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := withDepTracker(context.Background(), func() error {
		tracker := newDepTracker(ctx)
		return serialRun(ctx, tracker, []any{func() {}}, false)
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestSerialRun_ContextCancelledWithPriorError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	err := withDepTracker(context.Background(), func() error {
		tracker := newDepTracker(ctx)

		return serialRun(ctx, tracker, []any{
			func() error {
				return errors.New("prior error")
			},
			func() error {
				cancel() // Cancel after first error when using continueOnError
				return nil
			},
			func() error {
				return nil // This won't run due to cancellation
			},
		}, true) // continueOnError = true
	})

	// The error should be the prior error since it was captured before cancellation check
	if err == nil || err.Error() != "prior error" {
		t.Fatalf("expected prior error, got %v", err)
	}
}

func TestSerialRun_ContinueOnErrorAccumulates(t *testing.T) {
	firstCalled := false
	secondCalled := false

	err := withDepTracker(context.Background(), func() error {
		return Deps(
			func() error {
				firstCalled = true
				return errors.New("first error")
			},
			func() error {
				secondCalled = true
				return errors.New("second error")
			},
			ContinueOnError(),
		)
	})

	if err == nil || err.Error() != "first error" {
		t.Fatalf("expected first error to be returned, got %v", err)
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
