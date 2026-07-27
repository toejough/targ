# Issue #42 — reject an unresolvable argument before anything runs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** No target fires its dependencies or its command for an invocation that targ is going to
reject anyway.

**Architecture:** `opts.HelpOnly` already parses a target's args and hands back its leftovers
without executing, at every target kind. This adds `ResolveOnly`, a non-printing sibling of that
flag, and uses it twice: the serial dispatcher walks the whole chain in resolve mode before
executing any of it, and the parallel dispatcher resolves each default-mode unit before any
goroutine starts. Separately, `executeShellCommand` stops discarding the `explicit` parameter it
already receives, which is the original defect.

**Tech Stack:** Go 1.25.5, gomega assertions, rapid property tests, `targ` as its own build tool.

**Design doc:** `docs/plans/2026-07-27-issue-42-unknown-arg-prevalidation-design.md` (commit
`16d8a37`). Read it first — it carries the measured behavior tables and the two rejected
alternatives.

**Branch:** `issue-42-unknown-arg-prevalidation`, off `main` at `55c6e48`.

## Global Constraints

- **Gate is `targ check-full`.** Baseline measured **green, PASS 9/9** at `55c6e48`, tree clean
  after. Never use `check-for-fail` (stops at first error, causes whack-a-mole). Never use bare
  `go test` as final validation — it misses lint, coverage thresholds, and declaration ordering.
- **Every task runs its gate AFTER its commit.** `check-full` includes a `check-uncommitted` leg,
  so 9/9 is unreachable while a task's own changes sit in the working tree — a pre-commit run
  reports `PASS:8 FAIL:1` on correct work and reads as a failure that is not one. If the
  post-commit gate finds a real failure, fix it and amend; these commits are local and unpushed.
- **Lint is measured by `targ lint-full`.** A task's verification step runs the gates this section
  declares — `go build && go vet && go test` is never a proxy for the configured golangci-lint run.
- **errcheck is enabled repo-wide.** Every `fmt.Fprint*` to an errOut/stderr writer in this repo
  discards its return with `_, _ =`. `go vet` does not catch a violation; only the project lint
  config does.
- **mnd is enabled.** Bare numeric literals other than 0 and 1 in non-test code need a named
  constant or `//nolint:mnd` with a reason.
- **Declaration ordering is enforced** by `go-reorder`: types, then vars, then funcs, alphabetical
  within each group. Insert new declarations alphabetically, not "near related code". Run
  `targ reorder-decls` to auto-fix.
- **No new package-level mutable state.** Use dependency injection.
- **Per-function coverage threshold is 80%** for every exported function.
- **TDD red step is mandatory.** Write the test, run it, confirm it FAILS with the expected error
  (compilation error or assertion failure). Never write test and implementation together.
- **No pre-existing failures accepted.** If `check-full` fails it gets fixed, even if the failure
  predates this work.
- **Parallel tests must not share mutable state.** Each subtest owns its own data. Never fix a race
  by removing `t.Parallel()`.
- **Commit trailer is `AI-Used: [claude]`** and the body carries `Refs #42`. This repo does not use
  `Co-Authored-By`.
- **One gotcha:** a stale `golangci-lint` result cache produces phantom `lint-full` failures citing
  paths that no longer exist. Run `golangci-lint cache clean` and re-run before treating one as
  real.

## File Structure

| File | Responsibility | Change |
| --- | --- | --- |
| `internal/core/types.go` | `RunOptions` definition | Add one `ResolveOnly` field |
| `internal/core/command.go` | Per-target-kind execution and its early returns | Honor `ResolveOnly` at the function and deps-only seams, plus a separate one in the shell path below its validation; honor `explicit` in the shell path |
| `internal/core/run_env.go` | Dispatch — serial chain and parallel fan-out | Two-pass chain walk; upfront unit resolution; delete the post-hoc check |
| `test/shell_properties_test.go` | Shell-target behavior | Task 1 regression test |
| `test/execution_properties_test.go` | Execution and dispatch behavior | Tasks 2, 3, 4 regression tests |
| `docs/specs/*.md`, `README.md` | Living specs and user docs | Task 5 sync |

## Interfaces produced by this plan

- `RunOptions.ResolveOnly bool` — set true to resolve args to targets and return leftovers without
  executing anything. Mutually exclusive with `HelpOnly` in intent; `HelpOnly` additionally prints.
- `(*runExecutor).walkRoots(resolveOnly bool) error` — runs the serial chain in one mode.
- `(*runExecutor).executeGlobPattern(name string, opts RunOptions) error` — signature gains `opts`
  so the resolve mode propagates into glob-expanded targets.

---

## Task 1: shell targets honor `explicit`

This is the original defect and it stands alone: it fixes both single-target rows of the design
doc's measured table without any of the pre-validation machinery.

**Files:**
- Modify: `internal/core/command.go:1021-1029` (signature), `internal/core/command.go:1046-1050`
  (insertion point)
- Test: `test/shell_properties_test.go`, inside `TestProperty_ShellCommandDeps` (spans lines
  219-279)

