package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Fuzz: Execute handles arbitrary CLI args without panicking
func TestFuzz_Execute_ArbitraryCLIArgs(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary CLI args (may be invalid)
		args := rapid.SliceOfN(rapid.String(), 0, 20).Draw(rt, "args")

		target := targ.Targ(func() {})

		// Should not panic - either succeeds or returns error
		g.Expect(func() {
			_, _ = targ.Execute(append([]string{"app"}, args...), target)
		}).NotTo(Panic())
	})
}

// Fuzz: Execute handles arbitrary flag names without panicking
func TestFuzz_Execute_ArbitraryFlagNames(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary flag-like strings
		flagCount := rapid.IntRange(0, 5).Draw(rt, "flagCount")
		args := []string{"app"}

		for range flagCount {
			flag := rapid.StringMatching(`--?[a-zA-Z0-9_-]*`).Draw(rt, "flag")
			args = append(args, flag)

			// Sometimes add a value
			if rapid.Bool().Draw(rt, "hasValue") {
				value := rapid.String().Draw(rt, "value")
				args = append(args, value)
			}
		}

		target := targ.Targ(func() {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Execute handles arbitrary struct field names without panicking
func TestFuzz_Execute_ArbitraryFlagValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary values for known flags
		value := rapid.String().Draw(rt, "value")

		type Args struct {
			Name string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic with any string value
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--name", value}, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Map flag parsing handles arbitrary key=value strings
func TestFuzz_MapFlag_ArbitraryKeyValueStrings(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary strings that might look like key=value
		value := rapid.String().Draw(rt, "value")

		type Args struct {
			Labels map[string]string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--labels", value}, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Integer parsing handles arbitrary strings
func TestFuzz_IntFlag_ArbitraryStrings(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(rt, "value")

		type Args struct {
			Count int `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--count", value}, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Boolean flag parsing handles arbitrary strings
func TestFuzz_BoolFlag_ArbitraryStrings(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(rt, "value")

		type Args struct {
			Verbose bool `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic - bool flags might accept or reject the value
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--verbose", value}, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Slice flag parsing handles arbitrary repeated values
func TestFuzz_SliceFlag_ArbitraryValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		values := rapid.SliceOfN(rapid.String(), 0, 10).Draw(rt, "values")

		type Args struct {
			Tags []string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		args := []string{"app"}
		for _, v := range values {
			args = append(args, "--tags", v)
		}

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	})
}

// Fuzz: Target name handles special characters
func TestFuzz_TargetName_ArbitraryStrings(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.String().Draw(rt, "name")

		// Empty string is invalid and handled by Targ validation
		if name == "" {
			return
		}

		// Should not panic when setting arbitrary name
		g.Expect(func() {
			_ = targ.Targ(func() {}).Name(name)
		}).NotTo(Panic())
	})
}

// Fuzz: Timeout duration parsing handles arbitrary strings
func TestFuzz_TimeoutFlag_ArbitraryStrings(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(rt, "value")

		target := targ.Targ(func() {})

		// Should not panic - either parses or returns error
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--timeout", value}, target)
		}).NotTo(Panic())
	})
}
