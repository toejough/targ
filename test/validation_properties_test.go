package targ_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestProperty_UsageLine(t *testing.T) {
	t.Parallel()

	t.Run("UsageLineShowsPositionalArgs", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Input  string `targ:"positional"`
			Output string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("convert")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Positional args shown in usage line with capitalized names
		g.Expect(result.Output).To(ContainSubstring("Input"))
		g.Expect(result.Output).To(ContainSubstring("Output"))
	})

	t.Run("UsageLineShowsOptionalPositional", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Required string `targ:"positional"`
			Optional string `targ:"positional,optional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Usage line shows positional args; optional ones in brackets
		g.Expect(result.Output).To(ContainSubstring("Required"))
		g.Expect(result.Output).To(ContainSubstring("[Optional"))
	})

	t.Run("UsageLineShowsVariadicPositional", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files []string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("process")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Variadic positionals shown with capitalized name and ...
		g.Expect(result.Output).To(ContainSubstring("Files"))
	})

	t.Run("LongEnumValuesWrapInHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Long enum (>40 chars) should wrap across multiple lines
		type Args struct {
			Format string `targ:"flag,enum=long-option-one|another-long|yet-another|final-opt"`
		}

		target := targ.Targ(func(_ Args) {}).Name("export")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// All enum values should appear in help
		g.Expect(result.Output).To(ContainSubstring("long-option-one"))
		g.Expect(result.Output).To(ContainSubstring("another-long"))
		g.Expect(result.Output).To(ContainSubstring("yet-another"))
		g.Expect(result.Output).To(ContainSubstring("final-opt"))
	})

	t.Run("ShortEnumValuesInlineInHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Short enum (<40 chars) should be inline
		type Args struct {
			Format string `targ:"flag,enum=json|xml|csv"`
		}

		target := targ.Targ(func(_ Args) {}).Name("export")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// All enum values should appear in help
		g.Expect(result.Output).To(ContainSubstring("json"))
		g.Expect(result.Output).To(ContainSubstring("xml"))
		g.Expect(result.Output).To(ContainSubstring("csv"))
	})

	t.Run("FlagWithNoUsageShowsJustName", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--verbose"))
	})
}

func TestProperty_Validation(t *testing.T) {
	t.Parallel()

	t.Run("FunctionReturningErrorIsValid", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() error { return nil }).Name("valid")

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("NiladicFunctionIsValid", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("niladic")

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("FunctionWithContextIsValid", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Functions accepting context are valid
		target := targ.Targ(func(_ context.Context) error { return nil }).Name("ctx")

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("NonFunctionPanics", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Passing a non-function should panic
		g.Expect(func() {
			targ.Targ(42)
		}).To(Panic())
	})

	t.Run("EmptyStringPanics", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Empty string is invalid
		g.Expect(func() {
			targ.Targ("")
		}).To(Panic())
	})

	// Tests for validateFuncType - raw functions passed to Execute
	// Invalid functions print error to output and are skipped

	t.Run("RawFunctionTooManyInputsReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Raw function with 2 inputs should report validation error
		result, _ := targ.Execute(
			[]string{"app"},
			func(_ context.Context, _ string) {},
		)
		g.Expect(result.Output).To(ContainSubstring("must be niladic or accept context"))
	})

	t.Run("RawFunctionNonContextInputReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Raw function with non-context single input should report validation error
		result, _ := targ.Execute(
			[]string{"app"},
			func(_ int) {},
		)
		g.Expect(result.Output).To(ContainSubstring("must accept context.Context"))
	})

	t.Run("RawFunctionWrongReturnTypeReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Raw function returning non-error type should report validation error
		result, _ := targ.Execute(
			[]string{"app"},
			func() int { return 42 },
		)
		g.Expect(result.Output).To(ContainSubstring("must return only error"))
	})

	t.Run("RawFunctionMultipleReturnsReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Raw function with multiple returns should report validation error
		result, _ := targ.Execute(
			[]string{"app"},
			func() (int, error) { return 0, nil },
		)
		g.Expect(result.Output).To(ContainSubstring("must return only error"))
	})

	t.Run("RawValidFunctionWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		// Valid raw function should work
		_, err := targ.Execute(
			[]string{"app"},
			func() { executed = true },
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeTrue())
	})

	// Tests for Target-wrapped functions with validation errors
	t.Run("TargetWrongReturnTypeReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Target with function returning non-error type
		target := targ.Targ(func() int { return 42 }).Name("bad-return")

		result, _ := targ.Execute([]string{"app", "bad-return"}, target)
		g.Expect(result.Output).To(ContainSubstring("must return only error"))
	})

	t.Run("TargetMultipleReturnsReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Target with function returning multiple values
		target := targ.Targ(func() (int, error) { return 0, nil }).Name("multi-return")

		result, _ := targ.Execute([]string{"app", "multi-return"}, target)
		g.Expect(result.Output).To(ContainSubstring("must return only error"))
	})
}