**Interfaces:**
- Consumes: `errUnknownCommand` (`internal/core/parse.go:26`), `parsed.remaining` from
  `parseShellCommandArgs`
- Produces: nothing later tasks import; behavior only

- [ ] **Step 1: Write the failing test**

Add this subtest at the end of `TestProperty_ShellCommandDeps` in
`test/shell_properties_test.go`, after `DepErrorPreventsShellCommand`:

```go
	// Issue #42: a bare unknown arg must be rejected before the dep chain and
	// the shell command run. executeShellCommand received `explicit` and threw
	// it away, so the whole invocation happened and only then errored.
	t.Run("UnknownBareArgRunsNeitherDepNorShellCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false
		dep := targ.Targ(func() { depRan = true }).Name("dep")

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true

			return nil
		}

		main := targ.Targ("echo hello").Name("main").Deps(dep)

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "bogus"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
			main,
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
		g.Expect(depRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("unknown command: bogus"))
	})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `targ test`

Expected: FAIL in `TestProperty_ShellCommandDeps/UnknownBareArgRunsNeitherDepNorShellCommand`,
with `shellRan` and `depRan` both true — the command and its dependency ran. This is the defect
reproducing inside the suite.

If it PASSES, stop: the premise is wrong and the rest of this plan needs rechecking.

- [ ] **Step 3: Write the minimal implementation**

In `internal/core/command.go`, change the `executeShellCommand` signature (currently line 1027) to
take the parameter instead of discarding it:

```go
func executeShellCommand(
	ctx context.Context,
	args []string,
	node *commandNode,
	_ []commandInstance, // parents - not used for shell commands
	_ map[string]bool, // visited - not used for shell commands
	explicit bool,
	opts RunOptions,
) ([]string, error) {
```

Then, immediately after the existing unknown-flag check (currently lines 1047-1050) and **before**
the `runNodeDeps` call, insert:

```go
	// Mirror trySubcommandOrUnknown (parse.go:150-154): in default mode an
	// unresolvable leftover is an error; in multi-root mode it is a chaining
	// candidate the dispatcher will try as the next command.
	if !explicit && len(parsed.remaining) > 0 {
		return nil, fmt.Errorf("%w: %s", errUnknownCommand, parsed.remaining[0])
	}
```

`fmt` is already imported in this file.

- [ ] **Step 4: Run the test and verify it passes**

Run: `targ test`

Expected: PASS, whole suite green.

- [ ] **Step 5: Prove the test discriminates**

Temporarily revert only the `if !explicit` block (leave the signature change in place) and run
`targ test` again.

Expected: the new subtest FAILS. If it still passes, the assertion is not discriminating and must
be strengthened before proceeding. Restore the block afterward.

- [ ] **Step 6: Commit**

```bash
git add internal/core/command.go test/shell_properties_test.go
git commit -F - <<'EOF'
fix(core): shell targets reject an unknown bare arg before running

executeShellCommand took `explicit` as `_` and ignored it, while
executeFunctionWithParents honored it from the same call site
(command.go:164 and :168 pass the same value). So on a single-target
module a shell target ran its dependency chain and its command and only
then reported the unknown argument, while a function target rejected the
same invocation without running anything.

Mirror trySubcommandOrUnknown (parse.go:150-154): with explicit false an
unresolvable leftover is an error; with it true the leftover is still
handed back for chaining, so multi-root behavior is untouched.

Refs #42

AI-Used: [claude]
EOF
```

- [ ] **Step 7: Run the full gate, after the commit**

Run: `targ check-full`

Expected: PASS 9/9.

The gate runs **after** the commit, not before: `check-full` includes a `check-uncommitted` leg,
so 9/9 is unreachable while this task's own changes sit in the working tree — a pre-commit run
reports `PASS:8 FAIL:1` on correct work, which reads as a failure and is not one. Every task in
this plan orders its gate this way.

Two notes on this specific gate run. `unparam` may now observe `explicit` on `executeShellCommand`
— it has a single caller passing a non-constant value, from `command.go:168`, so no finding is
expected. And if lint reports a phantom failure citing a path that does not exist, run
`golangci-lint cache clean` and re-run before treating it as real.

If the gate finds a real failure, fix it and amend; the commit is local and unpushed.

---

## Task 2: `ResolveOnly` — resolve args without executing

**Files:**
- Modify: `internal/core/types.go:41` area (`RunOptions`), `internal/core/command.go:896`, `:944`,
  `:1037`
- Test: `test/execution_properties_test.go`, inside `TestProperty_Execution` (starts line 73)

**Interfaces:**
- Produces: `RunOptions.ResolveOnly bool`, consumed by Tasks 3 and 4.

- [ ] **Step 1: Write the failing test**

Add this subtest inside `TestProperty_Execution` in `test/execution_properties_test.go`:

```go
	// Issue #42: ResolveOnly parses args and returns leftovers without running
	// anything - the primitive the chain pre-pass is built on.
	t.Run("ResolveOnlyRunsNeitherTargetNorDeps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		depRan := false
		dep := targ.Targ(func() { depRan = true }).Name("dep")

		targetRan := false
		target := targ.Targ(func() { targetRan = true }).Name("cmd").Deps(dep)

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "cmd"},
			targ.RunOptions{ResolveOnly: true, AllowDefault: true},
			target,
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(targetRan).To(BeFalse())
		g.Expect(depRan).To(BeFalse())
	})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `targ test`

