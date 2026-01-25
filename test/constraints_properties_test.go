package targ_test

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Property: Failure has clear error message
func TestProperty_Invariant_FailureHasClearErrorMessage(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		errMsg := rapid.StringMatching(`[a-zA-Z ]{10,30}`).Draw(rt, "errMsg")

		target := targ.Targ(func() error {
			return errors.New(errMsg)
		})

		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())

		// Error message should be present in output or error
		hasMsg := strings.Contains(result.Output, errMsg) ||
			strings.Contains(err.Error(), errMsg)
		g.Expect(hasMsg).To(BeTrue())
	})
}

// Property: Unknown command has clear error
func TestProperty_Invariant_UnknownCommandHasClearError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {}).Name("known")

	result, err := targ.ExecuteWithOptions(
		[]string{"app", "unknown"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(result.Output).To(ContainSubstring("Unknown"))
}

// Property: Invalid flag format has clear error
func TestProperty_Invariant_InvalidFlagFormatHasClearError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Labels map[string]string `targ:"flag"`
	}

	target := targ.Targ(func(_ Args) {})

	_, err := targ.Execute([]string{"app", "--labels", "invalid-no-equals"}, target)
	g.Expect(err).To(HaveOccurred())
}

// Property: Help flag does not execute target
func TestProperty_Invariant_HelpFlagDoesNotExecuteTarget(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	target := targ.Targ(func() { called = true })

	_, err := targ.Execute([]string{"app", "--help"}, target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeFalse())
}

// Property: Nil target panics
func TestProperty_Invariant_NilTargetPanics(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(nil)
	}).To(Panic())
}

// Property: Empty string target panics
func TestProperty_Invariant_EmptyStringTargetPanics(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ("")
	}).To(Panic())
}

// Property: Invalid target type panics
func TestProperty_Invariant_InvalidTargetTypePanics(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(42) // int is not func or string
	}).To(Panic())
}

// Property: Duplicate names produce error
func TestProperty_Invariant_DuplicateNamesProduceError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	a := targ.Targ(func() {}).Name("dup")
	b := targ.Targ(func() {}).Name("dup")

	_, err := targ.Execute([]string{"app", "dup"}, a, b)
	g.Expect(err).To(HaveOccurred())
}

// Property: AllowDefault=false requires explicit command
func TestProperty_Invariant_AllowDefaultFalseRequiresExplicitCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	called := false
	target := targ.Targ(func() { called = true })

	_, err := targ.ExecuteWithOptions(
		[]string{"app"},
		targ.RunOptions{AllowDefault: false},
		target,
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeFalse()) // Should show usage, not execute
}

// Property: Invalid duration flag value has clear error
func TestProperty_Invariant_InvalidDurationFlagHasClearError(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {})

	_, err := targ.Execute([]string{"app", "--timeout", "not-a-duration"}, target)
	g.Expect(err).To(HaveOccurred())
}
