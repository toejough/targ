# Issue #47 — a command builder that can set a subprocess's environment

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** `targ.Cmd("golangci-lint", "run").Env("GOLANGCI_LINT_CACHE", dir).Run(ctx)` — a
per-invocation environment override for a subprocess, which targ cannot express today.

**Architecture:** A `Command` value in `internal/core` holds a name, args, and an ordered list of
`K=V` overrides. Its three terminals (`Run`, `RunV`, `Output`) each take a `context.Context`, so
every one routes through `parallelShellEnv` and cannot reproduce the printer-bypass defect of the
non-context helpers (#49). `internal/sh` gains one `envv []string` parameter on the three functions
that construct an `exec.Cmd`; `nil` means inherit, which is what every existing caller passes. The
three context-aware public helpers are then reimplemented as one-line delegations to the builder,
so there is exactly one construction path for a ctx-aware subprocess.

**Tech Stack:** Go 1.25.5, gomega assertions, rapid property tests, `targ` as its own build tool.

**Issue:** <https://github.com/toejough/targ/issues/47>. Read it first — it carries the rejected
alternatives (a fifth `RunContextWith` variant; env on `ShellEnv`; `os.Setenv` in `dev/targets.go`)
and the reason each was rejected.

**Branch:** `issue-47-cmd-env-builder`, off `main` at `376dad6`.

## Global Constraints

- **Gate is `targ check-full`.** Never use `check-for-fail` (stops at first error, causes
  whack-a-mole). Never use bare `go test` as final validation — it misses lint, coverage
  thresholds, and declaration ordering.
- **Baseline, measured 2026-07-28 at `376dad6`** (the branch point) by running the real gate:

  ```text
    PASS      check-coverage-for-fail  (20.598s)
    PASS      check-uncommitted        (13ms)
    PASS      reorder-decls-check      (2.241s)
    PASS      lint-fast                (1.395s)
    PASS      lint-full                (2.966s)
    PASS      deadcode                 (3.647s)
    PASS      check-thin-api           (2ms)
    PASS      check-nils-for-fail      (18.619s)
    PASS      test-integration         (3.768s)

  PASS:9
  All checks passed!
  ```

  `git status` clean after. Every task's gate is measured against this.
- **Commit per task; never push.** Joe's disposition, 2026-07-28: local commits on
  `issue-47-cmd-env-builder` are authorized as each task completes. **Nothing is pushed, merged, or
  rebased onto `main` without asking him first.** This overrides any skill default that would push.
- **Run every `targ` command from `/Users/joe/repos/personal/targ`.** The harness resets shell cwd
  between calls, and a targ run in the wrong checkout can drift `go.mod`. Use an explicit `cd` or
  an absolute path every time.
- **Every task runs its gate AFTER its commit.** `check-full` includes a `check-uncommitted` leg,
  so 9/9 is unreachable while a task's own changes sit in the working tree — a pre-commit run
  reports `PASS:8 FAIL:1` on correct work and reads as a failure that is not one.
- **`exec.Cmd.Env == nil` means inherit.** `environ()` MUST return `nil` when no overrides are
  declared. This is what makes the convergence in Task 4 a bit-for-bit no-op for every existing
  caller, and it is the single most important line in this plan.
- **Never strip a child's environment to `[]string{}`.** A negative control filters the one
  variable out of `os.Environ()`. Stripping breaks `HOME`/`XDG`/`GOCOVERDIR` propagation and has
  previously caused a ~50% coverage-gate flake.
- **Declaration ordering is enforced** by `go-reorder`: types, then vars, then funcs, alphabetical
  within each group. Insert new declarations alphabetically, not "near related code". Run
  `targ reorder-decls` to auto-fix.
- **No new package-level mutable state.** Use dependency injection.
- **Per-function coverage threshold is 80%** for every exported function. `Cmd`, `Env`, `Run`,
  `RunV`, and `Output` each need their own test or `check-coverage-for-fail` fails.
- **TDD red step is mandatory.** Write the test, run it, confirm it FAILS with the expected error
  (compilation error or assertion failure). Never write test and implementation together.
- **errcheck is enabled repo-wide.** Every `fmt.Fprint*` to a writer discards its return with
  `_, _ =`.
- **mnd is enabled.** Bare numeric literals other than 0 and 1 in non-test code need a named
  constant or `//nolint:mnd` with a reason.
- **No pre-existing failures accepted.** If `check-full` fails it gets fixed, even if the failure
  predates this work.
- **Parallel tests must not share mutable state.** Each subtest owns its own data. Never fix a race
  by removing `t.Parallel()`.
- **Commit trailer is `AI-Used: [claude]`** and the body carries `Refs #47`. This repo does not use
  `Co-Authored-By`.
- **One gotcha:** a stale `golangci-lint` result cache produces phantom `lint-full` failures citing
  paths that no longer exist. Run `golangci-lint cache clean` and re-run before treating one as
  real.

## File Structure

| File | Responsibility | Change |
| --- | --- | --- |
| `internal/sh/context.go` | `exec.Cmd` construction for ctx-aware runs | Add `envv []string` to `OutputContext`, `RunContextV`, `RunContextWithIO`; assign `cmd.Env` |
| `internal/sh/sh_test.go` | Shell-layer behavior | Task 1 seam tests |
| `internal/core/cmd.go` | **NEW.** The `Command` builder and its three terminals | Create |
| `internal/core/cmd_test.go` | **NEW.** Builder behavior | Create |
| `internal/core/target.go:612-636` | `core.RunContext`, `core.RunContextV` | Reimplement as builder delegations |
| `internal/core/target.go:1104` | Shell-target run | Pass `nil` envv |
| `internal/core/command.go:2045` | Shell-target run | Pass `nil` envv |
| `internal/core/git.go:205` | Git output helper | Pass `nil` envv |
| `targ.go` | Public API | Add `Command` alias + `Cmd`; repoint `OutputContext` |
| `README.md`, `docs/specs/*.md` | User docs and living specs | Task 5 sync |

`internal/core/command.go` already exists and is 2300 lines — the new file is `cmd.go`, not
`command.go`.

## Interfaces produced by this plan

- `internalsh.RunContextWithIO(ctx, env *ShellEnv, name string, args []string, envv []string) error`
- `internalsh.RunContextV(ctx, env *ShellEnv, name string, args []string, envv []string) error`
- `internalsh.OutputContext(ctx, name string, args []string, stdin io.Reader, envv []string) (string, error)`
  — in all three, `envv == nil` means inherit the parent environment.
- `core.Command` — struct with unexported fields `args []string`, `env []string`, `name string`.
- `core.Cmd(name string, args ...string) *Command`
- `(*core.Command).Env(key, value string) *Command`
- `(*core.Command).Output(ctx context.Context) (string, error)`
- `(*core.Command).Run(ctx context.Context) error`
- `(*core.Command).RunV(ctx context.Context) error`
- `core.OutputContext(ctx context.Context, name string, args ...string) (string, error)` — new
  package-level shim so the root wrapper stays 1:1, matching `core.RunContext`'s shape.
- `targ.Command = core.Command` (type alias), `targ.Cmd(name string, args ...string) *Command`

## Doc-surface disposition

Enumeration grep run 2026-07-28 over `README.md`, `docs/specs/`, `docs/archive/`, `CLAUDE.md` for
`targ.Run`, `targ.Output`, `RunContext`, `RunV`, `RunContextWithIO`, `OutputContext`:

| File / line | Disposition | Reason |
| --- | --- | --- |
| `README.md:17` | UPDATE | Feature-table row lists the run helpers; add the builder |
| `README.md:415-428` | UPDATE | "Run commands with `targ.Run` and friends" enumerates all six; add `targ.Cmd` + an env example |
| `docs/specs/architecture.md:56` | UPDATE | Describes `internal/sh` naming `RunContextWithIO`/`OutputContext`; signatures gain `envv` |
| `docs/specs/implementation.md:10` | UPDATE | "Key exports" list; add `Cmd()` |
| `docs/specs/implementation.md:157-158` | UPDATE | `internal/sh` purpose + key-function list; `envv` param |
| `docs/specs/tests.md:108,110` | UPDATE | Properties and test names for RunContext printer routing; add new test names |
| `docs/specs/tests.md:314` | UPDATE | Given/When/Then names `RunContextWithIO`; signature changes |
| `docs/specs/tests.md:332` | KEEP | IMPL-19 coverage note stays accurate |
| `docs/archive/requirements.md:514`, `docs/archive/issues.md:174`, `docs/archive/architecture.md:498,500,783` | N/A | Historical archive, and these refer to a long-superseded `targ.Run()` *entry-point* concept, not the shell helper |
| `CLAUDE.md` | KEEP | Does not reference the run helpers at all — nothing to go stale |

---

## Task 1: `internal/sh` accepts a per-invocation environment

The lowest layer first: the three functions that build an `exec.Cmd` learn to set `cmd.Env`. No
caller uses it yet — every existing call site passes `nil` and must keep behaving identically.

**Files:**
- Modify: `internal/sh/context.go:12-50` (`OutputContext`), `:53-61` (`RunContextV`), `:64-76`
  (`RunContextWithIO`)
- Modify (callers, `nil` only): `internal/core/target.go:615`, `internal/core/target.go:629`,
  `internal/core/target.go:1104`, `internal/core/command.go:2045`, `internal/core/git.go:205`,
  `targ.go:227`, **`internal/sh/sh_test.go:45`, `internal/sh/sh_test.go:69`**
- Test: `internal/sh/sh_test.go`

The two `sh_test.go` call sites are inside `TestProperty_ForegroundProcessGroup` and use the
current 4-arg signature. They are easy to miss because that file is also where the new test goes —
miss them and `go vet ./...` fails with `not enough arguments in call to internal.RunContextWithIO`.

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the three `envv []string` signatures listed under "Interfaces produced by this plan".

- [ ] **Step 1: Write the failing test**

Add to `internal/sh/sh_test.go`, in alphabetical position among the `TestProperty_*` functions:

```go
// Deliberately NOT parallel at this level: t.Setenv panics when the test OR ANY
// PARENT is parallel, and one subtest below needs it. Subtests opt in individually.
func TestProperty_SubprocessEnvironment(t *testing.T) {
	t.Run("NilEnvvInheritsParentEnvironment", func(t *testing.T) {
		g := gomega.NewWithT(t)

		t.Setenv("TARG_SH_INHERIT_PROBE", "inherited")

		out, err := internal.OutputContext(
			t.Context(), "sh", []string{"-c", "printf %s \"$TARG_SH_INHERIT_PROBE\""}, nil, nil,
		)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("inherited"))
	})

	t.Run("EnvvOverridesAreVisibleToTheChild", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		envv := append(os.Environ(), "TARG_SH_OVERRIDE_PROBE=overridden")

		out, err := internal.OutputContext(
			t.Context(), "sh", []string{"-c", "printf %s \"$TARG_SH_OVERRIDE_PROBE\""}, nil, envv,
		)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("overridden"))
	})

	t.Run("EnvvDoesNotStripTheRestOfTheEnvironment", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		envv := append(os.Environ(), "TARG_SH_UNRELATED_PROBE=set")

		out, err := internal.OutputContext(
			t.Context(), "sh", []string{"-c", "printf %s \"$PATH\""}, nil, envv,
		)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal(os.Getenv("PATH")))
	})
}
```

**Why the outer function is not parallel.** `testing.(*T).checkParallel` walks the *entire parent
chain*, not just the current test — a `t.Setenv` anywhere under a parallel ancestor panics with
`testing: test using t.Setenv, t.Chdir, or cryptotest.SetGlobalRandom can not use t.Parallel`, and
that panic takes down the whole package's test binary. Marking only the Setenv subtest serial is
not enough. The two subtests that do not use `t.Setenv` still call `t.Parallel()` themselves.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/sh/ -run TestProperty_SubprocessEnvironment -v
```

Expected: FAIL — compilation error, `too many arguments in call to internal.OutputContext`.

- [ ] **Step 3: Verify the call sites, then write the implementation**

Confirm the working tree still matches this plan's line numbers before editing by line:

```bash
cd /Users/joe/repos/personal/targ && grep -rn "RunContextWithIO(\|internalsh.RunContextV(\|OutputContext(" \
  internal/core/target.go internal/core/command.go internal/core/git.go targ.go internal/sh/sh_test.go
```

Expected: eight hits — `target.go:615`, `target.go:629`, `target.go:1104`, `command.go:2045`,
`git.go:205`, `targ.go:227`, `sh_test.go:45`, `sh_test.go:69`. If the count or the line numbers
differ, the tree has drifted: re-derive the list from the grep and edit those sites instead.

In `internal/sh/context.go`, change the three signatures and assign `cmd.Env`:

```go
// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
// envv sets the child's environment; nil inherits the parent's.
func OutputContext(
	ctx context.Context,
	name string,
	args []string,
	stdin io.Reader,
	envv []string,
) (string, error) {
	//nolint:gosec // G204: targ is a build tool — running user-specified commands is its purpose
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Env = envv
	// ... rest unchanged
```

```go
// RunContextV runs a command with context support, printing it first.
// envv sets the child's environment; nil inherits the parent's.
func RunContextV(ctx context.Context, env *ShellEnv, name string, args []string, envv []string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	_, _ = fmt.Fprintln(env.Stdout, "+", FormatCommand(name, args))

	return RunContextWithIO(ctx, env, name, args, envv)
}
```

```go
// RunContextWithIO runs a command with context support and custom IO.
// envv sets the child's environment; nil inherits the parent's.
func RunContextWithIO(ctx context.Context, env *ShellEnv, name string, args []string, envv []string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	//nolint:gosec // G204: targ is a build tool — running user-specified commands is its purpose
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	cmd.Stdin = env.Stdin
	cmd.Env = envv

	return runWithContext(ctx, cmd, env.Foreground)
}
```

Then update all six existing call sites to pass `nil` as the new final argument:

- `internal/core/target.go:615` → `internalsh.RunContextWithIO(ctx, env, name, args, nil)`
- `internal/core/target.go:629` → `internalsh.RunContextV(ctx, env, name, args, nil)`
- `internal/core/target.go:1104` → `internalsh.RunContextWithIO(ctx, env, "sh", []string{"-c", cmd}, nil)`
- `internal/core/command.go:2045` → `internalsh.RunContextWithIO(ctx, nil, "sh", []string{"-c", substituted}, nil)`
- `internal/core/git.go:205` → `internalsh.OutputContext(ctx, name, args, os.Stdin, nil)`
- `targ.go:227` → `internalsh.OutputContext(ctx, name, args, os.Stdin, nil)`
- `internal/sh/sh_test.go:45` → append `, nil` to the existing `RunContextWithIO` call
- `internal/sh/sh_test.go:69` → append `, nil` to the existing `RunContextWithIO` call

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/sh/ -run TestProperty_SubprocessEnvironment -v
```

Expected: PASS, all three subtests.

- [ ] **Step 5: Reorder and commit**

```bash
cd /Users/joe/repos/personal/targ && targ reorder-decls
git add internal/sh/context.go internal/sh/sh_test.go internal/core/target.go \
        internal/core/command.go internal/core/git.go targ.go
git commit -m "feat(sh): accept a per-invocation environment for ctx-aware runs

The three functions that build an exec.Cmd take an envv parameter. nil
means inherit, which is what all six existing call sites pass, so
behavior is unchanged.

Refs #47

AI-Used: [claude]"
```

- [ ] **Step 6: Gate**

```bash
cd /Users/joe/repos/personal/targ && targ check-full
```

Expected: `PASS:9`.

---

## Task 2: the `Command` builder

**Files:**
- Create: `internal/core/cmd.go`
- Test: `internal/core/cmd_test.go`

**Interfaces:**
- Consumes: `internalsh.OutputContext`, `internalsh.RunContextV`, `internalsh.RunContextWithIO`
  with their Task 1 `envv` parameter.
- Produces: `core.Command`, `core.Cmd`, `(*Command).Env`, `(*Command).Output`, `(*Command).Run`,
  `(*Command).RunV`, `core.OutputContext`.

- [ ] **Step 1: Write the failing test**

Create `internal/core/cmd_test.go`:

```go
//go:build !windows

package core_test

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/core"
)

// echoVar returns a shell command that prints one environment variable.
func echoVar(name string) []string {
	return []string{"-c", "printf %s \"$" + name + "\""}
}

// Deliberately NOT parallel at this level — NoOverrideInheritsEverything uses
// t.Setenv, which panics under a parallel ancestor. Subtests opt in individually.
func TestCmdEnv(t *testing.T) {
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

	// Serial: t.Setenv panics in a parallel test.
	t.Run("NoOverrideInheritsEverything", func(t *testing.T) {
		g := gomega.NewWithT(t)

		t.Setenv("TARG_CMD_INHERIT", "inherited")

		out, err := core.Cmd("sh", echoVar("TARG_CMD_INHERIT")...).Output(t.Context())

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("inherited"))
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

func TestCmdRun(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	err := core.Cmd("sh", "-c", "exit 0").Env("TARG_CMD_RUN", "1").Run(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
}

func TestCmdRunV(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	err := core.Cmd("sh", "-c", "exit 0").Env("TARG_CMD_RUNV", "1").RunV(t.Context())

	g.Expect(err).ToNot(gomega.HaveOccurred())
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

	results := make([]string, builders)

	var wg sync.WaitGroup

	for i := range builders {
		wg.Add(1)

		go func() {
			defer wg.Done()

			want := "value-" + strings.Repeat("x", i)

			out, err := core.Cmd("sh", echoVar("TARG_CMD_CONCURRENT")...).
				Env("TARG_CMD_CONCURRENT", want).
				Output(t.Context())
			if err == nil {
				results[i] = out
			}
		}()
	}

	wg.Wait()

	for i := range builders {
		g.Expect(results[i]).To(gomega.Equal("value-" + strings.Repeat("x", i)))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/core/ -run 'TestCmd|TestProperty_Cmd' -v
```

Expected: FAIL — compilation error, `undefined: core.Cmd`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/core/cmd.go`:

```go
package core

import (
	"context"
	"os"

	internalsh "github.com/toejough/targ/internal/sh"
)

// Command is a subprocess invocation with optional per-invocation environment
// overrides. Build one with Cmd, then run it with Run, RunV, or Output.
//
// Every terminal takes a context, so a Command always routes through the
// parallel printer when its target is running in parallel mode.
type Command struct {
	args []string
	env  []string
	name string
}

// Cmd creates a Command for the named program with the given arguments.
//
//	targ.Cmd("golangci-lint", "run").Env("GOLANGCI_LINT_CACHE", dir).Run(ctx)
func Cmd(name string, args ...string) *Command {
	return &Command{name: name, args: args, env: nil}
}

// OutputContext executes a command and returns combined output, with context support.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return Cmd(name, args...).Output(ctx)
}

// Env adds an environment variable for this command only. It is repeatable, and
// the last value declared for a key wins. Declared variables are added to the
// inherited environment — they do not replace it.
func (c *Command) Env(key, value string) *Command {
	c.env = append(c.env, key+"="+value)

	return c
}

// Output runs the command and returns its combined output.
func (c *Command) Output(ctx context.Context) (string, error) {
	return internalsh.OutputContext(ctx, c.name, c.args, os.Stdin, c.environ())
}

// Run executes the command, streaming stdout/stderr.
// In parallel mode, output is routed through the parallel printer.
func (c *Command) Run(ctx context.Context) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextWithIO(ctx, env, c.name, c.args, c.environ())

	if pw != nil {
		pw.Flush()
	}

	return err
}

