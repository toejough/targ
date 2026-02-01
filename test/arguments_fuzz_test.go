// TEST-027: Argument fuzz tests - validates robustness of argument parsing
// traces: ARCH-001

package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Fuzz: Boolean flag parsing handles arbitrary strings.
func FuzzBoolFlag_ArbitraryStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(t, "value")

		type Args struct {
			Verbose bool `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic - bool flags might accept or reject the value
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--verbose", value}, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Execute handles arbitrary CLI args without panicking.
func FuzzExecute_ArbitraryCLIArgs(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary CLI args (may be invalid)
		args := rapid.SliceOfN(rapid.String(), 0, 20).Draw(t, "args")

		target := targ.Targ(func() {})

		// Should not panic - either succeeds or returns error
		g.Expect(func() {
			_, _ = targ.Execute(append([]string{"app"}, args...), target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Execute handles arbitrary flag names without panicking.
func FuzzExecute_ArbitraryFlagNames(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary flag-like strings
		flagCount := rapid.IntRange(0, 5).Draw(t, "flagCount")
		args := []string{"app"}

		for range flagCount {
			flag := rapid.StringMatching(`--?[a-zA-Z0-9_-]*`).Draw(t, "flag")
			args = append(args, flag)

			// Sometimes add a value
			if rapid.Bool().Draw(t, "hasValue") {
				value := rapid.String().Draw(t, "value")
				args = append(args, value)
			}
		}

		target := targ.Targ(func() {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Execute handles arbitrary struct field names without panicking.
func FuzzExecute_ArbitraryFlagValues(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary values for known flags
		value := rapid.String().Draw(t, "value")

		type Args struct {
			Name string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic with any string value
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--name", value}, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Integer parsing handles arbitrary strings.
func FuzzIntFlag_ArbitraryStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(t, "value")

		type Args struct {
			Count int `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--count", value}, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Map flag parsing handles arbitrary key=value strings.
func FuzzMapFlag_ArbitraryKeyValueStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		// Generate arbitrary strings that might look like key=value
		value := rapid.String().Draw(t, "value")

		type Args struct {
			Labels map[string]string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--labels", value}, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Slice flag parsing handles arbitrary repeated values.
func FuzzSliceFlag_ArbitraryValues(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		values := rapid.SliceOfN(rapid.String(), 0, 10).Draw(t, "values")

		type Args struct {
			Tags []string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		args := make([]string, 0, 1+2*len(values))

		args = append(args, "app")
		for _, v := range values {
			args = append(args, "--tags", v)
		}

		// Should not panic
		g.Expect(func() {
			_, _ = targ.Execute(args, target)
		}).NotTo(Panic())
	}))
}

// Fuzz: Target name handles special characters.
func FuzzTargetName_ArbitraryStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		name := rapid.String().Draw(t, "name")

		// Empty string is invalid and handled by Targ validation
		if name == "" {
			return
		}

		// Should not panic when setting arbitrary name
		g.Expect(func() {
			_ = targ.Targ(func() {}).Name(name)
		}).NotTo(Panic())
	}))
}

// Fuzz: Timeout duration parsing handles arbitrary strings.
func FuzzTimeoutFlag_ArbitraryStrings(f *testing.F) {
	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		g := NewWithT(t)

		value := rapid.String().Draw(t, "value")

		target := targ.Targ(func() {})

		// Should not panic - either parses or returns error
		g.Expect(func() {
			_, _ = targ.Execute([]string{"app", "--timeout", value}, target)
		}).NotTo(Panic())
	}))
}