Expected: FAIL — a compilation error, `unknown field ResolveOnly in struct literal of type
targ.RunOptions`. A compile failure is a valid red step.

- [ ] **Step 3: Add the field**

In `internal/core/types.go`, inside `RunOptions`, add the field directly after `HelpOnly`:

```go
	HelpOnly          bool // Internal: set when --help is detected, skips execution
	ResolveOnly       bool // Internal: resolve args to targets without running them
	HasDefault        bool // Internal: set when the sole root is reachable bare; drives default-root help rendering
```

- [ ] **Step 4: Honor it at the seams — two shared, one separate**

Two of the existing `if opts.HelpOnly` early returns simply gain `ResolveOnly`. The shell path
does **not**; it gets its own return in a different position, for the reason spelled out below.
Do **not** touch the help printing at `command.go:157-161` — that stays `HelpOnly`-only, and it is
the entire difference between the two modes.

In `executeDepsOnlyTarget` (currently `command.go:895-897`):

```go
	// Skip execution in help-only and resolve-only mode
	if opts.HelpOnly || opts.ResolveOnly {
		return args, nil
	}
```

In `executeFunctionWithParents` (currently `command.go:943-946`):

```go
	// In help-only and resolve-only mode, skip validation and execution
	if opts.HelpOnly || opts.ResolveOnly {
		return result.remaining, nil
	}
```

**`executeShellCommand` is the exception — do NOT add `ResolveOnly` to its `HelpOnly` return.**
That return sits at `command.go:1037`, which is *above* Task 1's explicit check at `:1047`. Adding
`ResolveOnly` there would make the resolve pass hand `bogus` straight back as a leftover without
ever applying the explicit rule, and Task 3's chain walk would then report it as an unresolvable
*command* — `Unknown command: bogus` plus a usage dump — instead of the `unknown command: bogus`
that Task 1 established and that Task 1's own test asserts. That is a measured regression, not a
hypothetical; a Gate A reviewer produced it by following the earlier version of this step.

Leave `:1037` exactly as it is:

```go
	if opts.HelpOnly {
		return parsed.remaining, nil
	}
```

and add a **separate** early return lower down, immediately after Task 1's explicit check and
**before** the `runNodeDeps` call:

```go
	// Resolve mode validates but does not execute, so this sits below the
	// unknown-flag and explicit checks rather than beside the HelpOnly return.
	if opts.ResolveOnly {
		return parsed.remaining, nil
	}
```

- [ ] **Step 5: Run the tests and verify they pass**

Run: `targ test`

Expected: PASS, including Task 1's
`TestProperty_ShellCommandDeps/UnknownBareArgRunsNeitherDepNorShellCommand`. If that test now
fails with `Unknown command: bogus` instead of `unknown command: bogus`, the `ResolveOnly` return
was placed at `:1037` instead of below the explicit check — re-read this step.

- [ ] **Step 6: Verify declaration ordering**

Run: `targ reorder-decls-check`

Expected: PASS. If it reports a move, run `targ reorder-decls` and include the result in this
task's commit rather than leaving it for a later task to trip over.

- [ ] **Step 7: Commit**

```bash
git add internal/core/types.go internal/core/command.go test/execution_properties_test.go
git commit -F - <<'EOF'
feat(core): add ResolveOnly, a non-printing sibling of HelpOnly

HelpOnly already parses a target's args and hands back its leftovers
without executing, at every target kind - function, shell, deps-only, and
group. The only thing keeping it from being a general resolve pass is that
it also prints help.

ResolveOnly reuses those seams and skips the printing, giving one
primitive: given a node and args, what is left over, without running
anything. The chain pre-pass and the parallel unit resolution both build
on it.

The shell path takes its own return placed below the unknown-flag and
explicit checks rather than beside the HelpOnly one, so resolve mode still
validates. Sharing that seam would return a bad arg as a leftover before
the explicit rule ran, and the chain walk would then misreport it as an
unknown command rather than an unknown argument.

Refs #42

AI-Used: [claude]
EOF
```

- [ ] **Step 8: Run the full gate, after the commit**

Run: `targ check-full`

Expected: PASS 9/9.

This runs **after** the commit deliberately. `check-full` includes a `check-uncommitted` leg, so
9/9 is unreachable while this task's own changes are still in the working tree — running it before
the commit yields `PASS:8 FAIL:1` on correct work. If the gate finds a real failure here, fix it
and amend; the commit is local and unpushed.

---

## Task 3: resolve the whole serial chain before running any of it

**Files:**
- Modify: `internal/core/run_env.go:251-273` (`executeGlobPattern`), `internal/core/run_env.go:279-331`
  (`executeRoots`)
- Test: `test/execution_properties_test.go`, inside `TestProperty_Execution`

