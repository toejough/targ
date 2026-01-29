package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestWriteRootHelpWithDeregisteredPackages(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteRootHelp(&buf, help.RootHelpOpts{
		BinaryName:           "targ",
		Description:          "Test description",
		DeregisteredPackages: []string{"github.com/foo/bar", "github.com/baz/qux"},
	})
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Deregistered packages"))
	g.Expect(output).To(ContainSubstring("github.com/foo/bar"))
	g.Expect(output).To(ContainSubstring("github.com/baz/qux"))
}
