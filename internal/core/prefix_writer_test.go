package core_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestPrefixWriter(t *testing.T) {
	t.Parallel()

	t.Run("CompleteLine", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[build] ", p)

		_, err := w.Write([]byte("compiling...\n"))
		g.Expect(err).ToNot(HaveOccurred())
		p.Close()

		g.Expect(buf.String()).To(Equal("[build] compiling...\n"))
	})

	t.Run("PartialLinesFlushedOnClose", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[test] ", p)

		_, _ = w.Write([]byte("partial"))
		w.Flush()
		p.Close()

		g.Expect(buf.String()).To(Equal("[test] partial\n"))
	})

	t.Run("MultipleLines", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[build] ", p)

		_, _ = w.Write([]byte("line1\nline2\nline3\n"))

		p.Close()

		g.Expect(buf.String()).To(Equal("[build] line1\n[build] line2\n[build] line3\n"))
	})

	t.Run("ChunkedWrites", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)
		w := core.NewPrefixWriter("[x] ", p)

		_, _ = w.Write([]byte("hel"))
		_, _ = w.Write([]byte("lo\nwor"))
		_, _ = w.Write([]byte("ld\n"))

		p.Close()

		g.Expect(buf.String()).To(Equal("[x] hello\n[x] world\n"))
	})
}
