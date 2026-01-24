//go:build !targ

package targ_test

import (
	"testing"

	"github.com/toejough/targ"
)

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
