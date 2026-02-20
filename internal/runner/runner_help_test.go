// TEST-007: Runner help properties - validates CLI help output for runner commands
// traces: ARCH-007

package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
	"github.com/toejough/targ/internal/runner"
)

// TestGoldenFile_HelpOutput compares help output against golden files.
// Set TARG_UPDATE_GOLDEN=1 to regenerate golden files.
func TestGoldenFile_HelpOutput(t *testing.T) {
	t.Parallel()

	type goldenTest struct {
		name       string
		fn         func(*strings.Builder)
		goldenFile string
	}

	tests := []goldenTest{
		{
			name:       "create",
			fn:         func(b *strings.Builder) { runner.PrintCreateHelp(b) },
			goldenFile: "create.golden",
		},
		{
			name:       "sync",
			fn:         func(b *strings.Builder) { runner.PrintSyncHelp(b) },
			goldenFile: "sync.golden",
		},
		{
			name:       "to-func",
			fn:         func(b *strings.Builder) { runner.PrintToFuncHelp(b) },
			goldenFile: "to-func.golden",
		},
		{
			name:       "to-string",
			fn:         func(b *strings.Builder) { runner.PrintToStringHelp(b) },
			goldenFile: "to-string.golden",
		},
	}

	updateGolden := os.Getenv("TARG_UPDATE_GOLDEN") == "1"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var buf strings.Builder

			tt.fn(&buf)

			// Strip ANSI codes for comparison (allows styling changes without test failures)
			actual := help.StripANSI(buf.String())

			goldenPath := filepath.Join("testdata", "golden", tt.goldenFile)

			if updateGolden {
				err := os.WriteFile(goldenPath, []byte(actual), 0o644)
				g.Expect(err).NotTo(HaveOccurred(), "failed to write golden file")
				t.Logf("Updated golden file: %s", goldenPath)

				return
			}

			expected, err := os.ReadFile(goldenPath)
			g.Expect(err).NotTo(HaveOccurred(), "failed to read golden file %s", goldenPath)

			g.Expect(actual).To(Equal(string(expected)),
				"help output does not match golden file %s\n"+
					"Run with TARG_UPDATE_GOLDEN=1 to update", goldenPath)
		})
	}
}

func TestProperty_ContainsHelpFlagMatchesArgs(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		count := rapid.IntRange(0, 8).Draw(t, "count")

		args := make([]string, 0, count)

		for range count {
			arg := rapid.String().Draw(t, "arg")
			args = append(args, arg)
		}

		expected := false

		for _, a := range args {
			if a == "--help" || a == "-h" {
				expected = true
				break
			}
		}

		g.Expect(runner.ContainsHelpFlag(args)).To(Equal(expected))
	})
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
	g.Expect(usageIdx).To(BeNumerically(">=", 0))

	if spec.hasPositionals {
		g.Expect(output).To(ContainSubstring("Positionals:"))
	}

	if spec.hasFlags {
		g.Expect(output).To(ContainSubstring("Flags:"))
		validateFlagsSection(g, lines)
	}

	if spec.hasFormats {
		g.Expect(output).To(ContainSubstring("Formats:"))
	}
}
