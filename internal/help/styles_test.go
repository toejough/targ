package help_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestLipglossImportWorks(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Simply verify lipgloss is importable and basic functionality works
	style := lipgloss.NewStyle().Bold(true)
	g.Expect(style.Render("test")).To(ContainSubstring("test"))
}

func TestStylesHasAllRequiredFields(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	styles := help.DefaultStyles()

	// Header should be bold
	g.Expect(styles.Header.GetBold()).To(BeTrue(), "Header should be bold")

	// Flag should have cyan color (ANSI 6)
	g.Expect(styles.Flag.GetForeground()).NotTo(BeNil(), "Flag should have foreground color")

	// Placeholder should have yellow color (ANSI 3)
	g.Expect(styles.Placeholder.GetForeground()).
		NotTo(BeNil(), "Placeholder should have foreground color")

	// Subsection should be bold (like Header but for nested sections)
	g.Expect(styles.Subsection.GetBold()).To(BeTrue(), "Subsection should be bold")
}