**Interfaces:**
- Consumes: `RunOptions.ResolveOnly` from Task 2.
- Produces: `(*runExecutor).walkRoots(resolveOnly bool) error`;
  `(*runExecutor).executeGlobPattern(name string, opts RunOptions) error`.

**The trap in this task.** Walking the chain twice means every mode flag on `RunOptions` is now
seen twice. Two things break if the split is naive, and a Gate A reviewer produced both by
implementing an earlier version of this step:

- A caller who sets `RunOptions{ResolveOnly: true}` wants **one** non-executing pass. A split that
  unconditionally does resolve-then-execute silently promotes that caller's request into a real
  run, and Task 2's own test (`ResolveOnlyRunsNeitherTargetNorDeps`) fails with `targetRan == true`.
- `HelpOnly` survives unchanged into both passes, so everything gated on it — help printing and
  traversal — fires once per pass. Three pre-existing tests that have nothing to do with this issue
  (`TestProperty_Hierarchy/DefaultModeSoleGroupRootHelpOnRootNamePrintsOnce`,
  `DefaultModeSoleGroupRootHelpDescendsToNamedSubcommand`, `DefaultRootAdvertisesBothWorkingForms`)
  go red, counting doubled output.

Both are the same defect: the two-pass walk has no notion of "the caller already asked for a single
non-executing pass." Step 4 handles it with one up-front check.

- [ ] **Step 1: Write the failing tests**

Add these three subtests inside `TestProperty_Execution` in `test/execution_properties_test.go`:

```go
	// Issue #42: on a multi-target module `targ gen bogus` used to run gen and
	// only then reject bogus. The whole chain resolves before any of it runs.
	t.Run("UnresolvableChainTokenRunsNothing", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		genRan := false
		gen := targ.Targ(func() { genRan = true }).Name("gen")
		other := targ.Targ(func() {}).Name("other")

		result, err := targ.Execute([]string{"app", "gen", "bogus"}, gen, other)

		g.Expect(err).To(HaveOccurred())
		g.Expect(genRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("Unknown command: bogus"))
	})

	// Issue #42: the chain is walked twice, so anything gated on a mode flag
	// must not fire once per pass. A caller-supplied ResolveOnly means ONE
	// non-executing pass, not resolve-then-execute.
	t.Run("CallerSuppliedResolveOnlyStaysASinglePass", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ran := false
		target := targ.Targ(func() { ran = true }).Name("cmd")

		_, err := targ.ExecuteWithOptions(
			[]string{"app", "cmd"},
			targ.RunOptions{ResolveOnly: true, AllowDefault: true},
			target,
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ran).To(BeFalse())
	})

	// Issue #42: same hazard on the help path - HelpOnly survives into both
	// passes, so help would render twice.
	t.Run("HelpPrintsOnceWhenTheChainIsWalkedTwice", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := targ.Targ(func() {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "cmd", "--help"}, target)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(strings.Count(result.Output, "Usage:")).To(Equal(1))
	})
```

`strings` is already imported in this file.

Also add this subtest — it is a plain regression assertion, not a guard's teeth-check. See the note
under Step 5 for why the distinction matters here.

```go
	// Issue #42: the resolve pass parses the same args the execute pass will.
	// Pin that a variadic positional is still bound exactly once.
	t.Run("ChainPrePassBindsVariadicPositionalOnce", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var got []string

		collect := targ.Targ(func(a CollectArgs) { got = a.Files }).Name("collect")

		_, err := targ.Execute([]string{"app", "collect", "x", "y"}, collect)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal([]string{"x", "y"}))
	})
```

Add this type at package level in the same file, alphabetically among the other package-level
types (declaration ordering is enforced):

```go
// CollectArgs has a variadic positional, used to pin that the resolve pass and
// the execute pass together bind it exactly once.
type CollectArgs struct {
	Files []string `targ:"positional"`
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `targ test`

Expected, and check each individually rather than reading the summary count:

| subtest | expected at this step | why |
| --- | --- | --- |
| `UnresolvableChainTokenRunsNothing` | **FAIL**, `genRan` true | the chain still resolves lazily |
| `CallerSuppliedResolveOnlyStaysASinglePass` | PASS | single-pass walk today, so a caller's `ResolveOnly` is already honored |
| `HelpPrintsOnceWhenTheChainIsWalkedTwice` | PASS | single-pass walk today, so help renders once |
| `ChainPrePassBindsVariadicPositionalOnce` | PASS | nothing double-parses today |

Only the first is the red step. The other three are regression assertions that must stay green
through Steps 3-4 — they pin behavior this task is at risk of breaking, and they are the reason
Step 5 checks them explicitly rather than trusting a green summary line.

- [ ] **Step 3: Thread `opts` through `executeGlobPattern`**

In `internal/core/run_env.go`, change the signature and the two uses of `e.opts`:

```go
func (e *runExecutor) executeGlobPattern(name string, opts RunOptions) error {
	matches := e.findMatchingRootsGlob(name)
	if len(matches) == 0 {
		e.env.Printf("No targets match pattern: %s\n", name)

		return ExitError{Code: 1}
	}

	for _, matched := range matches {
		_, err := matched.executeWithParents(
			e.ctx,
			nil, // No args passed to glob-expanded targets
			nil,
			map[string]bool{},
			true,
			opts,
		)
		if err != nil {
			e.env.Printf("Error: %v\n", err)

			return ExitError{Code: 1}
		}
	}

	return nil
}
```

- [ ] **Step 4: Split `executeRoots` into a two-pass walk, with the single-pass guard**

Replace `executeRoots` with these two functions. Insert `walkRoots` alphabetically among the
`runExecutor` methods; Step 6 verifies the placement with `targ reorder-decls-check`.

The `HelpOnly || ResolveOnly` check is not an optimization. It is what keeps the three regression
assertions from Step 1 green — without it, a caller-supplied `ResolveOnly` gets promoted to a real
run and every `HelpOnly`-gated print fires twice.

```go
func (e *runExecutor) executeRoots() error {
	if e.opts.Overrides.Parallel {
		return e.runUnitsParallel()
	}

	// A caller that already asked for a single non-executing pass gets exactly
	// that. Promoting it to resolve-then-execute would run the target under a
	// caller-supplied ResolveOnly, and would fire every HelpOnly-gated print
	// once per pass.
	if e.opts.HelpOnly || e.opts.ResolveOnly {
		return e.walkRoots(e.opts.ResolveOnly)
	}

	// Resolve the whole chain before running any of it: a target must not fire
	// its deps or its command for an invocation that is going to be rejected.
	err := e.walkRoots(true)
	if err != nil {
		return err
	}

	return e.walkRoots(false)
}