// RunV executes the command, printing it first.
// In parallel mode, output is routed through the parallel printer.
func (c *Command) RunV(ctx context.Context) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextV(ctx, env, c.name, c.args, c.environ())

	if pw != nil {
		pw.Flush()
	}

	return err
}

// environ returns the child's environment. It returns nil when no overrides are
// declared, so exec inherits the parent's environment unchanged — that is what
// keeps this builder a no-op for callers that declare no environment.
//
// os/exec uses the last value for a duplicated key, so appending the overrides
// after os.Environ() gives both override-wins and later-call-wins.
func (c *Command) environ() []string {
	if len(c.env) == 0 {
		return nil
	}

	return append(os.Environ(), c.env...)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/core/ -run 'TestCmd|TestProperty_Cmd' -v
```

Expected: PASS, all subtests and both properties.

Then confirm no race:

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/core/ -run 'TestProperty_CmdEnvIsPerInvocation' -race -count=4
```

Expected: PASS, no `DATA RACE`.

- [ ] **Step 5: Reorder and commit**

```bash
cd /Users/joe/repos/personal/targ && targ reorder-decls
git add internal/core/cmd.go internal/core/cmd_test.go
git commit -m "feat(core): add Command, a builder with per-invocation environment

Cmd(name, args...).Env(k, v) accumulates K=V overrides; environ()
returns nil when none are declared so exec inherits unchanged, and
os.Environ() plus the overrides otherwise.

Refs #47

AI-Used: [claude]"
```

- [ ] **Step 6: Gate**

```bash
cd /Users/joe/repos/personal/targ && targ check-full
```

Expected: `PASS:9`.

---

## Task 3: converge the three context-aware helpers onto the builder

One construction path for a ctx-aware subprocess. `core.RunContext` and `core.RunContextV` lose
their hand-rolled `parallelShellEnv`/flush bodies; `targ.OutputContext` stops reaching past `core`
into `internal/sh`.

**Files:**
- Modify: `internal/core/target.go:610-636` (`RunContext`, `RunContextV`)
- Modify: `targ.go:224-228` (`OutputContext`)
- Test: `internal/core/cmd_test.go` (append)

**Interfaces:**
- Consumes: everything Task 2 produced.
- Produces: no new names — the three public signatures are unchanged.

Task 3 changes no behavior, so there is no RED step to write — the TDD discipline for a refactor is
a **characterization test written first, which passes before and after**. That test is Step 1; its
green run in Step 2 is what makes the Step 4 green run meaningful. Do not skip Step 2 on the
grounds that a passing test "proves nothing" — an unrun baseline turns Step 4 into an assumption.

- [ ] **Step 1: Write the characterization test**

Append to `internal/core/cmd_test.go`. Every subtest is serial — all three use `t.Setenv`:

```go
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
```

- [ ] **Step 2: Run it against the UNREFACTORED code and record the result**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/core/ -run TestConvergedHelpersInheritEnvironment -v
```

Expected: PASS, all three subtests. `core.OutputContext` exists because Task 2 created it, and the
other two exercise the current hand-rolled implementations. **Write the observed result into the
commit message in Step 5.** If any subtest fails here, stop — the invariant is not what this plan
claims and the refactor must not proceed until that is understood.

- [ ] **Step 3: Write the implementation**

Replace the bodies in `internal/core/target.go`:

```go
// RunContext executes a command with context support, routing output through
// the parallel printer when running in parallel mode.
func RunContext(ctx context.Context, name string, args ...string) error {
	return Cmd(name, args...).Run(ctx)
}

// RunContextV executes a command, prints it first, with context support.
// Routes output through the parallel printer when in parallel mode.
func RunContextV(ctx context.Context, name string, args ...string) error {
	return Cmd(name, args...).RunV(ctx)
}
```

Replace in `targ.go`:

```go
// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return core.OutputContext(ctx, name, args...)
}
```

If `os` becomes unused in `targ.go` after this, remove the import.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/joe/repos/personal/targ && go test ./internal/core/ -run 'TestConvergedHelpers|TestRunContext|TestParallelOutput' -v
```

