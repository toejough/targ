package targ_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestProperty_CommandHelp(t *testing.T) {
	t.Parallel()

	t.Run("HelpShowsFlagDescriptions", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate random description text
			desc := rapid.StringMatching(`[A-Za-z][a-z]{2,15}( [a-z]{2,10}){0,3}`).
				Draw(t, "description")

			// We can't dynamically create struct tags, so we test the property
			// that descriptions in struct tags appear in help output
			type Args struct {
				Flag string `targ:"flag,desc=Test flag description"`
			}

			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")
			target := targ.Targ(func(_ Args) {}).Name(cmdName).Description(desc)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(cmdName))
			g.Expect(result.Output).To(ContainSubstring("flag"))
			g.Expect(result.Output).To(ContainSubstring("Test flag description"))
		})
	})

	t.Run("HelpShowsUsageLine", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			cmdName := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "cmdName")

			type Args struct {
				File string `targ:"positional"`
			}

			target := targ.Targ(func(_ Args) {}).Name(cmdName)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(cmdName))
			g.Expect(result.Output).To(ContainSubstring("file"))
		})
	})

	t.Run("HelpShowsSubcommandList", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate unique subcommand names
			sub1Name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "sub1Name")
			sub2Name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "sub2Name")
			// Ensure they're different
			if sub1Name == sub2Name {
				sub2Name += "x"
			}

			sub1Desc := rapid.StringMatching(`[A-Z][a-z]{2,10}( [a-z]{2,8}){0,2}`).
				Draw(t, "sub1Desc")
			sub2Desc := rapid.StringMatching(`[A-Z][a-z]{2,10}( [a-z]{2,8}){0,2}`).
				Draw(t, "sub2Desc")
			groupName := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "groupName")

			sub1 := targ.Targ(func() {}).Name(sub1Name).Description(sub1Desc)
			sub2 := targ.Targ(func() {}).Name(sub2Name).Description(sub2Desc)
			root := targ.Group(groupName, sub1, sub2)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				root,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(sub1Name))
			g.Expect(result.Output).To(ContainSubstring(sub1Desc))
			g.Expect(result.Output).To(ContainSubstring(sub2Name))
			g.Expect(result.Output).To(ContainSubstring(sub2Desc))
		})
	})

	t.Run("HelpShowsEnumValues", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// We can't create dynamic struct tags, so we test that static enum values
			// appear in help with a randomly generated command name
			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")

			type Args struct {
				Level string `targ:"flag,enum=debug|info|warn|error"`
			}

			target := targ.Targ(func(_ Args) {}).Name(cmdName)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring("debug"))
			g.Expect(result.Output).To(ContainSubstring("info"))
			g.Expect(result.Output).To(ContainSubstring("warn"))
			g.Expect(result.Output).To(ContainSubstring("error"))
		})
	})

	t.Run("HelpShowsCachePatterns", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate glob-like patterns
			ext := rapid.StringMatching(`[a-z]{1,4}`).Draw(t, "extension")
			pattern := "**/*." + ext

			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")
			target := targ.Targ(func() {}).
				Name(cmdName).
				Cache(pattern)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring("Cache"))
			g.Expect(result.Output).To(ContainSubstring(pattern))
		})
	})

	t.Run("HelpShowsWatchPatterns", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate glob-like patterns
			dir := rapid.StringMatching(`[a-z]{2,6}`).Draw(t, "dir")
			ext := rapid.StringMatching(`[a-z]{1,4}`).Draw(t, "extension")
			pattern := fmt.Sprintf("%s/*.%s", dir, ext)

			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")
			target := targ.Targ(func() {}).
				Name(cmdName).
				Watch(pattern)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring("Watch"))
			g.Expect(result.Output).To(ContainSubstring(pattern))
		})
	})

	t.Run("HelpDisplaysWithManyPositionals", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			cmdName := rapid.StringMatching(`[a-z]{3,12}`).Draw(t, "cmdName")

			// Use a fixed struct with many positionals to test wrapping
			type Args struct {
				InputFile      string `targ:"positional"`
				OutputFile     string `targ:"positional"`
				ConfigFile     string `targ:"positional"`
				TemplateFile   string `targ:"positional"`
				LoggingFile    string `targ:"positional"`
				MetadataFile   string `targ:"positional"`
				AdditionalFile string `targ:"positional"`
			}

			target := targ.Targ(func(_ Args) {}).Name(cmdName)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(cmdName))
			g.Expect(result.Output).To(ContainSubstring("InputFile"))
		})
	})
}

