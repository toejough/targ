package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestParallelOutputDepLevel(t *testing.T) {
	t.Parallel()

	t.Run("ParallelDepsProducePrefixedOutput", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		aCalled := false
		bCalled := false

		a := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "from a\n")

			aCalled = true
		}).Name("a")
		b := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "from b\n")

			bCalled = true
		}).Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		t.Logf("Output: %q", result.Output)
		t.Logf("Error: %v", err)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(aCalled).To(BeTrue())
		g.Expect(bCalled).To(BeTrue())

		// Output should contain prefixed lines and summary
		g.Expect(result.Output).To(ContainSubstring("[a]"))
		g.Expect(result.Output).To(ContainSubstring("[b]"))
		g.Expect(result.Output).To(ContainSubstring("from a"))
		g.Expect(result.Output).To(ContainSubstring("from b"))
		g.Expect(result.Output).To(ContainSubstring("PASS:2"))
	})

	t.Run("FailFastReportsCancelledTargets", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() error {
			return errors.New("boom")
		}).Name("a")
		b := core.Targ(func(ctx context.Context) error {
			// Simulate slow task that gets cancelled
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		}).Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, _ := core.Execute([]string{"app"}, main)

		// Should show FAIL for a, CANCELLED for b
		g.Expect(result.Output).To(ContainSubstring("FAIL"))
		g.Expect(result.Output).To(ContainSubstring("CANCELLED"))
	})

	t.Run("ErrorTextIsPrefixedAndBeforeSummary", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) error {
			core.Print(ctx, "diagnostic line\n")
			return errors.New("detailed error:\n  line 1\n  line 2")
		}).Name("check-a")
		b := core.Targ(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		}).Name("check-b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, _ := core.Execute([]string{"app"}, main)
		t.Logf("Output:\n%s", result.Output)

		// Error text should appear with prefix
		g.Expect(result.Output).To(ContainSubstring("[check-a] Error: detailed error:"))
		g.Expect(result.Output).To(ContainSubstring("[check-a]   line 1"))
		g.Expect(result.Output).To(ContainSubstring("[check-a]   line 2"))

		// Error text should appear BEFORE the summary
		summaryIdx := strings.Index(result.Output, "FAIL:")
		errorIdx := strings.Index(result.Output, "detailed error:")

		g.Expect(summaryIdx).To(BeNumerically(">", 0), "summary should be present")
		g.Expect(errorIdx).To(BeNumerically(">", 0), "error text should be present")
		g.Expect(errorIdx).To(BeNumerically("<", summaryIdx),
			"error text should appear before summary")

		// Error text should NOT appear unprefixed after the summary
		afterSummary := result.Output[summaryIdx:]
		g.Expect(afterSummary).ToNot(ContainSubstring("detailed error:"),
			"error text should not be repeated after summary")
	})

	t.Run("LifecycleMessagesAppear", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func() {}).Name("a")
		b := core.Targ(func() {}).Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		g.Expect(err).ToNot(HaveOccurred())

		// Default lifecycle messages should appear
		g.Expect(result.Output).To(ContainSubstring("starting..."))
		g.Expect(result.Output).To(ContainSubstring("PASS"))
	})

	t.Run("PrefixesAreAligned", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "hi\n")
		}).Name("build")
		b := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "hi\n")
		}).Name("a")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		g.Expect(err).ToNot(HaveOccurred())

		// "a" prefix should be padded to match "build" length
		g.Expect(result.Output).To(ContainSubstring("[a]     "))
		g.Expect(result.Output).To(ContainSubstring("[build] "))
	})
}

func TestParallelOutputShellCommand(t *testing.T) {
	t.Parallel()

	t.Run("ShellOutputIsPrefixed", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ("echo hello").Name("a")
		b := core.Targ("echo world").Name("b")
		main := core.Targ(func() {}).Name("main").Deps(a, b, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		t.Logf("Output: %q", result.Output)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(result.Output).To(ContainSubstring("[a]"))
		g.Expect(result.Output).To(ContainSubstring("[b]"))
		g.Expect(result.Output).To(ContainSubstring("hello"))
		g.Expect(result.Output).To(ContainSubstring("world"))
	})

	t.Run("MultiLineShellOutputPrefixesEachLine", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ("printf 'line1\\nline2\\n'").Name("a")
		main := core.Targ(func() {}).Name("main").Deps(a, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		t.Logf("Output: %q", result.Output)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(result.Output).To(ContainSubstring("[a] line1\n"))
		g.Expect(result.Output).To(ContainSubstring("[a] line2\n"))
	})
}

func TestParallelOutputTopLevel(t *testing.T) {
	t.Parallel()

	t.Run("TopLevelParallelProducesPrefixedOutput", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "hello\n")
		}).Name("a")
		b := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "world\n")
		}).Name("b")

		result, err := core.ExecuteWithOptions(
			[]string{"app", "--parallel", "a", "b"},
			core.RunOptions{},
			a, b,
		)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(result.Output).To(ContainSubstring("[a]"))
		g.Expect(result.Output).To(ContainSubstring("[b]"))
		g.Expect(result.Output).To(ContainSubstring("PASS:2"))
	})

	t.Run("ErrorTextIsPrefixedAndBeforeSummary", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fail := core.Targ(func(ctx context.Context) error {
			core.Print(ctx, "diagnostic\n")
			return errors.New("top-level error:\n  some detail")
		}).Name("lint")
		pass := core.Targ(func(ctx context.Context) {
			core.Print(ctx, "all good\n")
		}).Name("test")

		result, _ := core.ExecuteWithOptions(
			[]string{"app", "--parallel", "lint", "test"},
			core.RunOptions{},
			fail, pass,
		)
		t.Logf("Output:\n%s", result.Output)

		// Error text should appear with prefix
		g.Expect(result.Output).To(ContainSubstring("[lint]"))
		g.Expect(result.Output).To(ContainSubstring("top-level error:"))

		// Error text should appear BEFORE the summary
		summaryIdx := strings.Index(result.Output, "FAIL:")
		errorIdx := strings.Index(result.Output, "top-level error:")

		g.Expect(summaryIdx).To(BeNumerically(">", 0))
		g.Expect(errorIdx).To(BeNumerically(">", 0))
		g.Expect(errorIdx).To(BeNumerically("<", summaryIdx),
			"error text should appear before summary")
	})
}

func TestRunContext(t *testing.T) {
	t.Parallel()

	t.Run("SerialModeSucceeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		err := core.RunContext(context.Background(), "echo", "hello")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("SerialModeReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		err := core.RunContext(context.Background(), "false")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestRunContextInParallelMode(t *testing.T) {
	t.Parallel()

	t.Run("ParallelModeRoutesOutputThroughPrinter", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) {
			_ = core.RunContext(ctx, "echo", "from-runcontext")
		}).Name("a")
		main := core.Targ(func() {}).Name("main").Deps(a, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("from-runcontext"))
	})
}

func TestRunContextV(t *testing.T) {
	t.Parallel()

	t.Run("SerialModeSucceeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		err := core.RunContextV(context.Background(), "echo", "hello")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("SerialModeReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		err := core.RunContextV(context.Background(), "false")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestRunContextVInParallelMode(t *testing.T) {
	t.Parallel()

	t.Run("ParallelModeRoutesOutputThroughPrinter", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		a := core.Targ(func(ctx context.Context) {
			_ = core.RunContextV(ctx, "echo", "from-runcontextv")
		}).Name("a")
		main := core.Targ(func() {}).Name("main").Deps(a, core.DepModeParallel)

		result, err := core.Execute([]string{"app"}, main)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("from-runcontextv"))
	})
}
