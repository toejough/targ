package runner_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/runner"
)

func TestContainsHelpFlag(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(runner.ContainsHelpFlag([]string{"--help"})).To(BeTrue())
	g.Expect(runner.ContainsHelpFlag([]string{"-h"})).To(BeTrue())
	g.Expect(runner.ContainsHelpFlag([]string{"foo", "--help"})).To(BeTrue())
	g.Expect(runner.ContainsHelpFlag([]string{"foo", "-h"})).To(BeTrue())
	g.Expect(runner.ContainsHelpFlag([]string{"foo", "bar"})).To(BeFalse())
	g.Expect(runner.ContainsHelpFlag([]string{})).To(BeFalse())
}

func TestProperty_HelpOutputStructure(t *testing.T) {
	t.Parallel()

	type helpFunc struct {
		name string
		fn   func(*strings.Builder)
		spec helpSpec
	}

	helpers := []helpFunc{
		{
			name: "create",
			fn:   func(b *strings.Builder) { runner.PrintCreateHelp(b) },
			spec: helpSpec{
				command:        "--create",
				hasPositionals: true,
				hasFlags:       true,
				hasFormats:     true,
			},
		},
		{
			name: "sync",
			fn:   func(b *strings.Builder) { runner.PrintSyncHelp(b) },
			spec: helpSpec{
				command:        "--sync",
				hasPositionals: false,
				hasFlags:       false,
				hasFormats:     false,
			},
		},
		{
			name: "to-func",
			fn:   func(b *strings.Builder) { runner.PrintToFuncHelp(b) },
			spec: helpSpec{
				command:        "--to-func",
				hasPositionals: false,
				hasFlags:       false,
				hasFormats:     false,
			},
		},
		{
			name: "to-string",
			fn:   func(b *strings.Builder) { runner.PrintToStringHelp(b) },
			spec: helpSpec{
				command:        "--to-string",
				hasPositionals: false,
				hasFlags:       false,
				hasFormats:     false,
			},
		},
	}

	for _, h := range helpers {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(t *rapid.T) {
				g := NewWithT(t)

				var buf strings.Builder
				h.fn(&buf)
				validateHelpOutput(g, buf.String(), h.spec)
			})
		})
	}
}

// helpSpec describes the expected sections in help output for property tests.
type helpSpec struct {
	command        string // e.g. "--create"
	hasPositionals bool
	hasFlags       bool
	hasFormats     bool
}

func validateFlagsSection(g Gomega, lines []string) {
	inFlags := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, "Flags:") {
			inFlags = true
			continue
		}

		if !inFlags || trimmed == "" {
			continue
		}

		// Stop at next section (ends with colon, not a flag)
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, "--") {
			inFlags = false
			continue
		}

		// Skip subsection headers like "Global Flags:" or "Command Flags:"
		if strings.HasSuffix(trimmed, ":") {
			continue
		}

		g.Expect(line).To(ContainSubstring("--"),
			"flag line should contain --: %q", trimmed)
	}
}

// validateHelpOutput checks structural invariants of help output.
func validateHelpOutput(g Gomega, output string, spec helpSpec) {
	g.Expect(output).NotTo(BeEmpty(), "help output should not be empty")

	lines := strings.Split(output, "\n")

	// First line is non-empty description
	g.Expect(lines[0]).NotTo(BeEmpty(), "first line should be non-empty description")
	// Second line is blank
	g.Expect(lines[1]).To(BeEmpty(), "second line should be blank")

	// "Usage:" line exists and contains binary name and command flag
	g.Expect(output).To(ContainSubstring("Usage:"))
	g.Expect(output).To(ContainSubstring("targ"))
	g.Expect(output).To(ContainSubstring(spec.command))

	// Section ordering
	usageIdx := strings.Index(output, "Usage:")
	examplesIdx := strings.Index(output, "Examples:")
	g.Expect(examplesIdx).To(BeNumerically(">", usageIdx),
		"Examples should come after Usage")

	if spec.hasPositionals {
		posIdx := strings.Index(output, "Positionals:")
		g.Expect(posIdx).To(BeNumerically(">", usageIdx),
			"Positionals should come after Usage")
		g.Expect(posIdx).To(BeNumerically("<", examplesIdx),
			"Positionals should come before Examples")
	}

	if spec.hasFlags {
		flagsIdx := strings.Index(output, "Flags:")
		g.Expect(flagsIdx).To(BeNumerically(">", usageIdx),
			"Flags should come after Usage")
		g.Expect(flagsIdx).To(BeNumerically("<", examplesIdx),
			"Flags should come before Examples")
	}

	if spec.hasFormats {
		formatsIdx := strings.Index(output, "Formats:")
		g.Expect(formatsIdx).To(BeNumerically(">", usageIdx),
			"Formats should come after Usage")
		g.Expect(formatsIdx).To(BeNumerically("<", examplesIdx),
			"Formats should come before Examples")
	}

	// No trailing whitespace on any line
	for i, line := range lines {
		g.Expect(line).To(Equal(strings.TrimRight(line, " \t")),
			"line %d has trailing whitespace: %q", i+1, line)
	}

	// Each example starts with "targ"
	inExamples := false

	for _, line := range lines {
		if strings.HasPrefix(line, "Examples:") {
			inExamples = true
			continue
		}

		if inExamples && strings.TrimSpace(line) != "" {
			trimmed := strings.TrimSpace(line)
			g.Expect(trimmed).To(HavePrefix("targ "),
				"example should start with 'targ': %q", trimmed)
		}
	}

	// If Flags: section exists, every flag line contains --
	if spec.hasFlags {
		validateFlagsSection(g, lines)
	}
}