func TestProperty_ShellCommandDeps(t *testing.T) {
	t.Parallel()

	t.Run("DepsRunBeforeShellCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false

		dep := targ.Targ(func() {
			depRan = true
		}).Name("dep")

		var executedCmd string

		mockRunner := func(_ context.Context, cmd string) error {
			executedCmd = cmd
			return nil
		}

		main := targ.Targ("echo hello").Name("main").Deps(dep)

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "main"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
			main, dep,
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(depRan).To(BeTrue())
		g.Expect(executedCmd).To(Equal("echo hello"))
	})

	t.Run("DepErrorPreventsShellCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep := targ.Targ(func() error {
			return errors.New("dep failed")
		}).Name("dep")

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true
			return nil
		}

		main := targ.Targ("echo hello").Name("main").Deps(dep)

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "main"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
			main, dep,
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
	})

	// Issue #42: a bare unknown arg must be rejected before the dep chain and
	// the shell command run. executeShellCommand received `explicit` and threw
	// it away, so the whole invocation happened and only then errored.
	t.Run("UnknownBareArgRunsNeitherDepNorShellCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false
		dep := targ.Targ(func() { depRan = true }).Name("dep")

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true

			return nil
		}

		main := targ.Targ("echo hello").Name("main").Deps(dep)

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "bogus"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
			main,
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
		g.Expect(depRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("unknown command: bogus"))
	})

	// Issue #42: the shell path's ResolveOnly return sits BELOW the explicit
	// check, not beside the HelpOnly one, so resolve mode still validates.
	// Merging it into the HelpOnly return at command.go:1036 would hand the
	// bad arg back as a chaining candidate and the dispatcher would misreport
	// it as an unknown COMMAND. This test is what catches that.
	t.Run("ResolveOnlyStillRejectsUnknownBareArg", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true

			return nil
		}

		main := targ.Targ("echo hello").Name("main")

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "bogus"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true, ResolveOnly: true},
			main,
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("unknown command: bogus"))
		g.Expect(result.Output).NotTo(ContainSubstring("Unknown command: bogus"))
	})

	// Issue #42: the positive half of the same seam - when the args DO
	// resolve, resolve mode runs neither the shell command nor its deps.
	t.Run("ResolveOnlyRunsNeitherShellCommandNorDeps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false
		dep := targ.Targ(func() { depRan = true }).Name("dep")

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true

			return nil
		}

		main := targ.Targ("echo hello").Name("main").Deps(dep)

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "main"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true, ResolveOnly: true},
			main,
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
		g.Expect(depRan).To(BeFalse())
	})

	// I1 (final review): with DisableHelp true, extractHelpFlag never strips
	// --help, so the token survives all the way to executeShellCommand's own
	// parsed.helpRequested branch - which sits above BOTH the HelpOnly and
	// ResolveOnly guards. The serial two-pass walk calls it once per pass
	// unless the print itself is gated on !ResolveOnly.
	t.Run("ShellHelpPrintsOnceWithDisableHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mockRunner := func(_ context.Context, _ string) error {
			return nil
		}

		main := targ.Targ("echo hello").Name("main")

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "main", "--help"},
			targ.RunOptions{DisableHelp: true, ShellRunner: mockRunner, AllowDefault: true},
			main,
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(strings.Count(result.Output, "Usage:")).To(Equal(1))
	})
}

func TestProperty_ShellCommandErrors(t *testing.T) {
	t.Parallel()

	t.Run("MissingVarReturnsError", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			varName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "varName")
			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")

			shellCmd := "echo $" + varName
			target := targ.Targ(shellCmd).Name(cmdName)

			result, err := targ.Execute(
				[]string{"app"},
				target,
			)

			g.Expect(err).To(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(varName))
		})
	})

	t.Run("UnknownLongFlagReturnsError", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			knownVar := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "knownVar")

			unknownFlag := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "unknownFlag")
			if knownVar == unknownFlag {
				unknownFlag += "x"
			}

			value := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			mockRunner := func(_ context.Context, _ string) error {
				t.Fatal("shell runner should not be called when unknown flag present")
				return nil
			}

			shellCmd := "mycommand $" + knownVar
			target := targ.Targ(shellCmd).Name("shell-cmd")

			result, err := targ.ExecuteWithOptions(
				[]string{"app", "--" + knownVar, value, "--" + unknownFlag, "badvalue"},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).To(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(unknownFlag))
		})
	})

	t.Run("UnknownShortFlagReturnsError", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			// Generate a known var starting with 'a' so we can use 'z' as unknown
			knownVar := "a" + rapid.StringMatching(`[a-z]{2,7}`).Draw(t, "knownVarSuffix")
			unknownLetter := "z"
			value := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			mockRunner := func(_ context.Context, _ string) error {
				t.Fatal("shell runner should not be called when unknown short flag present")
				return nil
			}

			shellCmd := "mycommand $" + knownVar
			target := targ.Targ(shellCmd).Name("shell-cmd")

			result, err := targ.ExecuteWithOptions(
				[]string{"app", "--" + knownVar, value, "-" + unknownLetter, "badvalue"},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).To(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring("-" + unknownLetter))
		})
	})
}

