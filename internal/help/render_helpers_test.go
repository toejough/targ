package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestStripANSIWithEscapeCodes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// ANSI escape for bold: \x1b[1m ... \x1b[0m
	input := "\x1b[1mhello\x1b[0m world"
	result := help.StripANSI(input)
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
