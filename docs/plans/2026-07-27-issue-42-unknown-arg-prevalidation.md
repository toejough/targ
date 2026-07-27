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
| `internal/core/command.go` | Per-target-kind execution and its early returns | Honor `ResolveOnly` at three existing seams; honor `explicit` in the shell path; fresh instance in resolve mode |
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

Then, immediately after the existing unknown-flag check (currently lines 1046-1050) and **before**
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

- [ ] **Step 6: Run the full gate**

Run: `targ check-full`

Expected: PASS 9/9. `unparam` may now observe that `explicit` has a single caller passing a
non-constant value — it does, from `command.go:168`, so no finding is expected. If lint reports a
phantom failure citing a path that does not exist, run `golangci-lint cache clean` and re-run.

- [ ] **Step 7: Commit**

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

- [ ] **Step 4: Honor it at the three existing seams**

All three are existing `if opts.HelpOnly` early returns. Change each to also fire on
`ResolveOnly`. Do **not** touch the help printing at `command.go:157-161` — that stays
`HelpOnly`-only, and it is the entire difference between the two modes.

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

In `executeShellCommand` (currently `command.go:1037-1039`):

```go
	if opts.HelpOnly || opts.ResolveOnly {
		return parsed.remaining, nil
	}
```

- [ ] **Step 5: Run the test and verify it passes**

Run: `targ test`

Expected: PASS.

- [ ] **Step 6: Verify declaration ordering**

Run: `targ reorder-decls-check`

Expected: PASS. If it reports a move, run `targ reorder-decls` and include the result in this
task's commit rather than leaving it for a later task to trip over.

- [ ] **Step 7: Run the full gate**

Run: `targ check-full`

Expected: PASS 9/9.

- [ ] **Step 8: Commit**

```bash
git add internal/core/types.go internal/core/command.go test/execution_properties_test.go
git commit -F - <<'EOF'
feat(core): add ResolveOnly, a non-printing sibling of HelpOnly

HelpOnly already parses a target's args and hands back its leftovers
without executing, at every target kind - function, shell, deps-only, and
group. The only thing keeping it from being a general resolve pass is that
it also prints help.

ResolveOnly reuses the same three early-return seams and skips the
printing, giving one primitive: given a node and args, what is left over,
without running anything. The chain pre-pass and the parallel unit
resolution both build on it.

Refs #42

AI-Used: [claude]
EOF
```

---

## Task 3: resolve the whole serial chain before running any of it

**Files:**
- Modify: `internal/core/run_env.go:251-273` (`executeGlobPattern`), `internal/core/run_env.go:279-331`
  (`executeRoots`), `internal/core/command.go` `executeFunctionWithParents` (fresh instance)
- Test: `test/execution_properties_test.go`, inside `TestProperty_Execution`

**Interfaces:**
- Consumes: `RunOptions.ResolveOnly` from Task 2.
- Produces: `(*runExecutor).walkRoots(resolveOnly bool) error`;
  `(*runExecutor).executeGlobPattern(name string, opts RunOptions) error`.

**Why the fresh instance is in this task:** `nodeInstance` (`command.go:1408-1418`) returns the
**shared** `node.Value` when the node has an addressable value, and slice binding appends
(`parse.go:221`, `parse.go:872`). Nothing double-parses today. This task introduces the first
double-parse, so without the guard a variadic positional or repeated flag would silently double.

- [ ] **Step 1: Write the two failing tests**

