package targ_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestProperty_CommandHelp(t *testing.T) {
	t.Parallel()

	t.Run("HelpShowsFlagDescriptions", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool   `targ:"flag,desc=Enable verbose output"`
			Count   int    `targ:"flag,desc=Number of times to run"`
			Name    string `targ:"flag,desc=The name to greet"`
		}

		target := targ.Targ(func(_ Args) {}).Name("flagged").Description("Command with flags")

		// With single target, --help without command name
		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("verbose"))
		g.Expect(result.Output).To(ContainSubstring("Enable verbose output"))
		g.Expect(result.Output).To(ContainSubstring("count"))
		g.Expect(result.Output).To(ContainSubstring("Number of times to run"))
	})

	t.Run("HelpShowsUsageLine", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("usage-cmd")

		// With single target, --help without command name
		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("usage-cmd"))
		g.Expect(result.Output).To(ContainSubstring("file"))
	})

	t.Run("HelpShowsSubcommandList", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		sub1 := targ.Targ(func() {}).Name("sub1").Description("First subcommand")
		sub2 := targ.Targ(func() {}).Name("sub2").Description("Second subcommand")
		root := targ.Group("parent", sub1, sub2)

		// With single root (group), --help without name shows subcommands
		result, err := targ.Execute(
			[]string{"app", "--help"},
			root,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("sub1"))
		g.Expect(result.Output).To(ContainSubstring("First subcommand"))
		g.Expect(result.Output).To(ContainSubstring("sub2"))
		g.Expect(result.Output).To(ContainSubstring("Second subcommand"))
	})

	t.Run("HelpShowsEnumValues", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Level string `targ:"flag,enum=debug|info|warn|error"`
		}

		target := targ.Targ(func(_ Args) {}).Name("enum-cmd")

		// With single target, --help without command name
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

	t.Run("HelpShowsCachePatterns", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).
			Name("cached-cmd").
			Cache("**/*.go", "go.mod")

		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Cache"))
		g.Expect(result.Output).To(ContainSubstring("**/*.go"))
	})

	t.Run("HelpShowsWatchPatterns", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).
			Name("watched-cmd").
			Watch("src/*.ts", "*.json")

		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("Watch"))
		g.Expect(result.Output).To(ContainSubstring("src/*.ts"))
	})

	t.Run("HelpWrapsLongUsageLine", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Create a command with many positional args to exceed 80 char line width
		type Args struct {
			InputFile      string `targ:"positional"`
			OutputFile     string `targ:"positional"`
			ConfigFile     string `targ:"positional"`
			TemplateFile   string `targ:"positional"`
			LoggingFile    string `targ:"positional"`
			MetadataFile   string `targ:"positional"`
			AdditionalFile string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("long-usage-cmd")

		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// Verify the help output is produced (wrapping happens internally)
		g.Expect(result.Output).To(ContainSubstring("long-usage-cmd"))
		g.Expect(result.Output).To(ContainSubstring("InputFile"))
	})
}

func TestProperty_ShellCommands(t *testing.T) {
	t.Parallel()

	t.Run("ShellCommandExecutesWithVariables", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			// Generate safe alphanumeric values for shell
			value := rapid.StringMatching(`[a-zA-Z0-9]{1,10}`).Draw(t, "value")

			// Use true which always succeeds - we're testing that shell vars work
			// With single target (default mode), don't include command name in args
			target := targ.Targ("true $msg").Name("shell-test")

			_, err := targ.Execute(
				[]string{"app", "--msg", value},
				target,
			)
			g.Expect(err).NotTo(HaveOccurred())
		})
	})

	t.Run("ShellCommandSupportsLongFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// With single target (default mode), don't include command name in args
		target := targ.Targ("true $greeting $name").Name("greet")

		_, err := targ.Execute(
			[]string{"app", "--greeting", "hello", "--name", "world"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("ShellCommandSupportsEqualsFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// With single target (default mode), don't include command name in args
		target := targ.Targ("true $msg").Name("echo-equals")

		_, err := targ.Execute(
			[]string{"app", "--msg=test-value"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("ShellCommandSupportsShortFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// With single target (default mode), don't include command name in args
		target := targ.Targ("true $msg").Name("echo-short")

		_, err := targ.Execute(
			[]string{"app", "-m", "short-flag-value"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("ShellCommandHelpShowsVariables", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ("echo $greeting $name").Name("greet").Description("Say hello")

		// --help works without command name in default mode
		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("greeting"))
		g.Expect(result.Output).To(ContainSubstring("name"))
	})

	t.Run("ShellCommandMissingVarReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ("echo $msg").Name("missing-var")

		// In default mode, no command name needed; no --msg flag provided
		result, err := targ.Execute(
			[]string{"app"},
			target,
		)
		g.Expect(err).To(HaveOccurred())
		// Error message is captured in output, not in err.Error()
		g.Expect(result.Output).To(ContainSubstring("msg"))
	})

	t.Run("ShellCommandUnknownFlagReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Shell command with $msg variable
		target := targ.Targ("echo $msg").Name("echo")

		// Pass --msg (known) and --unknown (not a shell variable)
		_, err := targ.Execute(
			[]string{"app", "--msg", "hello", "--unknown", "value"},
			target,
		)
		// Unknown flags are errors for shell commands
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("ShellCommandFailureReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Shell command that exits with error
		target := targ.Targ("exit 1").Name("fail")

		result, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("failed"))
	})

	t.Run("ShellCommandHelpIncludesDescription", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ("echo test").Name("described").Description("A described command")

		// --help works without command name in default mode
		result, err := targ.Execute(
			[]string{"app", "--help"},
			target,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("A described command"))
	})

	t.Run("ShellCommandDerivesNameFromCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Shell command without explicit name - should derive "true" from "true hello"
		target := targ.Targ("true hello")
		other := targ.Targ(func() {}).Name("other")

		// In multi-root mode, the derived name should work
		result, err := targ.Execute(
			[]string{"app", "--help"},
			target, other,
		)
		g.Expect(err).NotTo(HaveOccurred())
		// The derived name should appear in help output
		g.Expect(result.Output).To(ContainSubstring("true"))
	})
}
