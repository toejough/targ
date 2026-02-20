package core_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestPrinter(t *testing.T) {
	t.Parallel()

	t.Run("SendAndClose", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)

		p.Send("[build] compiling...\n")
		p.Send("[test]  running...\n")
		p.Close()

		output := buf.String()
		g.Expect(output).To(ContainSubstring("[build] compiling...\n"))
		g.Expect(output).To(ContainSubstring("[test]  running...\n"))
	})

	t.Run("PreservesOrder", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)

		for i := range 100 {
			p.Send(strings.Repeat("x", i) + "\n")
		}

		p.Close()

		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		g.Expect(lines).To(HaveLen(100))

		for i, line := range lines {
			g.Expect(line).To(Equal(strings.Repeat("x", i)))
		}
	})

	t.Run("CloseFlushesAll", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 1) // tiny buffer

		p.Send("line1\n")
		p.Send("line2\n")
		p.Send("line3\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("line1\nline2\nline3\n"))
	})
}
