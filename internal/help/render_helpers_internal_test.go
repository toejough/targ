package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestWriteFlagLineIndentPadsShortLine(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFlagLineIndent(&buf, "h", "", "", "Help", "  ")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("--h"))
	g.Expect(output).To(ContainSubstring("Help"))
}

func TestWriteFlagLineIndentUsesLongLineSpacing(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFlagLineIndent(&buf, "this-is-a-very-long-flag-name", "v", "<value>", "desc", "  ")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("--this-is-a-very-long-flag-name"))
	g.Expect(output).To(ContainSubstring("-v"))
	g.Expect(output).To(ContainSubstring("<value>"))
	g.Expect(output).To(ContainSubstring("desc"))
}

func TestWriteFormatLineWritesOutput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFormatLine(&buf, "duration", "<n><unit>")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("duration"))
	g.Expect(output).To(ContainSubstring("<n><unit>"))
}
