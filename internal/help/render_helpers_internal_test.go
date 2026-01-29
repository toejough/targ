package help_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/help"
)

func TestWriteFormatLineWritesOutput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buf strings.Builder
	help.WriteFormatLine(&buf, "duration", "<n><unit>")
	output := buf.String()

	g.Expect(output).To(ContainSubstring("duration"))
	g.Expect(output).To(ContainSubstring("<n><unit>"))
}