Expected: PASS. `TestRunContextInParallelMode` and `TestRunContextVInParallelMode` are the existing
printer-routing regressions — they must still pass, which is the proof the convergence preserved
parallel-printer behavior.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ && targ reorder-decls
git add internal/core/target.go internal/core/cmd_test.go targ.go
git commit -m "refactor(core): converge the ctx-aware run helpers onto Command

RunContext, RunContextV and OutputContext are now one-line delegations
to the builder, so exactly one path constructs a ctx-aware subprocess.
Signatures and behavior are unchanged.

Characterization test run against the unrefactored code first:
  go test ./internal/core/ -run TestConvergedHelpersInheritEnvironment
  ok  github.com/toejough/targ/internal/core  <PASTE REAL TIMING>
and again after the refactor, same result.

Refs #47

AI-Used: [claude]"
```

- [ ] **Step 6: Gate**

```bash
cd /Users/joe/repos/personal/targ && targ check-full
```

Expected: `PASS:9`.

---

## Task 4: expose the builder as public API

**Files:**
- Modify: `targ.go` (type alias block ~:51-99, function block)
- Test: `test/execution_properties_test.go`

**Interfaces:**
- Consumes: `core.Command`, `core.Cmd`.
- Produces: `targ.Command`, `targ.Cmd`.

- [ ] **Step 1: Write the failing test**

Append to `test/execution_properties_test.go`. **That file dot-imports gomega** (`. "github.com/onsi/gomega"`, line 19), so use the bare identifiers `NewWithT`/`HaveOccurred`/`Equal` — the
qualified `gomega.NewWithT` form does not resolve there and fails with `undefined: gomega`:

```go
func TestProperty_CmdBuilderPublicAPI(t *testing.T) {
	t.Parallel()

	t.Run("EnvOverrideReachesTheChild", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		out, err := targ.Cmd("sh", "-c", `printf %s "$TARG_PUBLIC_PROBE"`).
			Env("TARG_PUBLIC_PROBE", "public").
			Output(t.Context())

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(out).To(Equal("public"))
	})

	t.Run("InheritedEnvironmentSurvives", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		out, err := targ.Cmd("sh", "-c", `printf %s "$PATH"`).
			Env("TARG_PUBLIC_UNRELATED", "x").
			Output(t.Context())

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(out).To(Equal(os.Getenv("PATH")))
	})
}
```

Add `"os"` to that file's imports if absent.

Note the contrast with `internal/core/cmd_test.go`, which imports gomega qualified. Match whichever
convention the file you are editing already uses; do not change a file's import style.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/joe/repos/personal/targ && go test ./test/ -run TestProperty_CmdBuilderPublicAPI -v
```

