package help_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	. "github.com/onsi/gomega"
)

func TestLipglossImportWorks(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Simply verify lipgloss is importable and basic functionality works
	style := lipgloss.NewStyle().Bold(true)
	g.Expect(style.Render("test")).To(ContainSubstring("test"))
}
