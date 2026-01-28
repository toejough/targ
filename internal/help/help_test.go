package help_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

// TestPackageStructureExists verifies all expected types/functions exist.
// This is a structural test - if it compiles, the package structure is correct.
func TestPackageStructureExists(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// builder.go should export Builder type
	var _ help.Builder

	// content.go should export content types
	var (
		_ help.Positional
		_ help.Flag
		_ help.Format
		_ help.Subcommand
		_ help.Example
		_ help.ContentBuilder
	)

	// render.go - Render is a method on ContentBuilder, not standalone
	// Verified by the render_test.go tests

	// styles.go should export Styles
	var _ help.Styles

	g.Expect(true).To(BeTrue()) // Test passes if we get here (compilation succeeded)
}