Expected: FAIL — compilation error, `undefined: targ.Cmd`. If instead you see
`undefined: gomega`, the snippet was pasted with the qualified import form; fix that first, because
that error persists after the feature exists and would mask the real red.

- [ ] **Step 3: Write minimal implementation**

In `targ.go`, add the alias in alphabetical position within the existing alias block (between
`ChangeSet` at :51 and `DepGroup` at :54):

```go
// Command is a subprocess invocation with per-invocation environment overrides.
type Command = core.Command
```

And the constructor in alphabetical position among the functions:

```go
// Cmd creates a Command for the named program with the given arguments.
// Set per-invocation environment variables with Env, then run it with
// Run, RunV, or Output.
//
//	err := targ.Cmd("golangci-lint", "run").
//	    Env("GOLANGCI_LINT_CACHE", dir).
//	    Run(ctx)
func Cmd(name string, args ...string) *Command {
	return core.Cmd(name, args...)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/joe/repos/personal/targ && go test ./test/ -run TestProperty_CmdBuilderPublicAPI -v
```

Expected: PASS, both subtests.

- [ ] **Step 5: Commit**

```bash
cd /Users/joe/repos/personal/targ && targ reorder-decls
git add targ.go test/execution_properties_test.go
git commit -m "feat: expose targ.Cmd, a command builder with environment overrides

Refs #47

AI-Used: [claude]"
```