// walkRoots runs the chain once. With resolveOnly set, every target parses its
// args and hands back its leftovers without executing, so an unresolvable
// token is reported before the first target runs.
func (e *runExecutor) walkRoots(resolveOnly bool) error {
	opts := e.opts
	opts.ResolveOnly = resolveOnly

	remaining := e.rest

	for len(remaining) > 0 {
		if remaining[0] == "^" {
			remaining = remaining[1:]

			continue
		}

		name := remaining[0]

		if isGlobPattern(name) {
			err := e.executeGlobPattern(name, opts)
			if err != nil {
				return err
			}

			remaining = remaining[1:]

			continue
		}

		matched := e.findMatchingRoot(name)
		if matched == nil {
			e.env.Printf("Unknown command: %s\n", name)
			printUsage(e.env.Stdout(), e.roots, e.opts)

			return ExitError{Code: 1}
		}

		next, err := matched.executeWithParents(
			e.ctx,
			remaining[1:],
			nil,
			map[string]bool{},
			!e.hasDefault,
			opts,
		)
		if err != nil {
			var re reportedError
			if !errors.As(err, &re) {
				e.env.Printf("Error: %v\n", err)
			}

			return ExitError{Code: 1}
		}

		remaining = next
	}

	return nil
}
```

- [ ] **Step 5: Run the tests — all four, checked individually**

Run: `targ test`

Expected: all four subtests from Step 1 PASS, and the whole suite green. Confirm these three by
name, because they are the ones this task can silently break and a summary line will not tell you:

```
targ test 2>&1 | grep -E "CallerSuppliedResolveOnlyStaysASinglePass|HelpPrintsOnceWhenTheChainIsWalkedTwice|ChainPrePassBindsVariadicPositionalOnce"
```

Also confirm the three pre-existing help tests named in "The trap in this task" are still green:

```
targ test 2>&1 | grep -E "DefaultModeSoleGroupRootHelpOnRootNamePrintsOnce|DefaultModeSoleGroupRootHelpDescendsToNamedSubcommand|DefaultRootAdvertisesBothWorkingForms"
```

**On `ChainPrePassBindsVariadicPositionalOnce`:** an earlier version of this plan called it a guard
test and predicted it would go red here, justifying a fresh-instance guard in
`executeFunctionWithParents`. That prediction was executed and proved false. `commandNode.Value`
(declared `command.go:112`) is never assigned anywhere in this tree, production or test, so
`nodeHasAddressableValue` is always false and `nodeInstance` always returns a fresh
`reflect.New(node.Type).Elem()`. There is no shared instance to double-bind. The guard was dropped
as unreachable code — it also tripped `cyclop` at complexity 11 against a max of 10. The test stays
as a cheap regression assertion; it is not evidence of a hazard, and it should not be used to argue
for one.

- [ ] **Step 6: Verify declaration ordering**

Run: `targ reorder-decls-check`

Expected: PASS. If it reports a move, run `targ reorder-decls` and include the result in this
task's commit.

- [ ] **Step 7: Commit**

```bash
git add internal/core/run_env.go test/execution_properties_test.go
git commit -F - <<'EOF'
feat(core): resolve the whole serial chain before running any of it

The dispatch loop resolved each leftover only after the previous command
had already run, so `targ gen bogus` ran gen and then reported bogus. The
loop now runs twice over the same chain: once in ResolveOnly mode, which
parses every target's args and collects leftovers without executing, and
then for real only if the entire chain resolved.

Walking twice means every mode flag is seen twice, so executeRoots checks
for a caller-supplied HelpOnly or ResolveOnly first and does a single pass
in that case. Without it a caller asking for one non-executing pass would
get a real run, and every HelpOnly-gated print would fire once per pass.
Three regression tests pin those two cases and the variadic binding.