func TestProperty_ShellCommandExecution(t *testing.T) {
	t.Parallel()

	t.Run("VariablesSubstituted", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			var executedCmd string

			mockRunner := func(_ context.Context, cmd string) error {
				executedCmd = cmd
				return nil
			}

			target := targ.Targ("mycommand $msg").Name("shell-test")

			_, err := targ.ExecuteWithOptions(
				[]string{"app", "--msg", value},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(executedCmd).To(Equal("mycommand " + value))
		})
	})

	t.Run("FailureReturnsError", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			errMsg := rapid.StringMatching(`[a-z]{3,10}( [a-z]{2,8}){0,2}`).Draw(t, "errMsg")
			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")

			mockRunner := func(_ context.Context, _ string) error {
				return errors.New(errMsg)
			}

			target := targ.Targ("mycommand").Name(cmdName)

			result, err := targ.ExecuteWithOptions(
				[]string{"app"},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).To(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring("failed"))
		})
	})
}

func TestProperty_ShellCommandFlags(t *testing.T) {
	t.Parallel()

	t.Run("LongFlags", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			val1 := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "val1")
			val2 := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "val2")

			var executedCmd string

			mockRunner := func(_ context.Context, cmd string) error {
				executedCmd = cmd
				return nil
			}

			target := targ.Targ("mycommand $greeting $name").Name("greet")

			_, err := targ.ExecuteWithOptions(
				[]string{"app", "--greeting", val1, "--name", val2},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(executedCmd).To(Equal(fmt.Sprintf("mycommand %s %s", val1, val2)))
		})
	})

	t.Run("EqualsFlags", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			value := rapid.StringMatching(`[a-zA-Z0-9_-]{1,15}`).Draw(t, "value")

			var executedCmd string

			mockRunner := func(_ context.Context, cmd string) error {
				executedCmd = cmd
				return nil
			}

			target := targ.Targ("mycommand $msg").Name("echo-equals")

			_, err := targ.ExecuteWithOptions(
				[]string{"app", "--msg=" + value},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(executedCmd).To(Equal("mycommand " + value))
		})
	})

	t.Run("ShortFlags", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			value := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			var executedCmd string

			mockRunner := func(_ context.Context, cmd string) error {
				executedCmd = cmd
				return nil
			}

			target := targ.Targ("mycommand $msg").Name("echo-short")

			_, err := targ.ExecuteWithOptions(
				[]string{"app", "-m", value},
				targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(executedCmd).To(Equal("mycommand " + value))
		})
	})
}

func TestProperty_ShellCommandHelp(t *testing.T) {
	t.Parallel()

	t.Run("ShowsVariables", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			var1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "var1")

			var2 := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "var2")
			if var1 == var2 {
				var2 += "x"
			}

			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")
			desc := rapid.StringMatching(`[A-Z][a-z]{2,10}( [a-z]{2,8}){0,2}`).Draw(t, "desc")

			shellCmd := fmt.Sprintf("echo $%s $%s", var1, var2)
			target := targ.Targ(shellCmd).Name(cmdName).Description(desc)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(var1))
			g.Expect(result.Output).To(ContainSubstring(var2))
		})
	})

	t.Run("IncludesDescription", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			desc := rapid.StringMatching(`[A-Z][a-z]{2,12}( [a-z]{2,10}){1,3}`).Draw(t, "desc")
			cmdName := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdName")

			target := targ.Targ("echo test").Name(cmdName).Description(desc)

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(desc))
		})
	})
}

func TestProperty_ShellCommandNaming(t *testing.T) {
	t.Parallel()

	t.Run("DerivesNameFromCommand", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)

			cmdWord := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "cmdWord")
			arg := rapid.StringMatching(`[a-z]{2,8}`).Draw(t, "arg")

			shellCmd := fmt.Sprintf("%s %s", cmdWord, arg)
			target := targ.Targ(shellCmd)
			other := targ.Targ(func() {}).Name("other")

			result, err := targ.Execute(
				[]string{"app", "--help"},
				target, other,
			)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.Output).To(ContainSubstring(cmdWord))
		})
	})
}
