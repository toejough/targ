//go:build !windows

package core_test

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

func TestCmdEnv(t *testing.T) {
	t.Parallel()

	t.Run("DeclaredVariableIsVisibleToTheChild", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		out, err := core.Cmd("sh", echoVar("TARG_CMD_PROBE")...).
			Env("TARG_CMD_PROBE", "declared").
			Output(t.Context())

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("declared"))
	})

	t.Run("PathSurvivesAnOverride", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		out, err := core.Cmd("sh", echoVar("PATH")...).
			Env("TARG_CMD_UNRELATED", "x").
			Output(t.Context())

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal(os.Getenv("PATH")))
	})

	t.Run("LastDeclarationWins", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		out, err := core.Cmd("sh", echoVar("TARG_CMD_DUP")...).
			Env("TARG_CMD_DUP", "first").
			Env("TARG_CMD_DUP", "second").
			Output(t.Context())

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("second"))
	})
}

// TestCmdInheritsWhenNoOverrideDeclared is serial and standalone: t.Setenv panics
// under a parallel ancestor, and tparallel requires a function with parallel
// subtests to be parallel itself. This is the convergence no-op proof — with no
// Env call, cmd.Env stays nil and the child inherits everything.
func TestCmdInheritsWhenNoOverrideDeclared(t *testing.T) {
	g := gomega.NewWithT(t)

	t.Setenv("TARG_CMD_INHERIT", "inherited")

	out, err := core.Cmd("sh", echoVar("TARG_CMD_INHERIT")...).Output(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(out).To(gomega.Equal("inherited"))
}

// TestCmdRun asserts through the command's own exit status that the declared
// variable actually reached the child. A body of "exit 0" would pass even if
// Run dropped environ() entirely, which is the regression worth catching:
// Run and RunV are the terminals RunContext/RunContextV delegate to, so they
// carry every shell target and every parallel dep run.
func TestCmdRun(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	err := core.Cmd("sh", "-c", `test "$TARG_CMD_RUN" = declared`).
		Env("TARG_CMD_RUN", "declared").
		Run(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
}

// TestCmdRunInheritsAndOverrides pins that Run's environment is additive, not a
// replacement — PATH must survive alongside a declared override.
func TestCmdRunInheritsAndOverrides(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	err := core.Cmd("sh", "-c", `test -n "$PATH" && test "$TARG_CMD_RUN_BOTH" = x`).
		Env("TARG_CMD_RUN_BOTH", "x").
		Run(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
}

func TestCmdRunV(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	err := core.Cmd("sh", "-c", `test "$TARG_CMD_RUNV" = declared`).
		Env("TARG_CMD_RUNV", "declared").
		RunV(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
}

// TestConvergedHelpersInheritEnvironment pins the invariant that Task 3's
// convergence must preserve: the ctx-aware helpers inherit the parent's
// environment. Every subtest is serial because all three use t.Setenv.
func TestConvergedHelpersInheritEnvironment(t *testing.T) {
	t.Run("RunContextInheritsParentEnvironment", func(t *testing.T) {
		g := gomega.NewWithT(t)

		t.Setenv("TARG_CONVERGE_RUN", "inherited")

		err := core.RunContext(t.Context(), "sh", "-c", `test "$TARG_CONVERGE_RUN" = inherited`)

		g.Expect(err).ToNot(gomega.HaveOccurred())
	})

	t.Run("RunContextVInheritsParentEnvironment", func(t *testing.T) {
		g := gomega.NewWithT(t)

		t.Setenv("TARG_CONVERGE_RUNV", "inherited")

		err := core.RunContextV(t.Context(), "sh", "-c", `test "$TARG_CONVERGE_RUNV" = inherited`)

		g.Expect(err).ToNot(gomega.HaveOccurred())
	})

	t.Run("OutputContextInheritsParentEnvironment", func(t *testing.T) {
		g := gomega.NewWithT(t)

		t.Setenv("TARG_CONVERGE_OUT", "inherited")

		out, err := core.OutputContext(t.Context(), "sh", echoVar("TARG_CONVERGE_OUT")...)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("inherited"))
	})
}

func TestOutputContext(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	out, err := core.OutputContext(t.Context(), "sh", "-c", "printf hello")

	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(out).To(gomega.Equal("hello"))
}

func TestProperty_CmdEnvIsAdditive(t *testing.T) {
	t.Parallel()

	// rapid v1.2.0's *rapid.T does have Context(), but it is cancelled when the
	// property function returns. Capture the test's context once instead, so every
	// draw runs against the same lifetime.
	ctx := t.Context()
	wantPath := os.Getenv("PATH")

	rapid.Check(t, func(rt *rapid.T) {
		suffix := rapid.StringMatching(`[A-Z]{1,8}`).Draw(rt, "suffix")
		value := rapid.StringMatching(`[a-zA-Z0-9]{0,16}`).Draw(rt, "value")
		key := "TARG_PROP_" + suffix

		// The declared pair reaches the child.
		got, err := core.Cmd("sh", echoVar(key)...).Env(key, value).Output(ctx)
		if err != nil {
			rt.Fatalf("declared pair: %v", err)
		}

		if got != value {
			rt.Fatalf("declared pair: got %q want %q", got, value)
		}

		// An un-overridden inherited variable still reaches the child.
		gotPath, err := core.Cmd("sh", echoVar("PATH")...).Env(key, value).Output(ctx)
		if err != nil {
			rt.Fatalf("inherited PATH: %v", err)
		}

		if gotPath != wantPath {
			rt.Fatalf("inherited PATH: got %q want %q", gotPath, wantPath)
		}
	})
}

func TestProperty_CmdEnvIsPerInvocation(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	const builders = 8

	ctx := t.Context()
	results := make([]string, builders)

	var wg sync.WaitGroup

	for i := range builders {
		wg.Go(func() {
			want := fmt.Sprintf("value-%d", i)

			out, err := core.Cmd("sh", echoVar("TARG_CMD_CONCURRENT")...).
				Env("TARG_CMD_CONCURRENT", want).
				Output(ctx)
			if err != nil {
				results[i] = "error: " + err.Error()

				return
			}

			results[i] = out
		})
	}

	wg.Wait()

	for i := range builders {
		g.Expect(results[i]).To(gomega.Equal(fmt.Sprintf("value-%d", i)))
	}
}

// echoVar returns shell args that print one environment variable and nothing else.
func echoVar(name string) []string {
	return []string{"-c", "printf %s \"$" + name + "\""}
}