executeGlobPattern takes opts so resolve mode reaches glob-expanded
targets.

Refs #42

AI-Used: [claude]
EOF
```

- [ ] **Step 8: Run the full gate, after the commit**

Run: `targ check-full`

Expected: PASS 9/9. The gate runs after the commit for the reason given in Task 1 Step 7 —
`check-uncommitted` cannot pass while this task's changes are uncommitted.

---

## Task 4: resolve parallel units before the fan-out starts

**Files:**
- Modify: `internal/core/run_env.go:549-593` (`resolveUnits`), `internal/core/run_env.go:726-731`
  (delete the post-hoc check)
- Test: `test/execution_properties_test.go`, inside `TestProperty_Execution`

**Interfaces:**
- Consumes: `RunOptions.ResolveOnly` from Task 2.

**Output text constraint — do not break these.** Two existing tests pin the **lowercase**
`unknown command: bogus`: `ParallelFlagBogusFailsOnGroupRoot`
(`test/execution_properties_test.go:671-681`) and `ParallelFlagBogusFailsOnPlainRoot` (`:683-696`).
The rejection this task adds must therefore print the `Error: unknown command: <arg>` form, not
the capital-`U` `Unknown command:` form the multi-root branch of `resolveUnits` uses.

- [ ] **Step 1: Write the failing test**

Add this subtest inside `TestProperty_Execution` in `test/execution_properties_test.go`:

**Pick the case carefully.** The obvious test — a sole shell target under `targ -p bogus` — does
NOT go red here, because Task 1 already made `executeShellCommand` reject the arg. It rejects
inside the unit's goroutine rather than before the fan-out, but with only one unit there is
nothing else to protect, so no assertion can see the difference. A Gate A reviewer confirmed that
version passes before any Task 4 code lands.

What this task actually changes is that rejection moves **ahead of** the fan-out, which is only
observable when a *sibling* unit would otherwise run. A sole group root supplies that: `sub`
resolves to a runnable default-mode unit, `bogus` does not, and today `sub` runs anyway.

Add this subtest inside `TestProperty_Execution` in `test/execution_properties_test.go`:

```go
	// Issue #42: resolveUnits deferred the default-mode verdict to a post-hoc
	// check inside the goroutine, so a sibling unit ran before the bad one was
	// rejected. One unresolvable arg must stop the whole fan-out.
	t.Run("ParallelUnresolvableArgRunsNoSiblingUnit", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		subRan := false
		sub := targ.Targ(func() { subRan = true }).Name("sub")
		grp := targ.Group("grp", sub)

		result, err := targ.Execute([]string{"app", "--parallel", "sub", "bogus"}, grp)

		g.Expect(err).To(HaveOccurred())
		g.Expect(subRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("unknown command: bogus"))
	})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `targ test`

Expected: FAIL with `subRan` true — `sub` ran even though the invocation was rejected. If it fails
on the output substring instead, note which, because that changes what Step 3 has to fix.

- [ ] **Step 3: Resolve default-mode units upfront**

In `internal/core/run_env.go`, replace the `if e.hasDefault { ... }` branch inside `resolveUnits`
(currently lines 577-586) with:

```go
		if e.hasDefault {
			unit := parallelUnit{
				node:     e.roots[0],
				name:     arg,
				args:     []string{arg},
				explicit: false,
			}

			// Resolve before the fan-out rather than after: a unit that
			// consumes none of its args names no real subcommand of the sole
			// root, and nothing should run for an invocation that will fail.
			resolveOpts := e.opts
			resolveOpts.ResolveOnly = true

			next, resolveErr := unit.node.executeWithParents(
				e.ctx, unit.args, nil, map[string]bool{}, unit.explicit, resolveOpts)
			if resolveErr != nil || len(next) == len(unit.args) {
				e.env.Printf("Error: %v\n", fmt.Errorf("%w: %s", errUnknownCommand, arg))

				return nil, ExitError{Code: 1}
			}

			units = append(units, unit)

			continue
		}
```

Note the print verb: `%v` on an error value built by `fmt.Errorf`. `%w` is an `Errorf` verb and
does not belong in a `Printf`. The wording it produces — `Error: unknown command: <arg>` — is
required by `ParallelFlagBogusFailsOnGroupRoot` and `ParallelFlagBogusFailsOnPlainRoot`, which
assert the lowercase form; see this task's "Output text constraint" above. Do not substitute the
capital-`U` `Unknown command:` message the multi-root branch of `resolveUnits` uses.

`fmt` stays imported in `run_env.go`: Step 4 deletes its use at `:730`, but this step adds a new
one.

- [ ] **Step 4: Delete the post-hoc check**

In `startUnits`, delete these lines (currently `run_env.go:726-731`), keeping the
`executeWithParents` call and the `resultCh` send:

```go
			// Only default-mode units carry args; one that consumed none of
			// them named no real subcommand of the sole root.
			if err == nil && len(unit.args) > 0 && len(next) == len(unit.args) {
				err = fmt.Errorf("%w: %s", errUnknownCommand, unit.args[0])
			}
```

