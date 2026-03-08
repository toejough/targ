//nolint:maintidx // Test functions with many subtests have low maintainability index by design
package targ_test

import (
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// TagOptionsWrongInputArgs has TagOptions with wrong input types.
type TagOptionsWrongInputArgs struct {
	Name string `targ:"flag"`
}

func (a *TagOptionsWrongInputArgs) TagOptions(
	_ int,
	opts targ.TagOptions,
) (targ.TagOptions, error) {
	return opts, nil
}

// TagOptionsWrongOutputArgs has TagOptions with wrong output types.
type TagOptionsWrongOutputArgs struct {
	Name string `targ:"flag"`
}

func (a *TagOptionsWrongOutputArgs) TagOptions(fieldName string, _ targ.TagOptions) string {
	return fieldName
}

// TagOptionsWrongParamCountArgs has TagOptions with wrong number of parameters.
type TagOptionsWrongParamCountArgs struct {
	Name string `targ:"flag"`
}

func (a *TagOptionsWrongParamCountArgs) TagOptions() targ.TagOptions {
	return targ.TagOptions{}
}

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

	t.Run("NilInTargetListLogsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Passing nil directly to Execute (not wrapped in targ.Targ)
		// logs an error and skips the target
		result, _ := targ.Execute([]string{"app"}, nil)
		g.Expect(result.Output).To(ContainSubstring("nil target"))
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

	t.Run("UnknownSubcommandInDefaultModeReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := targ.Targ(func() {}).Name("sub")
		grp := targ.Group("grp", sub)

		// In default mode with a single group, unknown subcommand should error
		result, err := targ.Execute(
			[]string{"app", "unknown"},
			grp,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Unknown"))
	})

	t.Run("DisableHelpIgnoresHelpFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		called := false
		target := targ.Targ(func() { called = true }).Name("cmd")

		// With DisableHelp, --help is passed to the command as a regular arg
		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--help"},
			targ.RunOptions{DisableHelp: true},
			target,
		)
		// The command runs (--help becomes unknown flag)
		g.Expect(err).To(HaveOccurred()) // Unknown flag error
		g.Expect(called).To(BeFalse())   // Not called because of unknown flag
	})

	t.Run("DefaultTargetErrorReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() error {
			return errors.New("deliberate default error")
		})

		// Single target in default mode, no subcommand needed
		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("deliberate default error"))
	})

	t.Run("GroupSubcommandErrorReportsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := targ.Targ(func() error {
			return errors.New("subcommand error")
		}).Name("failing")
		grp := targ.Group("grp", sub)

		// Execute subcommand that fails
		result, err := targ.Execute([]string{"app", "failing"}, grp)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("subcommand error"))
	})

	t.Run("TagOptionsWrongParamCountReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(_ TagOptionsWrongParamCountArgs) {}).Name("cmd")

		// Calling --help triggers tag parsing which calls TagOptions method
		_, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("TagOptionsWrongInputTypeReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(_ TagOptionsWrongInputArgs) {}).Name("cmd")

		_, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("TagOptionsWrongOutputTypeReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func(_ TagOptionsWrongOutputArgs) {}).Name("cmd")

		_, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("DuplicateShortFlagNameReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
			Version bool `targ:"flag,short=v"` // Same short flag
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("flag already defined"))
	})

	t.Run("HelpOutputWithGetwdErrorFallsBackToAbsolutePath", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cmd")

		// Inject a failing Getwd - help should still work with absolute paths
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--help"},
			targ.RunOptions{
				Getwd: func() (string, error) {
					return "", errors.New("getwd failed")
				},
			},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Help output should still be generated (with absolute path fallback)
		g.Expect(result.Output).To(ContainSubstring("cmd"))
	})

	t.Run("HelpOutputShowsPlaceholderTypes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name        string  `targ:"flag"`                    // <string> placeholder
			Count       int     `targ:"flag"`                    // <int> placeholder
			Verbose     bool    `targ:"flag"`                    // [flag] placeholder
			Path        string  `targ:"flag,placeholder=<path>"` // custom placeholder
			Mode        string  `targ:"flag,enum=fast|slow"`     // enum placeholder
			Temperature float64 `targ:"flag"`                    // default (no placeholder)
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "cmd", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Check that various placeholder types are shown
		g.Expect(result.Output).To(ContainSubstring("<string>"))
		g.Expect(result.Output).To(ContainSubstring("<int>"))
		g.Expect(result.Output).To(ContainSubstring("<path>"))
		g.Expect(result.Output).To(ContainSubstring("{fast|slow}"))
	})
}
