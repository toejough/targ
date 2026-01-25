package targ_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Fuzz: Backoff parameters handle various values.
func FuzzBackoff_ArbitraryParameters(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		durationMs := rapid.IntRange(-100, 100).Draw(t, "durationMs")
		duration := time.Duration(durationMs) * time.Millisecond
		multiplier := rapid.Float64Range(-2.0, 5.0).Draw(t, "multiplier")

		// Should not panic
		g.Expect(func() {
			_ = targ.Targ(func() {}).Backoff(duration, multiplier)
		}).NotTo(Panic())
	}))
}

// Fuzz: Multiple builder calls in arbitrary order.
func FuzzBuilderChain_ArbitraryOrder(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		target := targ.Targ(func() {})

		// Apply random builder methods in random order
		ops := rapid.IntRange(0, 10).Draw(t, "numOps")
		for range ops {
			op := rapid.IntRange(0, 5).Draw(t, "op")
			switch op {
			case 0:
				target = target.Name(rapid.String().Draw(t, "name"))
			case 1:
				target = target.Description(rapid.String().Draw(t, "desc"))
			case 2:
				target = target.Cache(rapid.String().Draw(t, "cache"))
			case 3:
				target = target.Watch(rapid.String().Draw(t, "watch"))
			case 4:
				target = target.Retry()
			case 5:
				target = target.Times(rapid.IntRange(1, 5).Draw(t, "times"))
			}
		}

		// Should not panic regardless of order
		g.Expect(target).NotTo(BeNil())
	}))
}

// Fuzz: Cache pattern handles arbitrary glob patterns.
func FuzzCache_ArbitraryPatterns(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		pattern := rapid.String().Draw(t, "pattern")

		// Should not panic
		g.Expect(func() {
			_ = targ.Targ(func() {}).Cache(pattern)
		}).NotTo(Panic())
	}))
}

// Fuzz: Deps handles nil and empty dependencies.
func FuzzDeps_ArbitraryDependencies(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		numDeps := rapid.IntRange(0, 5).Draw(t, "numDeps")

		deps := make([]any, 0, numDeps)
		for range numDeps {
			deps = append(deps, targ.Targ(func() {}))
		}

		// Should not panic
		g.Expect(func() {
			_ = targ.Targ(func() {}).Deps(deps...)
		}).NotTo(Panic())
	}))
}

// Fuzz: Description handles arbitrary strings.
func FuzzDescription_ArbitraryStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		desc := rapid.String().Draw(t, "desc")

		// Should not panic
		g.Expect(func() {
			_ = targ.Targ(func() {}).Description(desc)
		}).NotTo(Panic())
	}))
}

// Fuzz: Shell command handles arbitrary command strings.
func FuzzShellCommand_ArbitraryCommandStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary shell command (limited to safe characters)
		cmd := rapid.StringMatching(`[a-zA-Z0-9 _-]{1,50}`).Draw(t, "cmd")

		// Should not panic - either succeeds or returns error
		g.Expect(func() {
			target := targ.Targ(cmd)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_ = target.Run(ctx)
		}).NotTo(Panic())
	}))
}

// Fuzz: Timeout parameter handles various durations.
func FuzzTimeout_ArbitraryDurations(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate durations including negative ones
		durationMs := rapid.IntRange(-1000, 1000).Draw(t, "durationMs")
		duration := time.Duration(durationMs) * time.Millisecond

		// Should not panic
		g.Expect(func() {
			target := targ.Targ(func() {}).Timeout(duration)

			// Only run if duration is reasonable
			if duration > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()

				_ = target.Run(ctx)
			}
		}).NotTo(Panic())
	}))
}

// Fuzz: Times parameter handles various values.
func FuzzTimes_ArbitraryValues(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		times := rapid.IntRange(-10, 100).Draw(t, "times")

		// Should not panic
		g.Expect(func() {
			target := targ.Targ(func() {}).Times(times)

			// Only run if times is positive to avoid hangs
			if times > 0 && times <= 10 {
				_ = target.Run(context.Background())
			}
		}).NotTo(Panic())
	}))
}

// Fuzz: Watch pattern handles arbitrary glob patterns.
func FuzzWatch_ArbitraryPatterns(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		pattern := rapid.String().Draw(t, "pattern")

		// Should not panic
		g.Expect(func() {
			_ = targ.Targ(func() {}).Watch(pattern)
		}).NotTo(Panic())
	}))
}

// Fuzz: While condition handles various predicates.
func FuzzWhile_ArbitraryPredicates(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate a predicate that stops after some calls
		maxCalls := rapid.IntRange(0, 5).Draw(t, "maxCalls")
		callCount := 0

		target := targ.Targ(func() { callCount++ }).
			Times(10).
			While(func() bool { return callCount < maxCalls })

		// Should not panic and should respect the predicate
		g.Expect(func() {
			_ = target.Run(context.Background())
		}).NotTo(Panic())

		// If maxCalls is 0, callCount stays 0 because while is checked first
		if maxCalls == 0 {
			g.Expect(callCount).To(Equal(0))
		} else {
			g.Expect(callCount).To(BeNumerically("<=", maxCalls))
		}
	}))
}
