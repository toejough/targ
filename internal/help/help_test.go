package help_test

import (
	"testing"

	"github.com/toejough/targ/internal/help"
	. "github.com/onsi/gomega"
)

// TestPackageStructureExists verifies all expected types/functions exist.
// This is a structural test - if it compiles, the package structure is correct.
func TestPackageStructureExists(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// builder.go should export Builder type
	var _ help.Builder

	// content.go should export content types
	var _ help.Positional
	var _ help.Flag
	var _ help.Format
	var _ help.Subcommand
	var _ help.Example

	// render.go should export Render function
	_ = help.Render

	// styles.go should export Styles
	var _ help.Styles

	// formats.go should export FormatRegistry
	var _ help.FormatRegistry

	g.Expect(true).To(BeTrue()) // Test passes if we get here (compilation succeeded)
}
