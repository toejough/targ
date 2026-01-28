package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestStripANSIWithEmptyString(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := help.StripANSI("")
	g.Expect(result).To(Equal(""))
}

func TestStripANSIWithEscapeCodes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// ANSI escape for bold: \x1b[1m ... \x1b[0m
	input := "\x1b[1mhello\x1b[0m world"
	result := help.StripANSI(input)
	g.Expect(result).To(Equal("hello world"))
}

func TestStripANSIWithMultipleEscapeCodes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Multiple ANSI codes
	input := "\x1b[1m\x1b[36mhello\x1b[0m \x1b[33mworld\x1b[0m"
	result := help.StripANSI(input)
	g.Expect(result).To(Equal("hello world"))
}

func TestStripANSIWithPlainText(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result := help.StripANSI("hello world")
	g.Expect(result).To(Equal("hello world"))
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

func TestWriteFormatLineFormatsCorrectly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFormatLine(&buf, "duration", "<int><unit>")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("duration"))
	g.Expect(output).To(ContainSubstring("<int><unit>"))
}

func TestWriteHeaderContainsText(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteHeader(&buf, "Test:")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("Test:"))
	g.Expect(output).To(HaveSuffix("\n"))
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
