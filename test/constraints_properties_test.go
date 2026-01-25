package targ_test

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

//nolint:funlen // subtest container
func TestProperty_Invariant(t *testing.T) {
	t.Parallel()

	t.Run("AllowDefaultFalseRequiresExplicitCommand", func(t *testing.T) {
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
		g.Expect(called).To(BeFalse())
	})

	t.Run("DuplicateNamesProduceError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := targ.Targ(func() {}).Name("dup")
		b := targ.Targ(func() {}).Name("dup")

		_, err := targ.Execute([]string{"app", "dup"}, a, b)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("EmptyStringTargetPanics", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		g.Expect(func() { targ.Targ("") }).To(Panic())
	})

	t.Run("FailureHasClearErrorMessage", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			errMsg := rapid.StringMatching(`[a-zA-Z ]{10,30}`).Draw(t, "errMsg")

			target := targ.Targ(func() error { return errors.New(errMsg) })

			result, err := targ.Execute([]string{"app"}, target)
			g.Expect(err).To(HaveOccurred())

			hasMsg := strings.Contains(result.Output, errMsg) ||
				strings.Contains(err.Error(), errMsg)
			g.Expect(hasMsg).To(BeTrue())
		})
	})

	t.Run("HelpFlagDoesNotExecuteTarget", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		called := false
		target := targ.Targ(func() { called = true })

		_, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(called).To(BeFalse())
	})

	t.Run("InvalidDurationFlagHasClearError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {})
		_, err := targ.Execute([]string{"app", "--timeout", "not-a-duration"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("InvalidFlagFormatHasClearError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Labels map[string]string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})
		_, err := targ.Execute([]string{"app", "--labels", "invalid-no-equals"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("InvalidTargetTypePanics", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		g.Expect(func() { targ.Targ(42) }).To(Panic())
	})

	t.Run("NilTargetPanics", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		g.Expect(func() { targ.Targ(nil) }).To(Panic())
	})

	t.Run("UnknownCommandHasClearError", func(t *testing.T) {
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
	})
}
