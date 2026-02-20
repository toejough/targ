// TEST-034: Binary mode propagation - validates BinaryMode flows from RunOptions to help
// traces: ARCH-007

package core_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestBinaryModePropagation(t *testing.T) {
	t.Parallel()

	t.Run("BinaryModeTrue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Create multiple targets to force root help (no auto-selection)
		target1 := core.Targ(func() {}).Name("build")
		target2 := core.Targ(func() {}).Name("test")

		// Create mock environment with --help (root help, no specific command)
		env := core.NewExecuteEnv([]string{"myapp", "--help"})

		// Create new state and register targets
		state := core.NewRegistryState()
		state.RegisterTarget(target1, target2)

		// Execute with BinaryMode=true
		err := state.ExecuteWithResolution(env, core.RunOptions{
			AllowDefault: true,
			BinaryMode:   true,
		})

		g.Expect(err).NotTo(HaveOccurred())

		output := env.Output()

		// In binary mode, should NOT show targ-only flags like --timeout, --parallel, --source
		g.Expect(output).ToNot(ContainSubstring("--timeout"),
			"binary mode should not show --timeout flag")
		g.Expect(output).ToNot(ContainSubstring("--parallel"),
			"binary mode should not show --parallel flag")
		g.Expect(output).ToNot(ContainSubstring("--source"),
			"binary mode should not show --source flag")
		g.Expect(output).ToNot(ContainSubstring("--create"),
			"binary mode should not show --create flag")
		g.Expect(output).ToNot(ContainSubstring("--no-binary-cache"),
			"binary mode should not show --no-binary-cache flag")

		// Should still show FlagModeAll flags (help and completion)
		g.Expect(output).To(ContainSubstring("--help"))
		g.Expect(output).To(ContainSubstring("--completion"))
	})

	t.Run("BinaryModeFalse", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Create a simple target
		target := core.Targ(func() {}).Name("build")

		// Create mock environment with --help
		env := core.NewExecuteEnv([]string{"targ", "--help"})

		// Create new state and register target
		state := core.NewRegistryState()
		state.RegisterTarget(target)

		// Execute with BinaryMode=false (targ CLI mode)
		err := state.ExecuteWithResolution(env, core.RunOptions{
			AllowDefault: true,
			BinaryMode:   false,
		})

		g.Expect(err).NotTo(HaveOccurred())

		output := env.Output()

		// In targ mode, should show targ-only flags
		g.Expect(output).To(ContainSubstring("--timeout"),
			"targ mode should show --timeout flag")
		g.Expect(output).To(ContainSubstring("--parallel"),
			"targ mode should show --parallel flag")
	})
}

func TestPrintUsageWithExamples(t *testing.T) {
	t.Parallel()

	t.Run("UserExamplesShownInRootHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target1 := core.Targ(func() {}).Name("build")
		target2 := core.Targ(func() {}).Name("test")

		env := core.NewExecuteEnv([]string{"app", "--help"})

		state := core.NewRegistryState()
		state.RegisterTarget(target1, target2)

		err := state.ExecuteWithResolution(env, core.RunOptions{
			AllowDefault: true,
			BinaryMode:   true,
			Examples: []core.Example{
				{Title: "Build and test", Code: "app build test"},
			},
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(env.Output()).To(ContainSubstring("Build and test"))
	})

	t.Run("RepoURLShownWhenNoMoreInfoText", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target1 := core.Targ(func() {}).Name("build")
		target2 := core.Targ(func() {}).Name("test")

		env := core.NewExecuteEnv([]string{"app", "--help"})

		state := core.NewRegistryState()
		state.RegisterTarget(target1, target2)

		err := state.ExecuteWithResolution(env, core.RunOptions{
			AllowDefault: true,
			BinaryMode:   true,
			RepoURL:      "https://example.com/repo",
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(env.Output()).To(ContainSubstring("https://example.com/repo"))
	})
}