- [ ] **Step 6: Gate**

```bash
cd /Users/joe/repos/personal/targ && targ check-full
```

Expected: `PASS:9`. `check-thin-api` is the one to watch: `targ.Cmd` must read as a thin
delegation. If it objects, the fallback is to keep the wrapper byte-identical in shape to
`targ.Targ` (`return core.Cmd(name, args...)`) — do not add logic to `targ.go` to satisfy it.

---

## Task 5: docs

**Files:**
- Modify: `README.md:17`, `README.md:415-428`
- Modify: `docs/specs/architecture.md:56`
- Modify: `docs/specs/implementation.md:10,157-158`
- Modify: `docs/specs/tests.md:108,110,314`

**Interfaces:**
- Consumes: the final public API from Task 4.
- Produces: nothing.

- [ ] **Step 1: Update `README.md`**

At :17, the feature-table row becomes:

```markdown
| Run shell commands  | `targ.Run("go", "build")`, `targ.RunContext(ctx, ...)`, or `targ.Cmd(...)` |
```

In the block at :415-428, after the context-aware examples, add:

```markdown
Set environment variables for a single command with the builder:

```go
err := targ.Cmd("golangci-lint", "run").
    Env("GOLANGCI_LINT_CACHE", cacheDir).
    Run(ctx)

