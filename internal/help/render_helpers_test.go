package help_test

import (
	"strings"
	"testing"

	"github.com/toejough/targ/internal/help"
	. "github.com/onsi/gomega"
)

func TestWriteHeaderContainsText(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteHeader(&buf, "Test:")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Test:"))
	g.Expect(output).To(HaveSuffix("\n"))
}

func TestWriteFlagLineFormatsCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFlagLine(&buf, "timeout", "", "<duration>", "Set timeout")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("--timeout"))
	g.Expect(output).To(ContainSubstring("<duration>"))
	g.Expect(output).To(ContainSubstring("Set timeout"))
}

func TestWriteFlagLineWithShortForm(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFlagLine(&buf, "help", "h", "", "Show help")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("--help"))
	g.Expect(output).To(ContainSubstring("-h"))
}

func TestWriteValueLineFormatsCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteValueLine(&buf, "shell", "bash, zsh, fish")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("shell"))
	g.Expect(output).To(ContainSubstring("bash, zsh, fish"))
}

func TestWriteFormatLineFormatsCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFormatLine(&buf, "duration", "<int><unit>")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("duration"))
	g.Expect(output).To(ContainSubstring("<int><unit>"))
}

func TestWriteExampleWithTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteExample(&buf, "Basic usage", "targ build")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Basic usage:"))
	g.Expect(output).To(ContainSubstring("targ build"))
}

func TestWriteExampleWithoutTitle(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteExample(&buf, "", "targ build")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("targ build"))
	g.Expect(output).NotTo(ContainSubstring(":"))
}
