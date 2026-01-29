package help_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_RenderIncludesValuesWhenPresent(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		count := rapid.IntRange(1, 4).Draw(t, "count")

		values := make([]help.Value, 0, count)
		for range count {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "name")
			desc := rapid.String().Draw(t, "desc")
			values = append(values, help.Value{Name: name, Desc: desc})
		}

		output := help.New("test").
			WithDescription("desc").
			AddValues(values...).
			AddExamples(help.Example{Title: "Basic", Code: "test run"}).
			Render()

		g.Expect(output).To(ContainSubstring("Values:"))

		for _, v := range values {
			g.Expect(output).To(ContainSubstring(v.Name))
			g.Expect(output).To(ContainSubstring(v.Desc))
		}
	})
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
			AddValues(help.Value{Name: "shell", Desc: "bash"}).
			AddSubcommands(help.Subcommand{Name: "sub"}).
			AddExamples(help.Example{Title: "ex", Code: "test"}).
			Render()

		// Section headers should appear in canonical order
		g := NewWithT(t)
		descIdx := indexOf(output, "description")
		usageIdx := indexOf(output, "Usage:")
		targFlagsIdx := indexOf(output, "Targ flags:")
		valuesIdx := indexOf(output, "Values:")
		formatsIdx := indexOf(output, "Formats:")
		posIdx := indexOf(output, "Positionals:")
		flagsIdx := indexOf(output, "Flags:")
		subsIdx := indexOf(output, "Subcommands:")
		examplesIdx := indexOf(output, "Examples:")

		g.Expect(descIdx).To(BeNumerically("<", usageIdx), "description before usage")
		g.Expect(usageIdx).To(BeNumerically("<", targFlagsIdx), "usage before targ flags")
		g.Expect(targFlagsIdx).To(BeNumerically("<", valuesIdx), "targ flags before values")
		g.Expect(valuesIdx).To(BeNumerically("<", formatsIdx), "values before formats")
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