out, err := targ.Cmd("go", "env", "GOMOD").Env("GOFLAGS", "-mod=mod").Output(ctx)
```

Declared variables are added to the inherited environment, not a replacement for it, and the
builder's terminals always take a context.
```

- [ ] **Step 2: Update the specs**

- `docs/specs/architecture.md:56` — the sentence naming `RunContextWithIO()` and `OutputContext()`
  gains: "Both take an `envv []string`; `nil` inherits the parent environment."
- `docs/specs/implementation.md:10` — the Key exports list is **already stale**: it names `Run()`,
  `RunContext()`, and `Output()` but omits their exported siblings `RunV()`, `RunContextV()`, and
  `OutputContext()` (all present in `targ.go` at :276, :271, :226). Add `Cmd()` **and** the three
  missing ones, so the line becomes:

  ```markdown
  **Key exports:** `Targ()`, `Main()`, `Register()`, `Execute()`, `Group()`, `Cmd()`, `Run()`, `RunV()`, `RunContext()`, `RunContextV()`, `Output()`, `OutputContext()`, `Match()`, `Watch()`, `Checksum()`, `Print()`, `Printf()`
  ```
- `docs/specs/implementation.md:157-158` — same `envv` note, and add `Cmd`/`Command` to the
  `internal/core` entry's key functions.
