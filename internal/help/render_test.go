package help_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_RenderSectionOrderIsCorrect(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		// Build a help with all sections
		output := help.New("test").
			WithDescription("description").
			WithUsage("test [options]").
			AddPositionals(help.Positional{Name: "file"}).
			AddGlobalFlags(help.Flag{Long: "--help", Desc: "Show help"}).
			AddCommandFlags(help.Flag{Long: "--verbose"}).
			AddFormats(help.Format{Name: "fmt"}).
			AddSubcommands(help.Subcommand{Name: "sub"}).
			AddExamples(help.Example{Title: "ex", Code: "test"}).
			Render()

		// Section headers should appear in canonical order
		g := NewWithT(t)
		descIdx := indexOf(output, "description")
		usageIdx := indexOf(output, "Usage:")
		targFlagsIdx := indexOf(output, "Targ flags:")
		formatsIdx := indexOf(output, "Formats:")
		posIdx := indexOf(output, "Positionals:")
		flagsIdx := indexOf(output, "Flags:")
		subsIdx := indexOf(output, "Subcommands:")
		examplesIdx := indexOf(output, "Examples:")

		g.Expect(descIdx).To(BeNumerically("<", usageIdx), "description before usage")
		g.Expect(usageIdx).To(BeNumerically("<", targFlagsIdx), "usage before targ flags")
		g.Expect(targFlagsIdx).To(BeNumerically("<", formatsIdx), "targ flags before formats")
		g.Expect(formatsIdx).To(BeNumerically("<", posIdx), "formats before positionals")
		g.Expect(posIdx).To(BeNumerically("<", flagsIdx), "positionals before flags")
		g.Expect(flagsIdx).To(BeNumerically("<", subsIdx), "flags before subcommands")
		g.Expect(subsIdx).To(BeNumerically("<", examplesIdx), "subcommands before examples")
	})
}

func TestRenderCommandFlagsMinimalFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Flag with only Long set (no short, no placeholder, no desc)
	output := help.New("test").
		WithDescription("desc").
		AddCommandFlags(help.Flag{Long: "--simple"}).
		Render()

	g.Expect(output).To(ContainSubstring("--simple"))
}

func TestRenderCommandFlagsWithLongAndShort(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddCommandFlags(help.Flag{Long: "--verbose", Short: "-v", Desc: "Verbose output"}).
		Render()

	g.Expect(output).To(ContainSubstring("Flags:"))
	g.Expect(output).To(ContainSubstring("--verbose"))
	g.Expect(output).To(ContainSubstring("-v"))
	g.Expect(output).To(ContainSubstring("Verbose output"))
}

func TestRenderCommandFlagsWithLongNameOnly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddCommandFlags(help.Flag{Long: "--verbose-long-name-flag", Desc: "A verbose flag with a long name"}).
		Render()

	g.Expect(output).To(ContainSubstring("--verbose-long-name-flag"))
	g.Expect(output).To(ContainSubstring("A verbose flag with a long name"))
}

func TestRenderCommandFlagsWithPlaceholder(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddCommandFlags(help.Flag{Long: "--timeout", Placeholder: "<duration>", Desc: "Set timeout"}).
		Render()

	g.Expect(output).To(ContainSubstring("--timeout"))
	g.Expect(output).To(ContainSubstring("<duration>"))
	g.Expect(output).To(ContainSubstring("Set timeout"))
}

func TestRenderCommandGroups(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		SetRoot(true).
		AddCommandGroups(help.CommandGroup{
			Source:   "dev/targets.go",
			Commands: []help.Command{{Name: "build", Desc: "Build the project"}},
		}).
		Render()

	g.Expect(output).To(ContainSubstring("Commands:"))
	g.Expect(output).To(ContainSubstring("build"))
	g.Expect(output).To(ContainSubstring("Build the project"))
}

func TestRenderExampleWithoutTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Code: "test run --all"}).
		Render()

	g.Expect(output).To(ContainSubstring("Examples:"))
	g.Expect(output).To(ContainSubstring("test run --all"))
}

