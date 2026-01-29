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

// Helper to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}
