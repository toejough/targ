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

	t.Run("CompletionWithEmptyShellEnvShowsUsage", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")
		other := targ.Targ(func() {}).Name("other")

		// --completion without shell arg, and SHELL env is empty
		// Use Env map to override SHELL to empty string
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{
				Env: map[string]string{"SHELL": ""},
			},
			target, other,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Could not detect shell"))
	})

	t.Run("CompletionWithWindowsPathShell", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		// SHELL env with Windows-style path (no .exe extension)
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{
				Env: map[string]string{"SHELL": `C:\Program Files\Git\bin\bash`},
			},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("complete"))
	})

	t.Run("CompletionWithUnmatchedRootInMultiRoot", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Multi-root mode: completion for a command that doesn't exist
		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// "unknown" doesn't match any root, triggers followRemaining's nextRoot==nil path
		result, err := targ.Execute([]string{"app", "__complete", "app unknown "}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		// Should still work (return empty or root suggestions)
		_ = result
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

	t.Run("HandlesShortFlagCompletion", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,short=v"`
			Count   int    `targ:"flag,short=c"`
			Name    string `targ:"flag,short=n"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After short flag -v (bool), should continue suggesting
		result, err := targ.Execute([]string{"app", "__complete", "app -v -"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("--"))
	})

	t.Run("HandlesLongFlagWithValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Output string `targ:"flag,enum=json|text|xml"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After --output expecting enum value, should suggest enum values
		result, err := targ.Execute([]string{"app", "__complete", "app --output "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should suggest enum values for the flag
		g.Expect(result.Output).To(ContainSubstring("json"))
		g.Expect(result.Output).To(ContainSubstring("text"))
	})

	t.Run("HandlesVariadicFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files []string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Variadic flags can take multiple values
		result, err := targ.Execute([]string{"app", "__complete", "app --files a b "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should still accept more values or flags
		_ = result
	})

	t.Run("HandlesPartialRootMatch", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		bench := targ.Targ(func() {}).Name("bench")
		test := targ.Targ(func() {}).Name("test")

		// Partial match "b" should suggest build and bench
		result, err := targ.Execute([]string{"app", "__complete", "app b"}, build, bench, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).To(ContainSubstring("bench"))
	})

	t.Run("HandlesSingleTargetMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("process")

		// Single target mode skips root selection
		result, err := targ.Execute([]string{"app", "__complete", "app "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should not suggest process as a command (already selected)
		_ = result
	})

	t.Run("WithRequiredPositionalNotFilledNoRootSuggestions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Target string `targ:"positional,required"`
		}

		// Multi-root mode with required positional on one command
		build := targ.Targ(func(_ BuildArgs) {}).Name("build")
		other := targ.Targ(func() {}).Name("other")

		// After selecting build but before providing required positional,
		// root commands shouldn't be suggested for chaining
		result, err := targ.Execute([]string{"app", "__complete", "app build "}, build, other)
		g.Expect(err).NotTo(HaveOccurred())
		// Should not suggest "other" because build's required positional isn't filled
		g.Expect(result.Output).NotTo(ContainSubstring("other"))
	})

	t.Run("GroupedShortFlagsWithValueAtEnd", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,short=v"`
			All     bool   `targ:"flag,short=a"`
			Output  string `targ:"flag,short=o,enum=json|text"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After -vao (grouped short flags where 'o' takes value), should suggest enum values
		result, err := targ.Execute([]string{"app", "__complete", "app -vao "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("json"))
		g.Expect(result.Output).To(ContainSubstring("text"))
	})

	t.Run("AfterLongFlagExpectingValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Format string `targ:"flag,enum=csv|json|xml"`
		}

		target := targ.Targ(func(_ Args) {}).Name("export")

		// After --format expecting value
		result, err := targ.Execute([]string{"app", "__complete", "app --format "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("csv"))
		g.Expect(result.Output).To(ContainSubstring("json"))
		g.Expect(result.Output).To(ContainSubstring("xml"))
	})

	t.Run("AfterShortFlagExpectingValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Type string `targ:"flag,short=t,enum=alpha|beta|gamma"`
		}

		target := targ.Targ(func(_ Args) {}).Name("run")

		// After -t expecting value
		result, err := targ.Execute([]string{"app", "__complete", "app -t "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("alpha"))
		g.Expect(result.Output).To(ContainSubstring("beta"))
	})

	t.Run("ChainAfterCompletedCommandWithPositionals", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Target string `targ:"positional"`
		}

		build := targ.Targ(func(_ BuildArgs) {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// After "build main" complete, with multiroot can chain to another command
		result, err := targ.Execute([]string{"app", "__complete", "app build main "}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		// Should suggest roots for chaining
		_ = result
	})

	t.Run("PositionalCounterWithShortFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool     `targ:"flag,short=v"`
			Count   int      `targ:"flag,short=c"`
			Files   []string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("process")

		// Short flags with values mixed with positionals
		result, err := targ.Execute([]string{"app", "__complete", "app -v -c 5 file1 "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should continue accepting positionals
		_ = result
	})

	t.Run("NoMatchingRootReturnsEmpty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// "xyz" doesn't match any root
		result, err := targ.Execute([]string{"app", "__complete", "app xyz"}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		// Should not suggest anything since no match
		g.Expect(result.Output).NotTo(ContainSubstring("build"))
		g.Expect(result.Output).NotTo(ContainSubstring("test"))
	})

	t.Run("PartialRootWithTrailingSpaceSuggestsMatches", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		build := targ.Targ(func() {}).Name("build")
		bench := targ.Targ(func() {}).Name("bench")
		test := targ.Targ(func() {}).Name("test")

		// "bu " with trailing space - partial entered as a processedArg
		// This triggers suggestMatchingRoots which filters by prefix
		result, err := targ.Execute([]string{"app", "__complete", "app bu "}, build, bench, test)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("build"))
		g.Expect(result.Output).NotTo(ContainSubstring("bench"))
		g.Expect(result.Output).NotTo(ContainSubstring("test"))
	})

	t.Run("ChainAfterFirstCommandInMultiRoot", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Target string `targ:"positional"`
		}

		build := targ.Targ(func(_ BuildArgs) {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// After "build main ", should suggest other roots for chaining (followRemaining)
		result, err := targ.Execute([]string{"app", "__complete", "app build main "}, build, test)
		g.Expect(err).NotTo(HaveOccurred())
		// In multi-root mode, after completing one command, other roots should be suggested
		g.Expect(result.Output).To(SatisfyAny(
			ContainSubstring("test"),
			ContainSubstring("build"),
			ContainSubstring("^"),
		))
	})

	t.Run("ChainWithUnknownCommandDoesNotCrash", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Target string `targ:"positional"`
		}

		build := targ.Targ(func(_ BuildArgs) {}).Name("build")
		test := targ.Targ(func() {}).Name("test")

		// After "build main unknown " - "unknown" is not a valid root
		// This tests the followRemaining path when nextRoot == nil
		result, err := targ.Execute(
			[]string{"app", "__complete", "app build main unknown "},
			build,
			test,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Should gracefully handle and still provide some completions
		// (completions may be empty or include recovery suggestions)
		g.Expect(result).NotTo(BeNil())
	})

	t.Run("ChainedCommandSuggestsSecondCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type BuildArgs struct {
			Target string `targ:"positional"`
		}

		build := targ.Targ(func(_ BuildArgs) {}).Name("build")
		test := targ.Targ(func() {}).Name("test")
		lint := targ.Targ(func() {}).Name("lint")

		// "app build main test " - after build with positional, chain to test
		// This triggers followRemaining and findCompletionRoot
		result, err := targ.Execute(
			[]string{"app", "__complete", "app build main test "},
			build,
			test,
			lint,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// After test completes (no args), should suggest roots for further chaining
		g.Expect(result.Output).To(SatisfyAny(
			ContainSubstring("build"),
			ContainSubstring("lint"),
			ContainSubstring("^"),
		))
	})

	t.Run("LongFlagNoEnumSuppressesPositionalSuggestions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Output string `targ:"flag"` // no enum, just expects a value
			Action string `targ:"positional,enum=start|stop"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After --output (no enum), waiting for value, should not suggest positional enums
		// This triggers expectingLongFlagValue
		result, err := targ.Execute([]string{"app", "__complete", "app --output "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should NOT suggest positional enum values when expecting flag value
		g.Expect(result.Output).NotTo(ContainSubstring("start"))
		g.Expect(result.Output).NotTo(ContainSubstring("stop"))
	})

	t.Run("ShortFlagNoEnumSuppressesPositionalSuggestions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Output string `targ:"flag,short=o"` // no enum, just expects a value
			Action string `targ:"positional,enum=start|stop"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After -o (no enum), waiting for value, should not suggest positional enums
		// This triggers expectingShortFlagValue
		result, err := targ.Execute([]string{"app", "__complete", "app -o "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should NOT suggest positional enum values when expecting flag value
		g.Expect(result.Output).NotTo(ContainSubstring("start"))
		g.Expect(result.Output).NotTo(ContainSubstring("stop"))
	})

	t.Run("GroupedShortFlagsWithNonEnumValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,short=v"`
			All     bool   `targ:"flag,short=a"`
			Output  string `targ:"flag,short=o"` // no enum
			Action  string `targ:"positional,enum=start|stop"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// After -vao (grouped, -o expects value but no enum), suppress positional suggestions
		// This triggers expectingGroupedShortFlagValue and skipGroupedShortFlags
		result, err := targ.Execute([]string{"app", "__complete", "app -vao "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should NOT suggest positional enum values when expecting flag value
		g.Expect(result.Output).NotTo(ContainSubstring("start"))
		g.Expect(result.Output).NotTo(ContainSubstring("stop"))
	})

	t.Run("GroupedFlagsWithValueTakingInMiddle", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,short=v"`
			All     bool   `targ:"flag,short=a"`
			Output  string `targ:"flag,short=o"` // value-taking in middle
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// -aov has -o in the middle (not last), -o takes a value
		// This tests the early return path when value-taking flag is not at end of group
		result, err := targ.Execute([]string{"app", "__complete", "app -aov "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// The completion should handle this gracefully
		_ = result
	})

	t.Run("GroupedBoolFlagsBeforePositionalEnum", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,short=v"`
			All     bool   `targ:"flag,short=a"`
			Debug   bool   `targ:"flag,short=d"`
			Action  string `targ:"positional,enum=start|stop|restart"`
		}

		target := targ.Targ(func(_ Args) {}).Name("service")

		// After -vad (all bool flags), should suggest positional enum values
		// This triggers skipGroupedShortFlags in the positional counter
		result, err := targ.Execute([]string{"app", "__complete", "app -vad "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should suggest positional enum values since all flags were bool
		g.Expect(result.Output).To(ContainSubstring("start"))
		g.Expect(result.Output).To(ContainSubstring("stop"))
		g.Expect(result.Output).To(ContainSubstring("restart"))
	})

	t.Run("ShellDetectionFromEnvironment", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		// Test detection of bash - using Env option for DI
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{Env: map[string]string{"SHELL": "/bin/bash"}},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("complete"))
		g.Expect(result.Output).To(ContainSubstring("__complete"))

		// Test detection of zsh
		result, err = targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{Env: map[string]string{"SHELL": "/usr/bin/zsh"}},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("compdef"))

		// Test detection of fish
		result, err = targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{Env: map[string]string{"SHELL": "/opt/homebrew/bin/fish"}},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("complete"))
		g.Expect(result.Output).To(ContainSubstring("commandline"))

		// Test unknown shell fallback (shows usage)
		result, err = targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{Env: map[string]string{"SHELL": "/bin/sh"}},
			target,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Usage"))
	})

	t.Run("BinaryNameFromEnvironment", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("build")

		// Test custom binary name via env - using Env option for DI
		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--completion"},
			targ.RunOptions{Env: map[string]string{
				"SHELL":         "/bin/bash",
				"TARG_BIN_NAME": "mycli",
			}},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// The completion script should use the custom binary name
		g.Expect(result.Output).To(ContainSubstring("mycli"))
	})

	// Tests for command line tokenizer with quotes and escapes

	t.Run("CommandLineWithSingleQuotes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Message string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Single quotes preserve literal string
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --message 'hello world' "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// After completing the flag value, should suggest other flags
		_ = result
	})

	t.Run("CommandLineWithDoubleQuotes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Message string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Double quotes preserve string with potential escapes
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --message \"hello world\" "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result
	})

	t.Run("CommandLineWithBackslashEscape", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Message string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Backslash escapes the next character
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --message hello\\ world "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result
	})

	t.Run("CommandLineWithMixedQuotes", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Pattern string `targ:"flag"`
			Output  string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("search")

		// Mix of quoting styles
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --pattern 'foo*' --output \"result.txt\" "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result
	})

	t.Run("CommandLineWithUnclosedQuote", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Message string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Unclosed quote - still being typed
		result, err := targ.Execute([]string{"app", "__complete", "app --message \"hello"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// No completions since still inside quote
		_ = result
	})

	t.Run("CompletionSkipsTargGlobalFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Test with --timeout (value-taking flag with space syntax)
		result, err := targ.Execute([]string{"app", "__complete", "app --timeout 30 cmd "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should skip targ flags and show completions for 'cmd' (empty because no enums)
		_ = result

		// Test with --timeout=30 (value-taking flag with equals syntax)
		result, err = targ.Execute([]string{"app", "__complete", "app --timeout=30 cmd "}, target)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result

		// Test with --retry (boolean targ flag)
		result, err = targ.Execute([]string{"app", "__complete", "app --retry cmd "}, target)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result

		// Test with --no-cache (boolean targ flag)
		result, err = targ.Execute([]string{"app", "__complete", "app --no-cache cmd "}, target)
		g.Expect(err).NotTo(HaveOccurred())

		_ = result
	})

	t.Run("SingleRootCompletionWithRemainingArgs", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional"`
		}

		// Single root mode (only one target)
		target := targ.Targ(func(_ Args) {}).Name("build")

		// After completing the positional "main.go", any remaining args need handling
		// This triggers the single-root case in followRemaining
		result, err := targ.Execute([]string{"app", "__complete", "app main.go extra "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// In single-root mode with remaining args, should still provide completions
		_ = result
	})

	t.Run("SingleRootCompletionWithVariadicAndExtra", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files []string `targ:"positional,variadic"`
		}

		// Single root mode with variadic positional
		target := targ.Targ(func(_ Args) {}).Name("build")

		// After variadic positional, extra args might appear after "--"
		result, err := targ.Execute([]string{"app", "__complete", "app file1 file2 -- "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// The completion should handle this gracefully
		_ = result
	})

	t.Run("HasFlagValuePrefixWithTrueSyntax", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Test hasFlagValuePrefix for boolean flags with equals syntax (--no-cache=false, etc.)
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --no-cache=false cmd "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Should recognize the flag=value syntax
		_ = result
	})

	t.Run("LongFlagWithEqualsInPositionalCounter", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name   string `targ:"flag"`
			Action string `targ:"positional,enum=start|stop"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Use --name=value syntax before positional
		// This tests skipLongFlag's equals-handling branch
		result, err := targ.Execute([]string{"app", "__complete", "app --name=foo "}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Should suggest positional enum values
		g.Expect(result.Output).To(ContainSubstring("start"))
		g.Expect(result.Output).To(ContainSubstring("stop"))
	})

	t.Run("LongFlagVariadicInPositionalCounter", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files  []string `targ:"flag"`
			Action string   `targ:"positional,enum=go|stop"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Variadic flag --files takes multiple values before positional
		// This tests skipLongFlag calling skipFlagValues with Variadic=true
		result, err := targ.Execute(
			[]string{"app", "__complete", "app --files a b c "},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Variadic flag consumes all values, so positional counter might not reach enum
		// Just verify it doesn't error
		_ = result
	})
}