- `docs/specs/tests.md:108` — add the property lines:
  `- Property: Cmd env overrides are additive — declared pairs reach the child, un-overridden
  inherited variables survive` and
  `- Property: Cmd env is per-invocation — concurrent builders do not observe each other`.
- `docs/specs/tests.md:110` — add `TestCmdEnv`, `TestCmdRun`, `TestCmdRunV`,
  `TestConvergedHelpersInheritEnvironment`, `TestProperty_CmdEnvIsAdditive`,
  `TestProperty_CmdEnvIsPerInvocation`, `TestProperty_CmdBuilderPublicAPI`,
  `TestProperty_SubprocessEnvironment` to the test-name list.
- `docs/specs/tests.md:314` — the Given/When/Then gains a clause: commands executed via
  `RunContextWithIO` with a nil `envv` inherit the parent environment.

- [ ] **Step 3: Verify no other doc references went stale**

The criterion is a count plus an absence, not an eyeball pass over the hits:

```bash
cd /Users/joe/repos/personal/targ
# 1. No doc still describes the old 4-arg shell signature.
grep -rn "RunContextWithIO()" README.md docs/specs/ CLAUDE.md | grep -v "envv"
# 2. Every public run helper appears in the Key exports line.
for fn in Cmd Run RunV RunContext RunContextV Output OutputContext; do
  grep -q "\`$fn()\`" docs/specs/implementation.md || echo "MISSING from Key exports: $fn"
done
```