`next` becomes unused at that call site; change the call to `_, err := unit.node.executeWithParents(...)`.

- [ ] **Step 5: Run the tests and verify they pass**

Run: `targ test`

Expected: PASS, including the two pre-existing tests named in the output-text constraint above.
If either of those two fails, the message shape is wrong — re-read the constraint; do not edit
those tests to match new output.

- [ ] **Step 6: Verify the post-hoc check is actually gone**

Do NOT grep for `len(next) == len(unit.args)` — Step 3's own new code contains that same
expression, so the grep returns a hit no matter what and can never establish the deletion. Match
the deleted comment instead, which is unique to the code being removed:

Run: `grep -n "named no real subcommand of the sole root" internal/core/run_env.go`

Expected: no output.

Then confirm the surviving occurrence is the new one, inside `resolveUnits` and not inside
`startUnits`:

Run: `grep -n "len(next) == len(unit.args)" internal/core/run_env.go`

Expected: exactly one hit, and its line number falls inside `resolveUnits`.

- [ ] **Step 7: Commit**

```bash
git add internal/core/run_env.go test/execution_properties_test.go
git commit -F - <<'EOF'
feat(core): resolve parallel units before the fan-out starts

resolveUnits pre-resolved every arg except in default mode, where an
unmatched arg became a unit with explicit false and the verdict was
deferred to a post-hoc check inside the unit's goroutine - which only ran
after the unit had. explicit false saved a function target there; it did
not save a shell target.

Default-mode units now resolve through ResolveOnly before any goroutine
starts, and the post-hoc check is deleted rather than kept alongside. The
rejection keeps the lowercase `unknown command: <arg>` wording that
ParallelFlagBogusFailsOnGroupRoot and ParallelFlagBogusFailsOnPlainRoot
pin.

The regression test uses a sole group root with a runnable sibling unit,
because a sole shell target cannot show the difference: Task 1 already
rejects that arg, just inside the goroutine instead of before the fan-out,
and with one unit there is nothing else to protect.

Refs #42

AI-Used: [claude]
EOF
```

- [ ] **Step 8: Run the full gate, after the commit**

Run: `targ check-full`

Expected: PASS 9/9. After the commit, for the reason given in Task 1 Step 7.

---

## Task 5: sync the living docs

The doc-surface disposition list below was produced by grepping the repo for the invariants this
change touches. Gate C reviews every file marked `update`.

**Files:**
- Modify: `docs/specs/architecture.md:21`, `docs/specs/requirements.md:21`, `docs/specs/tests.md`
  (T-3, starts line 32), `README.md:334-338`

| file | lines | disposition | reason |
| --- | --- | --- | --- |
| `docs/specs/architecture.md` | 21 (ARCH-3) | **update** | Owns both the `explicit=!hasDefault` sentence and the `resolveUnits` sentence; both change |
| `docs/specs/requirements.md` | 21 (REQ-3) | **update** | Owns target execution and default-target dispatch; the new "a rejected invocation runs nothing" invariant belongs here |
| `docs/specs/tests.md` | 32 (T-3) | **update** | T-3 is Execution behavior; the four new tests list there |
| `docs/specs/implementation.md` | 36-42 (IMPL-5) | keep | Files list already covers `command.go`, `run_env.go`, `parse.go`; Key functions names only exported entry points and no new exported function is added |
| `docs/specs/use-cases.md` | UC-1, UC-5 | keep | UC level states user goals; rejecting a bad invocation is not a new goal |
| `README.md` | 334-338 (Default Target) | **update** | User-visible guarantee change; the Default Target section states the sole-target dispatch contract |
| `docs/archive/**` | many | N/A | Frozen — verified independently: the only commit touching it since 2026-03-01 is the archiving commit `ac98256` (2026-03-08) |
| `docs/plans/2026-*` | many | N/A | Historical cycle records, not living docs |
| `specs/001-parallel-output/contracts/api.md` | 115-120 | N/A | Backward-compatibility list covers output prefixing, not argument resolution |
| `projects/portable-targets/**` | — | N/A | A different project's docs |
| `CLAUDE.md` | — | N/A | Development process rules; no dispatch semantics |

- [ ] **Step 1: Update ARCH-3**

In `docs/specs/architecture.md:21`, replace the sentence

> `explicit=!hasDefault` distinguishes whether an unmatched trailing arg errors immediately
> (default mode) or is handed back for root-chaining (multi-root mode).

with

> `explicit=!hasDefault` distinguishes whether an unmatched trailing arg errors immediately
> (default mode) or is handed back for root-chaining (multi-root mode); shell-command targets
> honor it on the same terms as function targets. Dispatch resolves before it executes:
> `executeRoots` walks the whole chain under `RunOptions.ResolveOnly` — a non-printing sibling of
> `HelpOnly` that parses each target's args and returns its leftovers without running it — and
> executes only if every token resolved.

and, in the same paragraph, replace

> Parallel mode (`-p`) resolves each arg to a `parallelUnit`: glob first, then an explicit root
> match, then (default mode only) the sole root's subcommand.