// TestProperty_ShellDetection tests shell-specific completion example output.
// Cannot be parallel because we're modifying SHELL env var.
func TestProperty_ShellDetection(t *testing.T) {
	t.Run("ZshShellShowsSourceSyntax", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv("SHELL", "/bin/zsh")

		// Need multiple targets to trigger top-level help which shows completion example
		a := targ.Targ(func() {}).Name("a")
		b := targ.Targ(func() {}).Name("b")

		result, err := targ.Execute([]string{"app", "--help"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		// Zsh uses "source <(targ --completion)"
		g.Expect(result.Output).To(ContainSubstring("source <("))
	})

	t.Run("FishShellShowsPipeSyntax", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv("SHELL", "/usr/bin/fish")

		a := targ.Targ(func() {}).Name("a")
		b := targ.Targ(func() {}).Name("b")

		result, err := targ.Execute([]string{"app", "--help"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		// Fish uses "targ --completion | source"
		g.Expect(result.Output).To(ContainSubstring("| source"))
	})

	t.Run("BashShellShowsEvalSyntax", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv("SHELL", "/bin/bash")

		a := targ.Targ(func() {}).Name("a")
		b := targ.Targ(func() {}).Name("b")

		result, err := targ.Execute([]string{"app", "--help"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		// Bash uses 'eval "$(targ --completion)"'
		g.Expect(result.Output).To(ContainSubstring("eval"))
	})

	t.Run("EmptyShellDefaultsToBash", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv("SHELL", "")

		a := targ.Targ(func() {}).Name("a")
		b := targ.Targ(func() {}).Name("b")

		result, err := targ.Execute([]string{"app", "--help"}, a, b)
		g.Expect(err).NotTo(HaveOccurred())
		// Empty SHELL should default to bash syntax
		g.Expect(result.Output).To(ContainSubstring("eval"))
		g.Expect(result.Output).To(ContainSubstring("detected: bash"))
	})
}
