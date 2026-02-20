package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

//nolint:paralleltest // serial: Execute() mutates package-level printOutput
func TestParallelOutputDepLevel(t *testing.T) {
	t.Run("ParallelDepsProducePrefixedOutput", func(t *testing.T) {
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

	t.Run("LifecycleMessagesAppear", func(t *testing.T) {
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

//nolint:paralleltest // serial: Execute() mutates package-level printOutput
func TestParallelOutputShellCommand(t *testing.T) {
	t.Run("ShellOutputIsPrefixed", func(t *testing.T) {
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

//nolint:paralleltest // serial: Execute() mutates package-level printOutput
func TestParallelOutputTopLevel(t *testing.T) {
	t.Run("TopLevelParallelProducesPrefixedOutput", func(t *testing.T) {
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

//nolint:paralleltest // serial: Execute() mutates package-level printOutput
func TestRunContextInParallelMode(t *testing.T) {
	t.Run("ParallelModeRoutesOutputThroughPrinter", func(t *testing.T) {
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

//nolint:paralleltest // serial: Execute() mutates package-level printOutput
func TestRunContextVInParallelMode(t *testing.T) {
	t.Run("ParallelModeRoutesOutputThroughPrinter", func(t *testing.T) {
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