Add both subtests inside `TestProperty_Execution` in `test/execution_properties_test.go`:

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

	// Issue #42: the resolve pass parses the same args the execute pass will,
	// and slice binding appends (parse.go:221, :872) into a node instance that
	// nodeInstance shares when it is addressable. Resolve must use a throwaway.
	t.Run("ChainPrePassDoesNotDoubleBindVariadicPositional", func(t *testing.T) {
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
// CollectArgs has a variadic positional, used to prove the resolve pass does
// not leave bound values behind for the execute pass to append to.
type CollectArgs struct {
	Files []string `targ:"positional"`
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `targ test`

Expected: `UnresolvableChainTokenRunsNothing` FAILS with `genRan` true.
`ChainPrePassDoesNotDoubleBindVariadicPositional` PASSES at this point — nothing double-parses
yet. That is expected and correct: it is a guard test that must go red in Step 4 and green again
in Step 5. Record its Step-2 result so the Step-4 transition is verifiable.

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

- [ ] **Step 4: Split `executeRoots` into a two-pass walk**

Replace `executeRoots` with these two functions. Insert `walkRoots` alphabetically among the
`runExecutor` methods — `targ reorder-decls` will place it if you get it wrong, but check.

```go
func (e *runExecutor) executeRoots() error {
	if e.opts.Overrides.Parallel {
		return e.runUnitsParallel()
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

- [ ] **Step 5: Run the tests — confirm the guard test now goes RED**

Run: `targ test`

Expected: `UnresolvableChainTokenRunsNothing` now PASSES.
`ChainPrePassDoesNotDoubleBindVariadicPositional` now **FAILS**, with `got` equal to
`["x", "y", "x", "y"]`.

That transition is the point of the guard test: it proves the double-bind hazard is real in this
tree rather than theoretical. If it does **not** fail here, stop and find out why before adding
the guard — a guard whose hazard cannot be demonstrated is untested no matter what the suite says.

- [ ] **Step 6: Add the fresh-instance guard**

In `internal/core/command.go`, in `executeFunctionWithParents`, replace the opening
`inst := nodeInstance(node)` with:

```go
	// Create instance for current node if it has a Type (struct arg). In
	// resolve mode the parse result is discarded, so bind into a throwaway:
	// nodeInstance shares node.Value when it is addressable, and slice binding
	// appends (parse.go:221, :872), so reusing it would double a variadic
	// positional across the resolve and execute passes.
	inst := nodeInstance(node)
	if opts.ResolveOnly && node != nil && node.Type != nil {
		inst = reflect.New(node.Type).Elem()
	}
```

`reflect` is already imported in this file.

- [ ] **Step 7: Run the tests and verify both pass**

Run: `targ test`

Expected: PASS — both new subtests green, whole suite green.

- [ ] **Step 8: Verify ordering and run the full gate**

Run: `targ reorder-decls-check` then `targ check-full`

Expected: both PASS; `check-full` 9/9. Fix any ordering move here rather than leaving it for a
later task.

- [ ] **Step 9: Commit**

```bash
git add internal/core/run_env.go internal/core/command.go test/execution_properties_test.go
git commit -F - <<'EOF'
feat(core): resolve the whole serial chain before running any of it

The dispatch loop resolved each leftover only after the previous command
had already run, so `targ gen bogus` ran gen and then reported bogus. The
loop now runs twice over the same chain: once in ResolveOnly mode, which
parses every target's args and collects leftovers without executing, and
then for real only if the entire chain resolved.

executeGlobPattern takes opts so resolve mode reaches glob-expanded
targets. executeFunctionWithParents binds into a throwaway instance in
resolve mode: nodeInstance shares node.Value when it is addressable and
slice binding appends, so the two passes would otherwise double a variadic
positional. A regression test pins that, and was confirmed to fail against
the two-pass walk before the guard was added.

Refs #42

AI-Used: [claude]
EOF
```

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

```go
	// Issue #42: -p on a sole shell target had the same defect as the serial
	// path, via a third route - resolveUnits deferred the verdict to a
	// post-hoc check that only fired after the unit had already run.
	t.Run("ParallelUnknownBareArgRunsNoShellCommand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		shellRan := false

		mockRunner := func(_ context.Context, _ string) error {
			shellRan = true

			return nil
		}

		main := targ.Targ("echo hello").Name("main")

		result, err := targ.ExecuteWithOptions(
			[]string{"app", "--parallel", "bogus"},
			targ.RunOptions{ShellRunner: mockRunner, AllowDefault: true},
			main,
		)

		g.Expect(err).To(HaveOccurred())
		g.Expect(shellRan).To(BeFalse())
		g.Expect(result.Output).To(ContainSubstring("unknown command: bogus"))
	})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `targ test`

Expected: FAIL with `shellRan` true.

Note: Task 1 made `executeShellCommand` error on this input, but in the parallel path that error
is raised **inside the unit's goroutine**, after the fan-out has begun. This test fails on
`shellRan` only if the shell command still runs; if Task 1 already made `shellRan` false, the
remaining value of this task is moving the rejection ahead of the fan-out. Record which assertion
actually failed — it determines whether Step 3 is a behavior fix or a structural one.

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
does not belong in a `Printf`. The wording it produces — `Error: unknown command: <arg>` — is what
the two pinned tests require.

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

Run: `grep -n "len(next) == len(unit.args)" internal/core/run_env.go`

Expected: no output.

- [ ] **Step 7: Run the full gate**

Run: `targ check-full`

Expected: PASS 9/9.

- [ ] **Step 8: Commit**

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

Refs #42

AI-Used: [claude]
EOF
```

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
`TestProperty_Execution/ChainPrePassDoesNotDoubleBindVariadicPositional`,
`TestProperty_Execution/ParallelUnknownBareArgRunsNoShellCommand`,
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

- [ ] **Step 6: Run the full gate**

Run: `targ check-full`

Expected: PASS 9/9, tree clean.

- [ ] **Step 7: Commit**

```bash
git add docs/specs/architecture.md docs/specs/requirements.md docs/specs/tests.md README.md
git commit -F - <<'EOF'
docs: dispatch resolves before it executes

ARCH-3 described the explicit=!hasDefault switch and the parallel unit
resolution as they were before #42, including a deferred verdict that no
longer exists. REQ-3 gained the user-visible invariant the change
establishes: an invocation targ will reject runs nothing.

T-3 lists the five new tests. The README states the guarantee where the
default-target contract already lives.

Refs #42

AI-Used: [claude]
EOF
```

---

## Acceptance

- [ ] `targ bogus` on a sole shell target: exit 1, `unknown command: bogus`, dep did not run,
      command did not run
- [ ] `targ gen bogus` on a sole shell target: same
- [ ] `targ gen bogus` on a two-target module: exit 1, `Unknown command: bogus`, `gen` did not run
- [ ] `targ gen other` on a two-target module: exit 0, both ran — chaining unchanged
- [ ] `targ -p bogus` on a sole shell target: exit 1, command did not run
- [ ] `ParallelFlagBogusFailsOnGroupRoot`, `ParallelFlagBogusFailsOnPlainRoot`,
      `ParallelFlagSurfacesTargetsOwnError`, and `MultipleTargetsRunSequentially` stay green,
      untouched
- [ ] A variadic positional receives its args once, not doubled
- [ ] `grep -n "len(next) == len(unit.args)" internal/core/run_env.go` returns nothing
- [ ] `targ check-full` PASS 9/9, tree clean
- [ ] Spec sync landed in ARCH-3, REQ-3, T-3, and the README

## Out of scope

- The multi-target `targ gen bogus` case is fixed here, but note it was never a shell-specific
  defect: a function target behaved identically. That is recorded in the design doc.
- `specs/001-parallel-output/contracts/api.md:119` was flagged for Joe's yes/no during issue #40
  and appears still unresolved. Not this cycle's to decide, and not to be filed unilaterally.