with

> Parallel mode (`-p`) resolves each arg to a `parallelUnit` before any goroutine starts: glob
> first, then an explicit root match, then (default mode only) the sole root's subcommand,
> resolved through `ResolveOnly` rather than deferred to a post-hoc check.

- [ ] **Step 2: Update REQ-3**

In `docs/specs/requirements.md:21`, append to the REQ-3 paragraph:

> An invocation that targ will reject runs nothing: every token in a command chain, and every
> `-p` unit, is resolved to a target before the first dependency or command executes. This holds
> for shell-command targets and function targets alike.

- [ ] **Step 3: Update T-3**

In `docs/specs/tests.md`, in the T-3 property list (section starts line 32), add:

```markdown
- Property: an unresolvable token runs nothing — not the target, not its deps — in serial chains, under `-p`, and for shell-command targets
- Property: the chain resolve pass does not double-bind a variadic positional
```

and add these test names to T-3's **Tests:** line:
`TestProperty_Execution/ResolveOnlyRunsNeitherTargetNorDeps`,
`TestProperty_Execution/UnresolvableChainTokenRunsNothing`,
`TestProperty_Execution/ChainPrePassBindsVariadicPositionalOnce`,
`TestProperty_Execution/CallerSuppliedResolveOnlyStaysASinglePass`,
`TestProperty_Execution/HelpPrintsOnceWhenTheChainIsWalkedTwice`,
`TestProperty_Execution/ParallelUnresolvableArgRunsNoSiblingUnit`,
`TestProperty_ShellCommandDeps/UnknownBareArgRunsNeitherDepNorShellCommand`.

- [ ] **Step 4: Update the README Default Target section**

In `README.md`, after the existing Default Target paragraph (line 336), add:

```markdown
An invocation targ rejects runs nothing. `targ bogus` reports the unknown command without running
the default target or its dependencies, and the same holds for a shell-command target, for a
chain like `targ build bogus`, and under `-p`.
```

- [ ] **Step 5: Verify no other doc claims the old behavior**

Run:

```bash
grep -rn "runs the command and then\|then errors\|after the side effects" README.md docs/specs/
```

Expected: no output. If a hit appears, that file was missed by the disposition table and needs a
disposition of its own.

- [ ] **Step 6: Commit**

```bash
git add docs/specs/architecture.md docs/specs/requirements.md docs/specs/tests.md README.md
git commit -F - <<'EOF'
docs: dispatch resolves before it executes

ARCH-3 described the explicit=!hasDefault switch and the parallel unit
resolution as they were before #42, including a deferred verdict that no
longer exists. REQ-3 gained the user-visible invariant the change
establishes: an invocation targ will reject runs nothing.

T-3 lists the new tests. The README states the guarantee where the
default-target contract already lives.

Refs #42

AI-Used: [claude]
EOF
```

- [ ] **Step 7: Run the full gate, after the commit**

Run: `targ check-full`

Expected: PASS 9/9, tree clean. After the commit, for the reason given in Task 1 Step 7.

---

## Acceptance

- [ ] `targ bogus` on a sole shell target: exit 1, `unknown command: bogus`, dep did not run,
      command did not run
- [ ] `targ gen bogus` on a sole shell target: same
- [ ] `targ gen bogus` on a two-target module: exit 1, `Unknown command: bogus`, `gen` did not run
- [ ] `targ gen other` on a two-target module: exit 0, both ran — chaining unchanged
- [ ] `targ -p sub bogus` on a sole group root: exit 1, `sub` did not run
- [ ] A caller-supplied `RunOptions{ResolveOnly: true}` stays one non-executing pass
- [ ] `targ cmd --help` renders its usage block exactly once, not once per chain pass
- [ ] `ParallelFlagBogusFailsOnGroupRoot`, `ParallelFlagBogusFailsOnPlainRoot`,
      `ParallelFlagSurfacesTargetsOwnError`, `MultipleTargetsRunSequentially`,
      `DefaultModeSoleGroupRootHelpOnRootNamePrintsOnce`,
      `DefaultModeSoleGroupRootHelpDescendsToNamedSubcommand`, and
      `DefaultRootAdvertisesBothWorkingForms` stay green, untouched
- [ ] A variadic positional receives its args once
- [ ] `grep -n "named no real subcommand of the sole root" internal/core/run_env.go` returns
      nothing, and `len(next) == len(unit.args)` survives only inside `resolveUnits`
- [ ] No fresh-instance guard was added to `executeFunctionWithParents` — the hazard it was
      written for does not exist, because `commandNode.Value` is never assigned
- [ ] `targ check-full` PASS 9/9, tree clean, run after each task's commit
- [ ] Spec sync landed in ARCH-3, REQ-3, T-3, and the README

## Out of scope

- The multi-target `targ gen bogus` case is fixed here, but note it was never a shell-specific
  defect: a function target behaved identically. That is recorded in the design doc.
- `specs/001-parallel-output/contracts/api.md:119` was flagged for Joe's yes/no during issue #40
  and appears still unresolved. Not this cycle's to decide, and not to be filed unilaterally.
