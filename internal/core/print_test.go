package core_test

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

// TestPrint tests are NOT parallel because they set package-level globals
// (printOutput) that would race across concurrent subtests.
func TestPrint(t *testing.T) { //nolint:tparallel // intentionally serial: tests mutate package-level printOutput
	t.Run("SerialWritesDirectly", func(t *testing.T) { //nolint:paralleltest // serial: mutates package-level printOutput
		g := NewWithT(t)

		var buf strings.Builder

		core.SetPrintOutput(&buf)

		defer core.SetPrintOutput(nil)

		ctx := context.Background()
		core.Print(ctx, "hello world\n")

		g.Expect(buf.String()).To(Equal("hello world\n"))
	})

	//nolint:paralleltest // serial: mutates package-level printOutput
	t.Run("ParallelPrefixesAndSendsToPrinter", func(t *testing.T) {
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "build",
			Printer:  p,
		})

		core.Print(ctx, "compiling...\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("[build] compiling...\n"))
	})

	t.Run("PrintfFormatsCorrectly", func(t *testing.T) { //nolint:paralleltest // serial: mutates package-level printOutput
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "test",
			Printer:  p,
		})

		core.Printf(ctx, "result: %d\n", 42)
		p.Close()

		g.Expect(buf.String()).To(Equal("[test] result: 42\n"))
	})

	//nolint:paralleltest // serial: mutates package-level printOutput
	t.Run("PrintfSerialWritesDirectly", func(t *testing.T) {
		g := NewWithT(t)

		var buf strings.Builder

		core.SetPrintOutput(&buf)

		defer core.SetPrintOutput(nil)

		ctx := context.Background()
		core.Printf(ctx, "count: %d\n", 7)

		g.Expect(buf.String()).To(Equal("count: 7\n"))
	})

	//nolint:paralleltest // serial: mutates package-level printOutput
	t.Run("MultiLineSplitsAndPrefixesEach", func(t *testing.T) {
		g := NewWithT(t)

		var buf strings.Builder

		p := core.NewPrinter(&buf, 10)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "lint",
			Printer:  p,
		})

		core.Print(ctx, "line1\nline2\n")
		p.Close()

		g.Expect(buf.String()).To(Equal("[lint] line1\n[lint] line2\n"))
	})

	t.Run("FormatPrefixWithPadding", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// maxLen=5 (e.g., "build")
		g.Expect(core.FormatPrefix("build", 5)).To(Equal("[build] "))
		g.Expect(core.FormatPrefix("test", 5)).To(Equal("[test]  "))
		g.Expect(core.FormatPrefix("a", 5)).To(Equal("[a]     "))
	})
}
