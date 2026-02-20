// TEST-006: Help rendering properties - validates help output format and structure
// traces: ARCH-007

package help_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_ANSICodesPairedCorrectly(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Build help with various sections that use styling
		output := help.New("test").
			WithDescription("description").
			AddGlobalFlags(help.Flag{Long: "--verbose", Short: "-v", Desc: "Verbose"}).
			AddCommandFlags(help.Flag{Long: "--output", Placeholder: "<file>"}).
			AddExamples(help.Example{Title: "Run", Code: "targ run"}).
			Render()

		// Count ANSI escape sequences (CSI sequences start with \x1b[)
		// Each style start should have a corresponding reset (\x1b[0m)
		escapeCount := 0
		resetCount := 0

		for i := range len(output) - 1 {
			if output[i] == '\x1b' && i+1 < len(output) && output[i+1] == '[' {
				// Check if this is a reset sequence
				if i+3 < len(output) && output[i+2] == '0' && output[i+3] == 'm' {
					resetCount++
				} else {
					escapeCount++
				}
			}
		}

		// Every style should be reset (or no styling at all)
		g.Expect(escapeCount).To(Equal(resetCount),
			"ANSI escapes (%d) should equal resets (%d)", escapeCount, resetCount)
	})
}

func TestProperty_EmptySectionsOmitted(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Build help with only description and usage (no other sections)
		output := help.New("test").
			WithDescription("description").
			WithUsage("test [options]").
			AddExamples(). // Explicitly empty
			Render()

		// These sections should NOT appear since they have no content
		g.Expect(output).NotTo(ContainSubstring("Targ flags:"))
		g.Expect(output).NotTo(ContainSubstring("Flags:"))
		g.Expect(output).NotTo(ContainSubstring("Positionals:"))
		g.Expect(output).NotTo(ContainSubstring("Subcommands:"))
		g.Expect(output).NotTo(ContainSubstring("Commands:"))
		g.Expect(output).NotTo(ContainSubstring("Values:"))
		g.Expect(output).NotTo(ContainSubstring("Formats:"))
		g.Expect(output).NotTo(ContainSubstring("Examples:"))
		g.Expect(output).NotTo(ContainSubstring("Execution:"))
		g.Expect(output).NotTo(ContainSubstring("More info:"))

		// But these should appear
		g.Expect(output).To(ContainSubstring("description"))
		g.Expect(output).To(ContainSubstring("Usage:"))
	})
}

func TestProperty_ExamplesHaveNoANSICodes(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate example code
		suffix := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "suffix")
		code := "targ " + suffix

		output := help.New("test").
			WithDescription("description").
			AddExamples(help.Example{Title: "Test", Code: code}).
			Render()

		// Find the examples section and check the code line
		lines := splitLines(output)
		inExamples := false

		for _, line := range lines {
			if indexOf(line, "Examples:") >= 0 {
				inExamples = true
				continue
			}

			if inExamples && indexOf(line, code) >= 0 {
				// The code itself should not contain ANSI codes
				// (it may have ANSI codes around it for the header, but the code content shouldn't)
				codeStart := indexOf(line, code)
				codeEnd := codeStart + len(code)
				codeSection := line[codeStart:codeEnd]
				g.Expect(codeSection).To(Equal(help.StripANSI(codeSection)),
					"example code should not contain ANSI codes")
			}
		}
	})
}

func TestProperty_GlobalFlagsBeforeCommandFlags(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random flag names
		globalFlag := "--" + rapid.StringMatching(`g[a-z]{2,6}`).Draw(t, "globalFlag")
		commandFlag := "--" + rapid.StringMatching(`c[a-z]{2,6}`).Draw(t, "commandFlag")

		output := help.New("test").
			WithDescription("description").
			AddGlobalFlags(help.Flag{Long: globalFlag, Desc: "A global flag"}).
			AddCommandFlags(help.Flag{Long: commandFlag, Desc: "A command flag"}).
			Render()

		// "Global flags:" section (contains global flags) should appear before "Flags:" section
		globalFlagsIdx := indexOf(output, "Global flags:")
		flagsIdx := indexOf(output, "Flags:")

		// Both sections should exist
		g.Expect(globalFlagsIdx).To(BeNumerically(">=", 0), "Global flags section should exist")
		g.Expect(flagsIdx).To(BeNumerically(">=", 0), "Flags section should exist")

		// Global flags should come before Flags
		g.Expect(globalFlagsIdx).To(BeNumerically("<", flagsIdx),
			"Global flags section should appear before Flags (command) section")
	})
}

func TestProperty_NoTrailingWhitespace(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Generate random flag names and descriptions that start/end with non-whitespace
		// (whitespace-only descriptions are an edge case that would need separate handling)
		flagName := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "flagName")
		desc := rapid.StringMatching(`[a-z][a-z ]{3,18}[a-z]`).Draw(t, "desc")

		output := help.New("test").
			WithDescription("description").
			AddGlobalFlags(help.Flag{Long: "--" + flagName, Desc: desc}).
			AddExamples(help.Example{Title: "Ex", Code: "targ test"}).
			Render()

		// Check each line for trailing whitespace
		lines := splitLines(output)
		for i, line := range lines {
			stripped := help.StripANSI(line)
			if stripped == "" {
				continue // Empty lines are fine
			}
			// Line should not end with space or tab
			g.Expect(stripped).
				NotTo(HaveSuffix(" "), "line %d has trailing space: %q", i+1, stripped)
			g.Expect(stripped).
				NotTo(HaveSuffix("\t"), "line %d has trailing tab: %q", i+1, stripped)
		}
	})
}

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
		globalFlagsIdx := indexOf(output, "Global flags:")
		valuesIdx := indexOf(output, "Values:")
		formatsIdx := indexOf(output, "Formats:")
		posIdx := indexOf(output, "Positionals:")
		flagsIdx := indexOf(output, "Flags:")
		subsIdx := indexOf(output, "Subcommands:")
		examplesIdx := indexOf(output, "Examples:")

		g.Expect(descIdx).To(BeNumerically("<", usageIdx), "description before usage")
		g.Expect(usageIdx).To(BeNumerically("<", globalFlagsIdx), "usage before global flags")
		g.Expect(globalFlagsIdx).To(BeNumerically("<", valuesIdx), "global flags before values")
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

// Helper to split string into lines
func splitLines(s string) []string {
	var lines []string

	start := 0

	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}

	if start < len(s) {
		lines = append(lines, s[start:])
	}

	return lines
}
