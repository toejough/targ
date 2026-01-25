package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Fuzz: Caret reset with arbitrary command chains.
func FuzzCaretReset_ArbitraryChains(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate a mix of command names and carets
		numCommands := rapid.IntRange(1, 10).Draw(t, "numCommands")

		args := []string{"app"}

		for range numCommands {
			if rapid.Bool().Draw(t, "isCaret") {
				args = append(args, "^")
			} else {
				args = append(args, rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "cmd"))
			}
		}

		target := targ.Targ(func() {}).Name("test")

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Glob patterns in command args.
func FuzzGlob_ArbitraryPatterns(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate patterns that look like globs
		pattern := rapid.StringMatching(`[a-z*?]{1,20}`).Draw(t, "pattern")

		target := targ.Targ(func() {}).Name("test")

		// Should not panic
		g.Expect(func() {
			_, _ = targ.ExecuteWithOptions(
				[]string{"app", pattern},
				targ.RunOptions{AllowDefault: false},
				target,
			)
		}).NotTo(Panic())
	}))
}

// Fuzz: Group name with valid patterns works.
func FuzzGroupName_ValidPatterns(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate valid group names (must match ^[a-z][a-z0-9-]*$)
		groupName := rapid.StringMatching(`[a-z][a-z0-9-]{0,10}`).Draw(t, "groupName")
		targetName := rapid.StringMatching(`[a-z][a-z0-9-]{2,10}`).Draw(t, "targetName")

		target := targ.Targ(func() {}).Name(targetName)

		// Should not panic with valid name
		g.Expect(func() {
			_ = targ.Group(groupName, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Multiple groups with arbitrary nesting.
func FuzzGroups_ArbitraryNesting(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		depth := rapid.IntRange(1, 5).Draw(t, "depth")

		// Build nested groups
		var current any = targ.Targ(func() {}).Name("leaf")

		for range depth {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "groupName")
			current = targ.Group(name, current)
		}

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app"}, current)
		}).NotTo(Panic())
	}))
}

// Fuzz: Mixed roots (targets and groups).
func FuzzMixedRoots_TargetsAndGroups(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		numRoots := rapid.IntRange(1, 5).Draw(t, "numRoots")

		roots := make([]any, 0, numRoots)
		for range numRoots {
			name := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "name")

			if rapid.Bool().Draw(t, "isGroup") {
				sub := targ.Targ(func() {}).Name("sub")
				roots = append(roots, targ.Group(name, sub))
			} else {
				roots = append(roots, targ.Targ(func() {}).Name(name))
			}
		}

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--help"}, roots...)
		}).NotTo(Panic())
	}))
}

// Fuzz: Multiple roots with arbitrary names.
func FuzzMultipleRoots_ArbitraryNames(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		numRoots := rapid.IntRange(1, 5).Draw(t, "numRoots")

		roots := make([]any, 0, numRoots)
		for range numRoots {
			name := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "name")
			roots = append(roots, targ.Targ(func() {}).Name(name))
		}

		// Pick one to call
		targetIdx := rapid.IntRange(0, numRoots-1).Draw(t, "targetIdx")

		target, ok := roots[targetIdx].(*targ.Target)
		if !ok {
			return // Skip if not a Target (shouldn't happen but satisfies linter)
		}

		targetName := target.GetName()

		// Should not panic (might error on duplicate names)
		g.Expect(func() {
			_, _ = targ.ExecuteWithOptions(
				[]string{"app", targetName},
				targ.RunOptions{AllowDefault: false},
				roots...,
			)
		}).NotTo(Panic())
	}))
}

// Fuzz: Path resolution handles arbitrary path segments.
func FuzzPathResolution_ArbitraryPathSegments(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary path segments
		segments := rapid.SliceOfN(
			rapid.String(),
			0, 10,
		).Draw(t, "segments")

		target := targ.Targ(func() {}).Name("target")

		args := append([]string{"app"}, segments...)

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Deeply nested path resolution.
func FuzzPathResolution_DeepNesting(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		depth := rapid.IntRange(1, 10).Draw(t, "depth")

		// Build deeply nested structure
		var current any = targ.Targ(func() {}).Name("leaf")

		path := []string{"leaf"}

		for range depth {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "groupName")
			current = targ.Group(name, current)
			path = append([]string{name}, path...)
		}

		// Build args to navigate the full path
		args := append([]string{"app"}, path...)

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, current)
		}).NotTo(Panic())
	}))
}

// TestGroupName_InvalidPatternsPanic tests that invalid group names panic.
// This is a table-driven test, not a fuzz test.
func TestGroupName_InvalidPatternsPanic(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {}).Name("valid-target")

	// Invalid names should panic
	invalidNames := []string{"", "123", "CamelCase", "with space", "-starts-dash"}
	for _, name := range invalidNames {
		g.Expect(func() {
			_ = targ.Group(name, target)
		}).To(Panic(), "expected panic for invalid group name: %q", name)
	}
}

// TestGroup_EmptyGroup tests that an empty group does not panic.
// This is a simple test, not a fuzz test.
func TestGroup_EmptyGroup(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	// An empty group should not panic
	g.Expect(func() {
		_ = targ.Group("empty")
	}).NotTo(Panic())
}