func TestRenderExecutionInfo(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		WithExecutionInfo(help.ExecutionInfo{
			Deps:    "build, fmt (serial)",
			Timeout: "30s",
		}).
		Render()

	g.Expect(output).To(ContainSubstring("Execution:"))
	g.Expect(output).To(ContainSubstring("build, fmt"))
	g.Expect(output).To(ContainSubstring("30s"))
}

func TestRenderGlobalFlags(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddGlobalFlags(help.Flag{Long: "--help", Short: "-h", Desc: "Show help"}).
		Render()

	g.Expect(output).To(ContainSubstring("Targ flags:"))
	g.Expect(output).To(ContainSubstring("--help"))
	g.Expect(output).To(ContainSubstring("-h"))
}

func TestRenderIncludesDescription(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("A test command").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("A test command"))
}

func TestRenderIncludesExamples(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic usage", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Examples:"))
	g.Expect(output).To(ContainSubstring("Basic usage"))
}

func TestRenderIncludesFormatsWhenPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddFormats(help.Format{Name: "duration", Desc: "e.g., 5s, 1m"}).
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Formats:"))
	g.Expect(output).To(ContainSubstring("duration"))
}

func TestRenderIncludesPositionalsWhenPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddPositionals(help.Positional{Name: "file", Placeholder: "<file>", Required: true}).
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Positionals:"))
	g.Expect(output).To(ContainSubstring("file"))
}

func TestRenderIncludesSubcommandsWhenPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddSubcommands(help.Subcommand{Name: "build", Desc: "Build project"}).
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Subcommands:"))
	g.Expect(output).To(ContainSubstring("build"))
}

func TestRenderIncludesUsage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		WithUsage("test [options] <file>").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Usage:"))
	g.Expect(output).To(ContainSubstring("test [options] <file>"))
}

func TestRenderIncludesValuesWhenPresent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddValues(help.Value{Name: "shell", Desc: "bash, zsh, or fish"}).
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("Values:"))
	g.Expect(output).To(ContainSubstring("shell"))
	g.Expect(output).To(ContainSubstring("bash, zsh, or fish"))
}

func TestRenderMoreInfoSection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		WithMoreInfo("https://example.com/docs").
		Render()

	g.Expect(output).To(ContainSubstring("More info:"))
	g.Expect(output).To(ContainSubstring("https://example.com/docs"))
}

func TestRenderOmitsEmptyFormats(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Formats:"))
}

func TestRenderOmitsEmptyPositionals(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Positionals:"))
}

func TestRenderOmitsEmptySubcommands(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Subcommands:"))
}

func TestRenderOmitsEmptyValues(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Values:"))
}

func TestRenderOmitsExamplesWhenExplicitlyDisabled(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// When AddExamples is called with no args, examples section is explicitly disabled
	output := help.New("test").WithDescription("desc").AddExamples().Render()
	g.Expect(output).NotTo(ContainSubstring("Examples:"))
}

func TestRenderOmitsExamplesWhenNotSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// When AddExamples is never called, examples section is omitted
	output := help.New("test").WithDescription("desc").Render()
	g.Expect(output).NotTo(ContainSubstring("Examples:"))
}

func TestRenderReturnsString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("A test command").
		WithUsage("test [options]").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(BeEmpty())
}

func TestRenderRootOnlyFlags(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		SetRoot(true).
		AddRootOnlyFlags(help.Flag{Long: "--source", Desc: "Source directory"}).
		Render()

	g.Expect(output).To(ContainSubstring("--source"))
	g.Expect(output).To(ContainSubstring("Source directory"))
}

func TestRenderShellCommand(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		WithShellCommand("go test ./...").
		Render()

	g.Expect(output).To(ContainSubstring("Command:"))
	g.Expect(output).To(ContainSubstring("go test ./..."))
}

func TestRenderSourceFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		WithSourceFile("dev/targets.go").
		Render()

	g.Expect(output).To(ContainSubstring("Source:"))
	g.Expect(output).To(ContainSubstring("dev/targets.go"))
}

// Helper to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}
