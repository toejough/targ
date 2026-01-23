//go:build !targ

package targ_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/toejough/targ"
)

type ContinueOnErrorCmd struct{}

func (c *ContinueOnErrorCmd) Run() error {
	return targ.Deps(incrementCount, incrementCount, targ.ContinueOnError())
}

type DepsCmd struct{}

func (d *DepsCmd) Run() error {
	return targ.Deps(incrementCount, incrementCount)
}

type ParallelDepsCmd struct{}

func (p *ParallelDepsCmd) Run() error {
	return targ.Deps(incrementCount, incrementCount, targ.Parallel())
}

type ResetDepsCmd struct{}

func (r *ResetDepsCmd) Run() error {
	// First call runs the target
	err := targ.Deps(incrementCount)
	if err != nil {
		return err
	}

	if atomic.LoadInt32(&testCallCount) != 1 {
		resetTestError = "expected 1 call after first Deps"
		return nil
	}

	// Second call should skip (already ran)
	err = targ.Deps(incrementCount)
	if err != nil {
		return err
	}

	if atomic.LoadInt32(&testCallCount) != 1 {
		resetTestError = "expected still 1 call after second Deps"
		return nil
	}

	// After reset, runs again
	targ.ResetDeps()

	err = targ.Deps(incrementCount)
	if err != nil {
		return err
	}

	if atomic.LoadInt32(&testCallCount) != 2 {
		resetTestError = "expected 2 calls after ResetDeps"
		return nil
	}

	return nil
}

type WithContextCmd struct{}

func (w *WithContextCmd) Run() error {
	ctx := context.Background()
	return targ.Deps(setCalled, targ.WithContext(ctx))
}

// TestAppendBuiltinExamples verifies custom examples come before built-ins.
func TestAppendBuiltinExamples(t *testing.T) {
	custom := targ.Example{Title: "Custom", Code: "custom"}
	examples := targ.AppendBuiltinExamples(custom)

	if len(examples) != 3 {
		t.Fatalf("expected 3 examples, got %d", len(examples))
	}

	if examples[0].Title != "Custom" {
		t.Fatalf("expected first example to be custom, got %q", examples[0].Title)
	}
}

// TestBuiltinExamples verifies built-in examples are returned.
func TestBuiltinExamples(t *testing.T) {
	examples := targ.BuiltinExamples()
	if len(examples) != 2 {
		t.Fatalf("expected 2 built-in examples, got %d", len(examples))
	}
}

// TestDeps verifies basic Deps functionality runs targets exactly once.
func TestDeps(t *testing.T) {
	atomic.StoreInt32(&testCallCount, 0)

	_, err := targ.Execute([]string{"test"}, &DepsCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&testCallCount) != 1 {
		t.Fatalf("expected target to run once, got %d", testCallCount)
	}
}

// TestDeps_WithContext verifies WithContext option.
func TestDeps_WithContext(t *testing.T) {
	testCalled = false

	_, err := targ.Execute([]string{"test"}, &WithContextCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !testCalled {
		t.Fatal("expected target to be called")
	}
}

// TestDeps_WithContinueOnError verifies ContinueOnError option.
func TestDeps_WithContinueOnError(t *testing.T) {
	atomic.StoreInt32(&testCallCount, 0)

	_, err := targ.Execute([]string{"test"}, &ContinueOnErrorCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&testCallCount) != 1 {
		t.Fatalf("expected targets to run once, got %d", testCallCount)
	}
}

// TestDeps_WithParallel verifies Parallel option works with deduplication.
func TestDeps_WithParallel(t *testing.T) {
	atomic.StoreInt32(&testCallCount, 0)

	_, err := targ.Execute([]string{"test"}, &ParallelDepsCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&testCallCount) != 1 {
		t.Fatalf("expected target to run once with Parallel, got %d", testCallCount)
	}
}

// TestEmptyExamples verifies empty examples returns empty slice.
func TestEmptyExamples(t *testing.T) {
	examples := targ.EmptyExamples()
	if len(examples) != 0 {
		t.Fatalf("expected empty slice, got %d examples", len(examples))
	}
}

// TestPrependBuiltinExamples verifies built-ins come before custom examples.
func TestPrependBuiltinExamples(t *testing.T) {
	custom := targ.Example{Title: "Custom", Code: "custom"}
	examples := targ.PrependBuiltinExamples(custom)

	if len(examples) != 3 {
		t.Fatalf("expected 3 examples, got %d", len(examples))
	}

	if examples[2].Title != "Custom" {
		t.Fatalf("expected last example to be custom, got %q", examples[2].Title)
	}
}

// TestResetDeps verifies ResetDeps clears the execution cache.
func TestResetDeps(t *testing.T) {
	atomic.StoreInt32(&testCallCount, 0)

	resetTestError = ""

	_, err := targ.Execute([]string{"test"}, &ResetDepsCmd{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resetTestError != "" {
		t.Fatal(resetTestError)
	}
}

// unexported variables.
var (
	resetTestError string
	testCallCount  int32
	testCalled     bool
)

func incrementCount() {
	atomic.AddInt32(&testCallCount, 1)
}

func setCalled() {
	testCalled = true
}