Expected: command 1 prints nothing (every surviving mention of `RunContextWithIO` carries the
`envv` note). Command 2 prints nothing. Any output is a stale doc — fix it before committing.

- [ ] **Step 4: Commit**

```bash
cd /Users/joe/repos/personal/targ
git add README.md docs/specs/
git commit -m "docs: document targ.Cmd and the envv parameter

Refs #47

AI-Used: [claude]"
```

- [ ] **Step 5: Final gate**

```bash
cd /Users/joe/repos/personal/targ && targ check-full
```

Expected: `PASS:9`, tree clean.

---

## Acceptance

- [ ] `targ.Cmd("sh", "-c", "printf %s \"$V\"").Env("V", "set").Output(ctx)` returns `"set"`.
- [ ] The same command with an unrelated `.Env` still sees the full inherited environment —
      `$PATH` in the child equals `os.Getenv("PATH")` in the parent.
- [ ] A `Command` with no `.Env` call produces `cmd.Env == nil`, so behavior is identical to
      calling `targ.RunContext` directly. This is the convergence no-op.
- [ ] Two `.Env` calls for one key: the later value wins.
- [ ] 8 concurrent builders with distinct values each observe only their own, under
      `-race -count=4` with zero `DATA RACE` reports.
- [ ] `TestRunContextInParallelMode` and `TestRunContextVInParallelMode` still pass after Task 3 —
      the convergence preserved parallel-printer routing.
- [ ] `go vet ./...` is clean, including `internal/sh/sh_test.go`'s two migrated call sites.
- [ ] `targ check-full` reports `PASS:9`, tree clean, after Task 5.

## Out of scope

Named here so a reviewer does not read their absence as an oversight:

- `.Dir()`, `.Stdin()`, and a replace-the-whole-environment mode — YAGNI until a caller needs one.
- Target-level `targ.Targ(fn).Env(k, v)` — a plausible follow-on, filed separately if wanted.
- Env support on the non-context `Run`/`RunV`/`Output` — they cannot reach the parallel printer
  (#49), and giving them new capability would deepen that defect.
- Fixing #49 itself.
- `internal/core/git.go:205` keeps calling `internalsh.OutputContext` directly. It needs neither
  env nor printer routing.
- Issue #46's actual fix. It consumes this API; it is not part of this plan.
