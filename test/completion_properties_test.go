package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestProperty_Completion(t *testing.T) {
	t.Parallel()

	t.Run("ScriptGenerationForBash", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		result, err := targ.Execute([]string{"app", "--completion", "bash"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Bash completion uses "complete" builtin
		g.Expect(result.Output).To(ContainSubstring("complete"))
		g.Expect(result.Output).To(ContainSubstring("__complete"))
		g.Expect(result.Output).NotTo(ContainSubstring("MISSING"))
	})

	t.Run("ScriptGenerationForZsh", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		result, err := targ.Execute([]string{"app", "--completion", "zsh"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Zsh completion uses "compdef"
		g.Expect(result.Output).To(ContainSubstring("compdef"))
		g.Expect(result.Output).To(ContainSubstring("__complete"))
		g.Expect(result.Output).NotTo(ContainSubstring("MISSING"))
	})

	t.Run("ScriptGenerationForFish", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		result, err := targ.Execute([]string{"app", "--completion", "fish"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Fish completion uses "complete -c"
		g.Expect(result.Output).To(ContainSubstring("complete"))
		g.Expect(result.Output).To(ContainSubstring("__complete"))
		g.Expect(result.Output).NotTo(ContainSubstring("MISSING"))
	})

	t.Run("UnsupportedShellReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		_, err := targ.Execute([]string{"app", "--completion", "powershell"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("HelpShowsCompletionExample", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("completion"))
	})

	t.Run("CompletionDoesNotExecuteTarget", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		executed := false
		target := targ.Targ(func() { executed = true }).Name("build")

		_, err := targ.Execute([]string{"app", "--completion", "bash"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(executed).To(BeFalse())
	})

	t.Run("DisabledCompletionRejectsFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "--completion", "bash"},
			targ.RunOptions{DisableCompletion: true},
			target,
		)
		g.Expect(err).To(HaveOccurred())
	})
}

// TestProperty_CompletionSuggestions tests the __complete command behavior.
// Note: __complete is an internal command used by shell completion scripts.
// The format is: app __complete "full command line"
func TestProperty_CompletionSuggestions(t *testing.T) {
	t.Parallel()

	t.Run("SuggestsSubcommandsAtGroupLevel", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")
		dev := targ.Group("dev", build, test)

		// __complete takes the full command line as a single string
		result, err := targ.Execute([]string{"app", "__complete", "app dev "}, dev)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("SuggestsRootsAtTopLevel", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		result, err := targ.Execute([]string{"app", "__complete", "app "}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("test"))
	})

	t.Run("SuggestsFlagsWithDashPrefix", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
		}

		target := targ.Targ(func(_ Args) {}).Name("build")

		result, err := targ.Execute([]string{"app", "__complete", "app --"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--verbose"))
		g.Expect(result.Output).To(ContainSubstring("--help"))
	})

	t.Run("SuggestsEnumValuesForEnumFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Mode string `targ:"flag,enum=dev|prod|staging"`
		}

		target := targ.Targ(func(_ Args) {}).Name("deploy")

		// After --mode, suggest enum values
		result, err := targ.Execute([]string{"app", "__complete", "app --mode "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("dev"))
		g.Expect(result.Output).To(ContainSubstring("prod"))
		g.Expect(result.Output).To(ContainSubstring("staging"))
	})

	t.Run("SuggestsPositionalEnumValues", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Action string `targ:"positional,enum=start|stop|restart"`
		}

		target := targ.Targ(func(_ Args) {}).Name("service")

		result, err := targ.Execute([]string{"app", "__complete", "app "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("start"))
		g.Expect(result.Output).To(ContainSubstring("stop"))
		g.Expect(result.Output).To(ContainSubstring("restart"))
	})

	t.Run("SuggestsCaretForPathReset", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub := targ.Targ(func() {}).Name("sub")
		grp := targ.Group("grp", sub)
		other := targ.Targ(func() {}).Name("other")

		// After completing a subcommand in multi-root mode, ^ should be suggested
		result, err := targ.Execute([]string{"app", "__complete", "app grp sub "}, grp, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("^"))
	})

	t.Run("ChainsAfterCompletedCommands", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// After completing build, should still suggest other roots for chaining
		result, err := targ.Execute([]string{"app", "__complete", "app build "}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		// Should suggest test as it can be chained, and/or build again
		g.Expect(result.Output).To(SatisfyAny(
			ContainSubstring("test"),
			ContainSubstring("build"),
		))
	})
}
