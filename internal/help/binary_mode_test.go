// TEST-033: Binary mode help output
package help_test

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestBinaryModeHelpOutput(t *testing.T) {
	t.Parallel()

	t.Run("RootHelpInBinaryMode", func(t *testing.T) {
		g := NewWithT(t)

		var buf bytes.Buffer

		// Simulate binary mode help
		opts := help.RootHelpOpts{
			BinaryName:  "myapp",
			Description: "My application",
			Filter: help.TargFlagFilter{
				IsRoot:     true,
				BinaryMode: true, // New flag to indicate binary mode
			},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		// Should use "[flags...]" not "[targ flags...]"
		g.Expect(output).To(ContainSubstring("myapp [flags...]"),
			"usage should show [flags...] in binary mode")
		g.Expect(output).ToNot(ContainSubstring("[targ flags...]"),
			"usage should not show [targ flags...] in binary mode")

		// Should only show --help and --completion
		g.Expect(output).To(ContainSubstring("--help"))
		g.Expect(output).To(ContainSubstring("--completion"))

		// Should NOT show runtime flags
		g.Expect(output).ToNot(ContainSubstring("--timeout"),
			"should not show --timeout in binary mode")
		g.Expect(output).ToNot(ContainSubstring("--parallel"),
			"should not show --parallel in binary mode")
		g.Expect(output).ToNot(ContainSubstring("--times"),
			"should not show --times in binary mode")
		g.Expect(output).ToNot(ContainSubstring("--retry"),
			"should not show --retry in binary mode")
		g.Expect(output).ToNot(ContainSubstring("--watch"),
			"should not show --watch in binary mode")
		g.Expect(output).ToNot(ContainSubstring("--cache"),
			"should not show --cache in binary mode")
	})

	t.Run("TargetHelpInBinaryMode", func(t *testing.T) {
		g := NewWithT(t)

		var buf bytes.Buffer

		opts := help.TargetHelpOpts{
			BinaryName:  "myapp",
			Name:        "build",
			Description: "Build the project",
			Filter: help.TargFlagFilter{
				IsRoot:     false,
				BinaryMode: true,
			},
		}

		help.WriteTargetHelp(&buf, opts)
		output := buf.String()

		// Should use "[flags...]" not "[targ flags...]"
		g.Expect(output).To(ContainSubstring("myapp [flags...] build"),
			"usage should show [flags...] in binary mode")
		g.Expect(output).ToNot(ContainSubstring("[targ flags...]"),
			"usage should not show [targ flags...] in binary mode")
	})

	t.Run("ExamplesUseBinaryName", func(t *testing.T) {
		g := NewWithT(t)

		var buf bytes.Buffer

		opts := help.RootHelpOpts{
			BinaryName:  "myapp",
			Description: "My application",
			Examples: []help.Example{
				{Title: "Run tests", Code: "myapp test"},
			},
			Filter: help.TargFlagFilter{
				IsRoot:     true,
				BinaryMode: true,
			},
		}

		help.WriteRootHelp(&buf, opts)
		output := buf.String()

		// Examples should use binary name, not "targ"
		g.Expect(output).To(ContainSubstring("myapp test"))
		g.Expect(output).ToNot(ContainSubstring("targ test"),
			"examples should use binary name, not 'targ'")
	})
}
