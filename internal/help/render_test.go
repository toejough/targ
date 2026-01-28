package help_test

import (
	"testing"

	"github.com/toejough/targ/internal/help"
	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

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

func TestRenderIncludesDescription(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("A test command").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).To(ContainSubstring("A test command"))
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

func TestRenderOmitsEmptyPositionals(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Positionals:"))
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

func TestRenderOmitsEmptyFormats(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Formats:"))
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

func TestRenderOmitsEmptySubcommands(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	output := help.New("test").
		WithDescription("desc").
		AddExamples(help.Example{Title: "Basic", Code: "test run"}).
		Render()

	g.Expect(output).NotTo(ContainSubstring("Subcommands:"))
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

func TestRenderOmitsExamplesWhenNotSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// When AddExamples is never called, examples section is omitted
	output := help.New("test").WithDescription("desc").Render()
	g.Expect(output).NotTo(ContainSubstring("Examples:"))
}

func TestRenderOmitsExamplesWhenExplicitlyDisabled(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// When AddExamples is called with no args, examples section is explicitly disabled
	output := help.New("test").WithDescription("desc").AddExamples().Render()
	g.Expect(output).NotTo(ContainSubstring("Examples:"))
}

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

// Helper to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
