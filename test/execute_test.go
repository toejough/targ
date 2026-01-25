package targ_test

// LEGACY_TESTS: This file contains tests being evaluated for redundancy.
// Property-based replacements are in *_properties_test.go files.
// Do not add new tests here. See docs/test-migration.md for details.

import (
	"context"
	"testing"
	"time"

	"github.com/toejough/targ"
)

// Args struct types for Target functions (these stay - they have no Run() method).

type InterleavedIntArgs struct {
	Values []targ.Interleaved[int] `targ:"flag"`
}

func TestInterleavedFlags_IntType(t *testing.T) {
	t.Parallel()

	var gotValues []targ.Interleaved[int]

	target := targ.Targ(func(args InterleavedIntArgs) {
		gotValues = args.Values
	})

	_, err := targ.Execute([]string{"app", "--values", "10", "--values", "20"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotValues) != 2 {
		t.Fatalf("expected 2 values, got %d", len(gotValues))
	}

	if gotValues[0].Value != 10 || gotValues[0].Position != 0 {
		t.Fatalf("expected values[0]={10,0}, got %+v", gotValues[0])
	}

	if gotValues[1].Value != 20 || gotValues[1].Position != 1 {
		t.Fatalf("expected values[1]={20,1}, got %+v", gotValues[1])
	}
}

func TestStringTarget_FailingCommand(t *testing.T) {
	t.Parallel()

	// A string target that exits with error
	target := targ.Targ("exit 1").Name("fail-cmd")

	_, err := targ.Execute([]string{"app"}, target)
	if err == nil {
		t.Fatal("expected error from failing shell command")
	}
}

func TestStringTarget_WithVars_MissingRequiredFlag(t *testing.T) {
	t.Parallel()

	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Missing --port should fail
	_, err := targ.Execute([]string{"app", "--name", "hello"}, target)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestStringTarget_WithVars_ShortFlags(t *testing.T) {
	t.Parallel()

	// First letters should become short flags
	target := targ.Targ("echo $name $port").Name("test-cmd")

	// Use short flags -n and -p
	result, err := targ.Execute([]string{"app", "-n", "hello", "-p", "8080"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v, output: %s", err, result.Output)
	}
}

func TestTimeout_EqualsStyle(t *testing.T) {
	t.Parallel()

	called := false

	target := targ.Targ(func(ctx context.Context) error {
		called = true

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return nil
		}
	}).Name("timeout-cmd")

	_, err := targ.Execute([]string{"app", "--timeout=1s"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected command to be called")
	}
}
