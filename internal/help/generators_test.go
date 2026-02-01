// TEST-022: Help generators properties - validates root help with deregistered packages
// traces: ARCH-007, ARCH-003

package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_WriteRootHelpWithDeregisteredPackages(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		count := rapid.IntRange(1, 4).Draw(t, "count")

		pkgs := make([]string, 0, count)
		for range count {
			pkg := rapid.StringMatching(`[a-z]+\.[a-z]+/[a-z0-9-]+/[a-z0-9-]+`).Draw(t, "pkg")
			pkgs = append(pkgs, pkg)
		}

		var buf strings.Builder
		help.WriteRootHelp(&buf, help.RootHelpOpts{
			BinaryName:           "targ",
			Description:          "Test description",
			DeregisteredPackages: pkgs,
		})
		output := buf.String()

		g.Expect(output).To(ContainSubstring("Deregistered packages"))

		for _, pkg := range pkgs {
			g.Expect(output).To(ContainSubstring(pkg))
		}
	})
}
